package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"demo-agent/catalog"
	"gopkg.in/yaml.v3"
)

type Client struct {
	baseURL      *url.URL
	demoAgentURL *url.URL
	httpClient   *http.Client
}

func New(agentStoreBaseURL, demoAgentBaseURL string) (*Client, error) {
	agentStoreURL, err := parseBaseURL(agentStoreBaseURL)
	if err != nil {
		return nil, fmt.Errorf("agent-store base URL: %w", err)
	}
	demoURL, err := parseBaseURL(demoAgentBaseURL)
	if err != nil {
		return nil, fmt.Errorf("demo-agent base URL: %w", err)
	}
	return &Client{baseURL: agentStoreURL, demoAgentURL: demoURL, httpClient: http.DefaultClient}, nil
}

func (client *Client) Bootstrap(ctx context.Context, source catalog.Catalog) error {
	contracts, err := client.functionContracts(ctx)
	if err != nil {
		return err
	}
	for _, contract := range source.FunctionContracts {
		if existing, found := contracts[functionContractKeyForCatalog(contract)]; found {
			if !sameContract(existing, contract) {
				return fmt.Errorf("catalog drift: function contract %q differs from active data", contract.Code)
			}
			continue
		}
		if err := client.createContract(ctx, contract); err != nil {
			return err
		}
	}
	for _, agent := range source.Agents {
		manifest, err := renderManifest(source, agent, client.demoAgentURL)
		if err != nil {
			return err
		}
		desired, err := client.validateManifest(ctx, manifest)
		if err != nil {
			return fmt.Errorf("validate manifest %q: %w", agent.Code, err)
		}
		existing, found, err := client.agent(ctx, agent.Code)
		if err != nil {
			return err
		}
		if !found {
			versionID, err := client.importManifest(ctx, manifest)
			if err != nil {
				return fmt.Errorf("import manifest %q: %w", agent.Code, err)
			}
			if err := client.publish(ctx, versionID); err != nil {
				return fmt.Errorf("publish agent %q: %w", agent.Code, err)
			}
			continue
		}
		activeID := activeVersionID(existing)
		if activeID == "" {
			return fmt.Errorf("catalog drift: agent %q exists without an active version", agent.Code)
		}
		existingManifest, err := client.exportManifest(ctx, activeID)
		if err != nil {
			return fmt.Errorf("read active manifest %q: %w", agent.Code, err)
		}
		if existingManifest.SHA256 != desired.SHA256 {
			return fmt.Errorf("catalog drift: active agent %q manifest differs", agent.Code)
		}
	}
	return nil
}

