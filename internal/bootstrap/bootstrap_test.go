package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"demo-agent/catalog"
)

func TestBootstrapCreatesThenReusesCatalogAndRejectsDrift(t *testing.T) {
	state := newBootstrapState()
	server := httptest.NewServer(http.HandlerFunc(state.handle))
	defer server.Close()
	source, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	client, err := New(server.URL, "http://demo-agent:8090")
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
		write(http.StatusOK, map[string]any{"versions": []map[string]string{{"id": agent.versionID, "status": "ACTIVE"}}})
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/api/agent-versions/") && strings.HasSuffix(request.URL.Path, "/manifest"):
		versionID := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/agent-versions/"), "/manifest")
		code := strings.TrimSuffix(versionID, "-v1")
		write(http.StatusOK, manifestResponse{SHA256: state.agents[code].sha256})
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

	client, err := New(server.URL, "http://demo-agent:8090")
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
