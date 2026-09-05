package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"demo-agent/catalog"
	"gopkg.in/yaml.v3"
)

func TestCatalogVerificationInputsRenderForEveryAgent(t *testing.T) {
	source, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if len(source.Agents) != 13 {
		t.Fatalf("expected 13 catalog agents, got %d", len(source.Agents))
	}
	for _, agent := range source.Agents {
		if len(agent.VerificationInput) == 0 {
			t.Fatalf("agent %q has no deterministic verification input", agent.Code)
		}
		content, err := renderManifest(source, agent, mustURL(t, "http://demo-agent:8090"))
		if err != nil {
			t.Fatalf("render %q: %v", agent.Code, err)
		}
		var rendered manifest
		if err := yaml.Unmarshal([]byte(content), &rendered); err != nil {
			t.Fatalf("parse rendered %q: %v", agent.Code, err)
		}
		if len(rendered.Agent.VerificationInput) == 0 {
			t.Fatalf("rendered %q lost verification input", agent.Code)
		}
	}
}

func TestBootstrapOnlyBackfillsLegacyEmptyVerificationInput(t *testing.T) {
	active := agentResponse{Versions: []agentVersionResponse{{ID: "v1", Status: "ACTIVE", Readiness: &struct {
		Status string `json:"status"`
	}{Status: "UNVERIFIED"}}}}
	if !activeVersionNeedsBackfill(active, "v1", "apiVersion: agentstore/v1\nagent:\n  code: legacy\n") {
		t.Fatal("expected legacy active version to need backfill")
	}
	withInput := "apiVersion: agentstore/v1\nagent:\n  code: legacy\n  verificationInput:\n    query: stable\n"
	if activeVersionNeedsBackfill(active, "v1", withInput) {
		t.Fatal("non-null verification input must not be backfilled")
	}
	if !activeVersionIsUnverified(active, "v1") {
		t.Fatal("fixture should remain unverified")
	}
}

func TestActiveVersionSelectionUsesCatalogSemver(t *testing.T) {
	agent := agentResponse{Versions: []agentVersionResponse{
		{ID: "newer", Semver: "2.0.0", Status: "ACTIVE"},
		{ID: "catalog", Semver: "1.0.0", Status: "ACTIVE"},
	}}
	if got := activeVersionID(agent, "1.0.0"); got != "catalog" {
		t.Fatalf("expected catalog version, got %q", got)
	}
	if got := activeVersionID(agent, "3.0.0"); got != "" {
		t.Fatalf("expected no matching version, got %q", got)
	}
}

func TestBootstrapBackfillsAndVerifiesLegacyInputOnceOverHTTP(t *testing.T) {
	full, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	source := full
	source.FunctionContracts = []catalog.FunctionContract{full.FunctionContracts[0]}
	source.Agents = []catalog.Definition{full.Agents[0]}
	state := newRecoveryState()
	desiredContent, err := renderManifest(source, source.Agents[0], mustURL(t, "http://demo-agent:8090"))
	if err != nil {
		t.Fatalf("render desired manifest: %v", err)
	}
	state.desiredContent = desiredContent
	state.content = withoutVerificationInput(desiredContent)
	state.digest = digest(state.content)
	server := httptest.NewServer(http.HandlerFunc(state.handle))
	defer server.Close()
	client, err := NewWithAccessToken(server.URL, "http://demo-agent:8090", "fixture-demo-access")
	if err != nil {
		t.Fatalf("new bootstrap client: %v", err)
	}
	if err := client.Bootstrap(context.Background(), source); err != nil {
		t.Fatalf("legacy bootstrap: %v", err)
	}
	if err := client.Bootstrap(context.Background(), source); err != nil {
		t.Fatalf("repeat legacy bootstrap: %v", err)
	}
	if state.backfillCalls != 1 || state.verifyCalls != 1 {
		t.Fatalf("expected one backfill and one verify, got backfill=%d verify=%d", state.backfillCalls, state.verifyCalls)
	}
}