type functionContractResponse struct {
	Code            string         `json:"code"`
	ContractVersion string         `json:"contractVersion"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	ResponseFormat  string         `json:"responseFormat"`
	InputSchema     map[string]any `json:"inputSchema"`
	OutputSchema    map[string]any `json:"outputSchema"`
}

type functionContractKey struct {
	code            string
	contractVersion string
}

func functionContractKeyForCatalog(contract catalog.FunctionContract) functionContractKey {
	return functionContractKey{code: contract.Code, contractVersion: contract.ContractVersion}
}

func functionContractKeyForResponse(contract functionContractResponse) functionContractKey {
	return functionContractKey{code: contract.Code, contractVersion: contract.ContractVersion}
}

func (client *Client) functionContracts(ctx context.Context) (map[functionContractKey]functionContractResponse, error) {
	var response []functionContractResponse
	if err := client.request(ctx, http.MethodGet, "/api/function-contracts", nil, &response); err != nil {
		return nil, fmt.Errorf("list function contracts: %w", err)
	}
	result := make(map[functionContractKey]functionContractResponse, len(response))
	for _, contract := range response {
		result[functionContractKeyForResponse(contract)] = contract
	}
	return result, nil
}

func (client *Client) createContract(ctx context.Context, contract catalog.FunctionContract) error {
	return client.request(ctx, http.MethodPost, "/api/function-contracts", functionContractResponse{Code: contract.Code, ContractVersion: contract.ContractVersion, Name: contract.Name, Description: contract.Description, ResponseFormat: contract.ResponseFormat, InputSchema: contract.InputSchema, OutputSchema: contract.OutputSchema}, nil)
}

type agentResponse struct {
	Versions []agentVersionResponse `json:"versions"`
}
type agentVersionResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
type manifestResponse struct {
	SHA256 string `json:"sha256"`
}

func (client *Client) agent(ctx context.Context, code string) (agentResponse, bool, error) {
	var response agentResponse
	err := client.request(ctx, http.MethodGet, "/api/agents/"+url.PathEscape(code), nil, &response)
	if httpError, ok := err.(httpError); ok && httpError.StatusCode == http.StatusNotFound {
		return agentResponse{}, false, nil
	}
	return response, err == nil, err
}

func (client *Client) validateManifest(ctx context.Context, content string) (manifestResponse, error) {
	var response manifestResponse
	err := client.request(ctx, http.MethodPost, "/api/agent-manifests/validate", map[string]string{"content": content}, &response)
	return response, err
}

func (client *Client) importManifest(ctx context.Context, content string) (string, error) {
	var response struct {
		VersionID string `json:"versionId"`
	}
	err := client.request(ctx, http.MethodPost, "/api/agent-manifests", map[string]string{"content": content}, &response)
	return response.VersionID, err
}

func (client *Client) publish(ctx context.Context, versionID string) error {
	return client.request(ctx, http.MethodPost, "/api/agent-versions/"+url.PathEscape(versionID)+"/publish", nil, nil)
}
func (client *Client) exportManifest(ctx context.Context, versionID string) (manifestResponse, error) {
	var response manifestResponse
	err := client.request(ctx, http.MethodGet, "/api/agent-versions/"+url.PathEscape(versionID)+"/manifest", nil, &response)
	return response, err
}

func (client *Client) request(ctx context.Context, method, path string, payload any, result any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL.String()+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return httpError{response.StatusCode, string(content)}
	}
	if result == nil {
		return nil
	}
	var envelope struct {
		IsSuccess bool            `json:"isSuccess"`
		Result    json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(content, &envelope); err != nil {
		return err
	}
	if !envelope.IsSuccess || len(envelope.Result) == 0 {
		return fmt.Errorf("agent-store returned unsuccessful response")
	}
	return json.Unmarshal(envelope.Result, result)
}

type httpError struct {
	StatusCode int
	body       string
}

func (err httpError) Error() string {
	return fmt.Sprintf("agent-store returned %d: %s", err.StatusCode, err.body)
}

func sameContract(existing functionContractResponse, expected catalog.FunctionContract) bool {
	return existing.ContractVersion == expected.ContractVersion && existing.Name == expected.Name && existing.Description == expected.Description && existing.ResponseFormat == expected.ResponseFormat && equalJSON(existing.InputSchema, expected.InputSchema) && equalJSON(existing.OutputSchema, expected.OutputSchema)
}
func equalJSON(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}
func activeVersionID(agent agentResponse) string {
	for _, version := range agent.Versions {
		if version.Status == "ACTIVE" {
			return version.ID
		}
	}
	return ""
}
func parseBaseURL(rawURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(strings.TrimRight(rawURL, "/"))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("must be an HTTP(S) origin")
	}
	return parsed, nil
}

type manifest struct {
	APIVersion   string               `yaml:"apiVersion"`
	Agent        manifestAgent        `yaml:"agent"`
	Dependencies []manifestDependency `yaml:"dependencies,omitempty"`
}
type manifestAgent struct {
	DeveloperID string           `yaml:"developerId"`
	Code        string           `yaml:"code"`
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Version     string           `yaml:"version"`
	UsageType   string           `yaml:"usageType"`
	Function    manifestFunction `yaml:"function"`
	Endpoint    string           `yaml:"endpoint"`
	Payment     manifestPayment  `yaml:"payment"`
}
type manifestFunction struct {
	Code    string `yaml:"code"`
	Version string `yaml:"version"`
}
type manifestPayment struct {
	PriceAtomic string `yaml:"priceAtomic"`
	Network     string `yaml:"network"`
	Asset       string `yaml:"asset"`
	PayTo       string `yaml:"payTo"`
}
type manifestDependency struct {
	Function    manifestFunction    `yaml:"function"`
	Providers   manifestProviders   `yaml:"providers"`
	Constraints manifestConstraints `yaml:"constraints"`
	Resolution  manifestResolution  `yaml:"resolution"`
}
type manifestProviders struct {
	Scope string `yaml:"scope"`
}
type manifestConstraints struct {
	VersionConstraint string `yaml:"versionConstraint"`
	Required          bool   `yaml:"required"`
	MaxPriceAtomic    string `yaml:"maxPriceAtomic"`
	MaxCalls          int    `yaml:"maxCalls"`
}
type manifestResolution struct {
	Strategy string `yaml:"strategy"`
}

func renderManifest(source catalog.Catalog, agent catalog.Definition, demoAgentURL *url.URL) (string, error) {
	value := manifest{APIVersion: catalog.APIVersion, Agent: manifestAgent{DeveloperID: source.DeveloperID, Code: agent.Code, Name: agent.Name, Description: agent.Description, Version: source.AgentVersion, UsageType: agent.UsageType, Function: manifestFunction{Code: agent.FunctionCode, Version: source.ContractVersion}, Endpoint: demoAgentURL.String() + "/agents/" + agent.Code + "/invoke", Payment: manifestPayment{PriceAtomic: agent.PriceAtomic, Network: source.Network, Asset: source.Asset, PayTo: agent.PayTo}}}
	for _, dependency := range agent.Dependencies {
		value.Dependencies = append(value.Dependencies, manifestDependency{Function: manifestFunction{Code: dependency.FunctionCode, Version: source.ContractVersion}, Providers: manifestProviders{Scope: dependency.ProviderScope}, Constraints: manifestConstraints{VersionConstraint: dependency.VersionConstraint, Required: dependency.Required, MaxPriceAtomic: dependency.MaxPriceAtomic, MaxCalls: dependency.MaxCalls}, Resolution: manifestResolution{Strategy: dependency.Strategy}})
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
