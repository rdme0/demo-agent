package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion     = "agentstore/v1"
	Network        = "eip155:84532"
	Asset          = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
	maxSchemaBytes = 64 * 1024
	maxSchemaDepth = 32
)

//go:embed agents.yaml
var embeddedAgents []byte

type Dependency struct {
	FunctionCode      string `yaml:"functionCode"`
	ProviderScope     string `yaml:"providerScope"`
	VersionConstraint string `yaml:"versionConstraint"`
	Required          bool   `yaml:"required"`
	MaxPriceAtomic    string `yaml:"maxPriceAtomic"`
	MaxCalls          int    `yaml:"maxCalls"`
	Strategy          string `yaml:"strategy"`
}

type Runtime struct {
	Role              string `yaml:"role"`
	Prompt            string `yaml:"prompt"`
	Fixture           any    `yaml:"fixture"`
	RequiresWebSearch bool   `yaml:"requiresWebSearch"`
	MaxOutputTokens   int64  `yaml:"maxOutputTokens"`
}

type FunctionContract struct {
	Code            string         `yaml:"code"`
	Name            string         `yaml:"name"`
	Description     string         `yaml:"description"`
	ResponseFormat  string         `yaml:"responseFormat"`
	InputSchema     map[string]any `yaml:"inputSchema"`
	OutputSchema    map[string]any `yaml:"outputSchema"`
	ContractVersion string         `yaml:"-"`
}

type Definition struct {
	Code                string         `yaml:"code"`
	Name                string         `yaml:"name"`
	Description         string         `yaml:"description"`
	DeveloperID         string         `yaml:"-"`
	DeveloperName       string         `yaml:"-"`
	FunctionCode        string         `yaml:"functionCode"`
	FunctionName        string         `yaml:"-"`
	FunctionDescription string         `yaml:"-"`
	ResponseFormat      string         `yaml:"-"`
	UsageType           string         `yaml:"usageType"`
	PriceAtomic         string         `yaml:"priceAtomic"`
	PayTo               string         `yaml:"payTo"`
	Prompt              string         `yaml:"-"`
	Fixture             any            `yaml:"-"`
	InputSchema         map[string]any `yaml:"-"`
	OutputSchema        map[string]any `yaml:"-"`
	RequiresWebSearch   bool           `yaml:"-"`
	AggregateMarkdown   bool           `yaml:"-"`
	MinimumSources      int            `yaml:"-"`
	MaxOutputTokens     int64          `yaml:"-"`
	Dependencies        []Dependency   `yaml:"dependencies"`
	Runtime             Runtime        `yaml:"runtime"`
}

type document struct {
	APIVersion        string             `yaml:"apiVersion"`
	ContractVersion   string             `yaml:"contractVersion"`
	AgentVersion      string             `yaml:"agentVersion"`
	Network           string             `yaml:"network"`
	Asset             string             `yaml:"asset"`
	Developer         developer          `yaml:"developer"`
	FunctionContracts []FunctionContract `yaml:"functionContracts"`
	Agents            []Definition       `yaml:"agents"`
}

type developer struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

type Catalog struct {
	ContractVersion   string
	AgentVersion      string
	Network           string
	Asset             string
	DeveloperID       string
	DeveloperName     string
	FunctionContracts []FunctionContract
	Agents            []Definition
}

func LoadEmbedded() (Catalog, error) {
	return Parse(embeddedAgents)
}

func Definitions() []Definition {
	catalog, err := LoadEmbedded()
	if err != nil {
		panic(fmt.Sprintf("invalid embedded catalog: %v", err))
	}
	return catalog.Agents
}