func TestBootstrapRetriesVerificationAfterBackfillSuccess(t *testing.T) {
	full, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	source := full
	source.FunctionContracts = []catalog.FunctionContract{full.FunctionContracts[0]}
	source.Agents = []catalog.Definition{full.Agents[0]}
	state := newRecoveryState()
	state.verifyFailures = 1
	desiredContent, err := renderManifest(source, source.Agents[0], mustURL(t, "http://demo-agent:8090"))
	if err != nil {
		t.Fatalf("render desired manifest: %v", err)
	}
	state.desiredContent = desiredContent
	state.content = withoutVerificationInput(desiredContent)
	state.digest = digest(state.content)
	server := httptest.NewServer(http.HandlerFunc(state.handle))
	defer server.Close()
	client, err := NewWithAccessToken(server.URL, "http://demo-agent:8090", "fixture-demo-access")
	if err != nil {
		t.Fatalf("new bootstrap client: %v", err)
	}
	if err := client.Bootstrap(context.Background(), source); err == nil {
		t.Fatal("expected first verification attempt to fail")
	}
	if err := client.Bootstrap(context.Background(), source); err != nil {
		t.Fatalf("retry bootstrap: %v", err)
	}
	if state.backfillCalls != 1 || state.verifyCalls != 2 {
		t.Fatalf("expected one backfill and two verification attempts, got backfill=%d verify=%d", state.backfillCalls, state.verifyCalls)
	}
}

func TestManifestVerificationInputComparisonDetectsCatalogDrift(t *testing.T) {
	existing := "apiVersion: agentstore/v1\nagent:\n  code: legacy\n  verificationInput:\n    query: old\n"
	desired := "apiVersion: agentstore/v1\nagent:\n  code: legacy\n  verificationInput:\n    query: new\n"
	if manifestVerificationInputMatches(existing, desired) {
		t.Fatal("different verification inputs must be treated as drift")
	}
	if !manifestVerificationInputMatches(desired, desired) {
		t.Fatal("identical verification inputs should match")
	}
}

type recoveryState struct {
	content        string
	digest         string
	desiredContent string
	backfillCalls  int
	verifyCalls    int
	verifyFailures int
	readiness      string
}

func newRecoveryState() *recoveryState {
	return &recoveryState{readiness: "UNVERIFIED"}
}

func withoutVerificationInput(content string) string {
	var value map[string]any
	if err := yaml.Unmarshal([]byte(content), &value); err != nil {
		return content
	}
	if agent, ok := value["agent"].(map[string]any); ok {
		delete(agent, "verificationInput")
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return content
	}
	return string(encoded)
}

func (state *recoveryState) handle(writer http.ResponseWriter, request *http.Request) {
	write := func(status int, result any) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]any{"isSuccess": status < 300, "result": result})
	}
	if request.Header.Get("Authorization") != "Bearer fixture-demo-access" {
		write(http.StatusUnauthorized, nil)
		return
	}
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/api/function-contracts":
		write(http.StatusOK, []functionContractResponse{})
	case request.Method == http.MethodPost && request.URL.Path == "/api/function-contracts":
		write(http.StatusCreated, map[string]any{})
	case request.Method == http.MethodPost && request.URL.Path == "/api/agent-manifests/validate":
		var body map[string]string
		_ = json.NewDecoder(request.Body).Decode(&body)
		state.desiredContent = body["content"]
		write(http.StatusOK, manifestResponse{SHA256: digest(state.desiredContent)})
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/api/agents/"):
		write(http.StatusOK, map[string]any{"versions": []map[string]any{{"id": "catalog-version", "semver": "1.0.0", "status": "ACTIVE", "readiness": map[string]string{"status": state.readiness}}}})
	case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/manifest"):
		write(http.StatusOK, manifestResponse{SHA256: state.digest, Content: state.content})
	case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/verification-input/backfill"):
		state.backfillCalls++
		state.content = state.desiredContent
		state.digest = digest(state.content)
		write(http.StatusOK, map[string]any{})
	case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/verify"):
		state.verifyCalls++
		if state.verifyFailures > 0 {
			state.verifyFailures--
			write(http.StatusServiceUnavailable, map[string]any{})
			return
		}
		state.readiness = "VERIFIED"
		write(http.StatusOK, map[string]any{})
	default:
		write(http.StatusNotFound, nil)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	return parsed
}

func TestBootstrapCreatesThenReusesCatalogAndRejectsDrift(t *testing.T) {
	state := newBootstrapState()
	server := httptest.NewServer(http.HandlerFunc(state.handle))
	defer server.Close()
	source, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	client, err := NewWithAccessToken(server.URL, "http://demo-agent:8090", "fixture-demo-access")
	if err != nil {
		t.Fatalf("new bootstrap client: %v", err)
	}
	if err := client.Bootstrap(context.Background(), source); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	if len(state.contracts) != 12 || len(state.agents) != 13 {
		t.Fatalf("unexpected bootstrap state: contracts=%d agents=%d", len(state.contracts), len(state.agents))
	}
	if err := client.Bootstrap(context.Background(), source); err != nil {
		t.Fatalf("repeat bootstrap: %v", err)
	}
	source.Agents[0].PriceAtomic = "9999"
	if err := client.Bootstrap(context.Background(), source); err == nil || !strings.Contains(err.Error(), "catalog drift") {
		t.Fatalf("expected catalog drift, got %v", err)
	}
}