func Parse(content []byte) (Catalog, error) {
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	var parsed document
	if err := decoder.Decode(&parsed); err != nil {
		return Catalog{}, fmt.Errorf("decode catalog YAML: %w", err)
	}
	if parsed.APIVersion != APIVersion || parsed.ContractVersion == "" || parsed.AgentVersion == "" {
		return Catalog{}, fmt.Errorf("catalog apiVersion, contractVersion, and agentVersion are required")
	}
	if parsed.Network != Network || !strings.EqualFold(parsed.Asset, Asset) || parsed.Developer.ID == "" || parsed.Developer.Name == "" {
		return Catalog{}, fmt.Errorf("catalog network, asset, and developer are invalid")
	}
	contracts := make(map[string]FunctionContract, len(parsed.FunctionContracts))
	for index := range parsed.FunctionContracts {
		contract := &parsed.FunctionContracts[index]
		if contract.Code == "" || contract.Name == "" || contract.Description == "" || contract.ResponseFormat == "" {
			return Catalog{}, fmt.Errorf("functionContracts[%d] is incomplete", index)
		}
		if err := validateFunctionContract(*contract); err != nil {
			return Catalog{}, fmt.Errorf("functionContracts[%d] is invalid: %w", index, err)
		}
		if _, exists := contracts[contract.Code]; exists {
			return Catalog{}, fmt.Errorf("duplicate function contract %q", contract.Code)
		}
		contract.ContractVersion = parsed.ContractVersion
		contracts[contract.Code] = *contract
	}
	if len(contracts) == 0 {
		return Catalog{}, fmt.Errorf("catalog requires function contracts")
	}
	codes := make(map[string]struct{}, len(parsed.Agents))
	for index := range parsed.Agents {
		agent := &parsed.Agents[index]
		contract, exists := contracts[agent.FunctionCode]
		if agent.Code == "" || agent.Name == "" || agent.Description == "" || !exists || agent.UsageType == "" || !positiveAtomic(agent.PriceAtomic) || !evmAddress(agent.PayTo) {
			return Catalog{}, fmt.Errorf("agents[%d] is incomplete or invalid", index)
		}
		if _, exists := codes[agent.Code]; exists {
			return Catalog{}, fmt.Errorf("duplicate agent code %q", agent.Code)
		}
		codes[agent.Code] = struct{}{}
		if agent.Runtime.Role != "root" && agent.Runtime.Role != "specialist" {
			return Catalog{}, fmt.Errorf("agent %q runtime role must be root or specialist", agent.Code)
		}
		if agent.Runtime.Prompt == "" || agent.Runtime.Fixture == nil {
			return Catalog{}, fmt.Errorf("agent %q runtime is incomplete", agent.Code)
		}
		if agent.Runtime.Role == "root" && len(agent.Dependencies) == 0 {
			return Catalog{}, fmt.Errorf("root agent %q requires dependencies", agent.Code)
		}
		if agent.Runtime.Role == "specialist" && len(agent.Dependencies) != 0 {
			return Catalog{}, fmt.Errorf("specialist agent %q cannot declare dependencies", agent.Code)
		}
		for _, dependency := range agent.Dependencies {
			if _, exists := contracts[dependency.FunctionCode]; !exists || dependency.ProviderScope != "marketplace" || dependency.VersionConstraint == "" || !dependency.Required || !positiveAtomic(dependency.MaxPriceAtomic) || dependency.MaxCalls < 1 || dependency.MaxCalls > 5 || !isProviderStrategy(dependency.Strategy) {
				return Catalog{}, fmt.Errorf("agent %q has invalid dependency", agent.Code)
			}
		}
		agent.DeveloperID = parsed.Developer.ID
		agent.DeveloperName = parsed.Developer.Name
		agent.FunctionName = contract.Name
		agent.FunctionDescription = contract.Description
		agent.ResponseFormat = contract.ResponseFormat
		agent.Prompt = agent.Runtime.Prompt
		agent.Fixture = agent.Runtime.Fixture
		agent.InputSchema = contract.InputSchema
		agent.OutputSchema = contract.OutputSchema
		agent.RequiresWebSearch = agent.Runtime.RequiresWebSearch
		agent.AggregateMarkdown = agent.Runtime.Role == "root"
		agent.MinimumSources = 3
		agent.MaxOutputTokens = agent.Runtime.MaxOutputTokens
		if agent.MaxOutputTokens == 0 {
			if agent.AggregateMarkdown {
				agent.MaxOutputTokens = 2048
			} else {
				agent.MaxOutputTokens = 1536
			}
		}
	}
	sort.Slice(parsed.FunctionContracts, func(left, right int) bool {
		return parsed.FunctionContracts[left].Code < parsed.FunctionContracts[right].Code
	})
	return Catalog{parsed.ContractVersion, parsed.AgentVersion, parsed.Network, parsed.Asset, parsed.Developer.ID, parsed.Developer.Name, parsed.FunctionContracts, parsed.Agents}, nil
}

var atomicPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
var addressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var providerStrategies = map[string]struct{}{
	"lowest_price":        {},
	"latest_version":      {},
	"highest_reliability": {},
	"fastest":             {},
}

func positiveAtomic(value string) bool { return atomicPattern.MatchString(value) }
func evmAddress(value string) bool     { return addressPattern.MatchString(value) }

func isProviderStrategy(value string) bool {
	_, exists := providerStrategies[value]
	return exists
}

func validateFunctionContract(contract FunctionContract) error {
	if err := validateSchema(contract.InputSchema); err != nil {
		return fmt.Errorf("input schema: %w", err)
	}
	if contract.InputSchema["type"] != "object" {
		return fmt.Errorf("input schema root type must be object")
	}
	if err := validateSchema(contract.OutputSchema); err != nil {
		return fmt.Errorf("output schema: %w", err)
	}
	if !matchesResponseFormat(contract.ResponseFormat, contract.OutputSchema) {
		return fmt.Errorf("output schema does not match %s", contract.ResponseFormat)
	}
	return nil
}

func validateSchema(schema map[string]any) error {
	if schema == nil {
		return fmt.Errorf("is required")
	}
	encoded, err := json.Marshal(schema)
	if err != nil || len(encoded) > maxSchemaBytes {
		return fmt.Errorf("exceeds size limit")
	}
	return validateSchemaValue(schema, 1)
}

func validateSchemaValue(value any, depth int) error {
	if depth > maxSchemaDepth {
		return fmt.Errorf("exceeds nesting limit")
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if (key == "$ref" || key == "$dynamicRef") && (!isLocalReference(child)) {
				return fmt.Errorf("contains remote reference")
			}
			if err := validateSchemaValue(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := validateSchemaValue(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func isLocalReference(value any) bool {
	reference, ok := value.(string)
	return ok && strings.HasPrefix(reference, "#")
}

func matchesResponseFormat(responseFormat string, schema map[string]any) bool {
	switch responseFormat {
	case "TEXT", "MARKDOWN":
		return schema["type"] == "string"
	case "STRUCTURED":
		return matchesStructuredSchema(schema)
	case "JSON":
		return true
	default:
		return false
	}
}

func matchesStructuredSchema(schema map[string]any) bool {
	if schema["type"] != "object" {
		return false
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || !containsRequired(schema["required"], "title") || !containsRequired(schema["required"], "sections") {
		return false
	}
	title, titleOK := properties["title"].(map[string]any)
	sections, sectionsOK := properties["sections"].(map[string]any)
	if !titleOK || !sectionsOK || title["type"] != "string" || sections["type"] != "array" {
		return false
	}
	item, itemOK := sections["items"].(map[string]any)
	itemProperties, propertiesOK := item["properties"].(map[string]any)
	return itemOK && propertiesOK && item["type"] == "object" &&
		containsRequired(item["required"], "label") && containsRequired(item["required"], "value") &&
		matchesStringSchema(itemProperties["label"])
}

func containsRequired(value any, expected string) bool {
	values, ok := value.([]any)
	if !ok {
		return false
	}
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func matchesStringSchema(value any) bool {
	schema, ok := value.(map[string]any)
	return ok && schema["type"] == "string"
}