type bootstrapState struct {
	contracts map[functionContractKey]functionContractResponse
	agents    map[string]agentState
	pending   map[string]string
}
type agentState struct {
	versionID string
	sha256    string
	content   string
}

func newBootstrapState() *bootstrapState {
	return &bootstrapState{contracts: map[functionContractKey]functionContractResponse{}, agents: map[string]agentState{}, pending: map[string]string{}}
}
func (state *bootstrapState) handle(writer http.ResponseWriter, request *http.Request) {
	write := func(status int, result any) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]any{"isSuccess": status < 300, "result": result})
	}
	if request.Header.Get("Authorization") != "Bearer fixture-demo-access" {
		write(http.StatusUnauthorized, nil)
		return
	}
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/api/function-contracts":
		values := make([]functionContractResponse, 0, len(state.contracts))
		for _, value := range state.contracts {
			values = append(values, value)
		}
		write(http.StatusOK, values)
	case request.Method == http.MethodPost && request.URL.Path == "/api/function-contracts":
		var value functionContractResponse
		_ = json.NewDecoder(request.Body).Decode(&value)
		state.contracts[functionContractKeyForResponse(value)] = value
		write(http.StatusCreated, value)
	case request.Method == http.MethodPost && request.URL.Path == "/api/agent-manifests/validate":
		var value map[string]string
		_ = json.NewDecoder(request.Body).Decode(&value)
		write(http.StatusOK, manifestResponse{SHA256: digest(value["content"])})
	case request.Method == http.MethodPost && request.URL.Path == "/api/agent-manifests":
		var value map[string]string
		_ = json.NewDecoder(request.Body).Decode(&value)
		code := manifestCode(value["content"])
		versionID := code + "-v1"
		state.pending[versionID] = digest(value["content"])
		write(http.StatusCreated, map[string]string{"versionId": versionID})
	case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/api/agent-versions/") && strings.HasSuffix(request.URL.Path, "/publish"):
		versionID := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/agent-versions/"), "/publish")
		code := strings.TrimSuffix(versionID, "-v1")
		state.agents[code] = agentState{versionID: versionID, sha256: state.pending[versionID]}
		write(http.StatusOK, map[string]string{})
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/api/agents/"):
		code := strings.TrimPrefix(request.URL.Path, "/api/agents/")
		agent, exists := state.agents[code]
		if !exists {
			write(http.StatusNotFound, nil)
			return
		}
		write(http.StatusOK, map[string]any{"versions": []map[string]string{{"id": agent.versionID, "semver": "1.0.0", "status": "ACTIVE"}}})
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/api/agent-versions/") && strings.HasSuffix(request.URL.Path, "/manifest"):
		versionID := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/agent-versions/"), "/manifest")
		code := strings.TrimSuffix(versionID, "-v1")
		write(http.StatusOK, manifestResponse{SHA256: state.agents[code].sha256, Content: state.agents[code].content})
	default:
		write(http.StatusNotFound, nil)
	}
}

func TestFunctionContractsKeepsDifferentVersionsOfTheSameCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/function-contracts" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"isSuccess": true,
			"result": []functionContractResponse{
				{Code: "finance.market", ContractVersion: "1.0.0"},
				{Code: "finance.market", ContractVersion: "2.0.0"},
			},
		})
	}))
	defer server.Close()

	client, err := NewWithAccessToken(server.URL, "http://demo-agent:8090", "fixture-demo-access")
	if err != nil {
		t.Fatalf("new bootstrap client: %v", err)
	}
	contracts, err := client.functionContracts(context.Background())
	if err != nil {
		t.Fatalf("list function contracts: %v", err)
	}
	if len(contracts) != 2 {
		t.Fatalf("expected both contract versions, got %d", len(contracts))
	}
	if _, found := contracts[functionContractKey{code: "finance.market", contractVersion: "2.0.0"}]; !found {
		t.Fatal("expected the 2.0.0 contract to remain addressable")
	}
}

func TestBootstrapRequiresHumanIssuedDemoAccessToken(t *testing.T) {
	client, err := NewWithAccessToken("http://agent-store:8080", "http://demo-agent:8090", "")
	if err != nil {
		t.Fatalf("new bootstrap client: %v", err)
	}
	if err := client.Bootstrap(context.Background(), catalog.Catalog{}); err == nil || !strings.Contains(err.Error(), "AGENT_STORE_DEMO_ACCESS_TOKEN") {
		t.Fatalf("expected missing demo access token error, got %v", err)
	}
}
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func manifestCode(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "code: ") {
			return strings.TrimPrefix(line, "code: ")
		}
	}
	return ""
}
