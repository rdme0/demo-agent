package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"demo-agent/internal/agent"
	"demo-agent/internal/config"
	"demo-agent/internal/runtime"

	"github.com/gin-gonic/gin"
	x402 "github.com/x402-foundation/x402/go/v2"
)

func TestServerReturnsFixtureAndUnknownAgentResponses(t *testing.T) {
	application := newSimulatedServer(t)

	for _, testCase := range []struct {
		slug      string
		outputKey string
		want      any
	}{
		{slug: "investment", outputKey: "recommendation", want: "balanced"},
		{slug: "financial", outputKey: "revenueGrowth", want: 0.14},
		{slug: "news", outputKey: "sentiment", want: "positive"},
		{slug: "risk", outputKey: "riskLevel", want: "medium"},
		{slug: "missing", outputKey: "status", want: "unknown-agent"},
	} {
		request := httptest.NewRequest(http.MethodPost, "/agents/"+testCase.slug+"/invoke", nil)
		response := httptest.NewRecorder()
		application.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d", testCase.slug, response.Code)
		}
		var payload struct {
			Output map[string]any `json:"output"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode %s response: %v", testCase.slug, err)
		}
		if payload.Output[testCase.outputKey] != testCase.want {
			t.Fatalf("%s output mismatch: %#v", testCase.slug, payload.Output)
		}
	}
}

func TestServerReturnsHealthResponse(t *testing.T) {
	application := newSimulatedServer(t)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	application.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != `{"status":"ok"}` {
		t.Fatalf("unexpected health response: %d %s", response.Code, response.Body.String())
	}
}

func TestServerProtectsConfiguredRoutesWithX402(t *testing.T) {
	registry, err := agent.NewRegistry(agent.NewFixtureAgents()...)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	application, err := New(config.Config{
		Payment: config.PaymentConfig{
			Mode: config.PaymentModeX402,
			Agents: map[string]config.PaymentTerms{
				"investment": {AmountAtomic: "1000", PayTo: "0x0000000000000000000000000000000000000001"},
			},
		},
	}, registry, callbackStub{}, facilitatorStub{})
	if err != nil {
		t.Fatalf("new x402 server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/agents/investment/invoke", nil)
	response := httptest.NewRecorder()
	application.ServeHTTP(response, request)

	if response.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", response.Code, response.Body.String())
	}
	paymentRequired := response.Header().Get("Payment-Required")
	if paymentRequired == "" {
		t.Fatal("expected PAYMENT-REQUIRED header")
	}

	decoded, err := base64.StdEncoding.DecodeString(paymentRequired)
	if err != nil {
		t.Fatalf("decode payment required: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("decode payment required JSON: %v", err)
	}
	accepts := payload["accepts"].([]any)
	requirement := accepts[0].(map[string]any)
	if requirement["network"] != config.BaseSepoliaNetwork || requirement["asset"] != config.BaseSepoliaUSDC || requirement["amount"] != "1000" {
		t.Fatalf("unexpected payment requirement: %#v", requirement)
	}
	extra, exists := requirement["extra"].(map[string]any)
	if !exists {
		t.Fatalf("expected EIP-712 asset metadata: %#v", requirement)
	}
	if extra["assetTransferMethod"] != "eip3009" {
		t.Fatalf("unexpected asset transfer method: %#v", extra["assetTransferMethod"])
	}
}

func newSimulatedServer(t *testing.T) *gin.Engine {
	t.Helper()

	registry, err := agent.NewRegistry(agent.NewFixtureAgents()...)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	application, err := New(config.Config{Payment: config.PaymentConfig{Mode: config.PaymentModeSimulated}}, registry, callbackStub{}, nil)
	if err != nil {
		t.Fatalf("new simulated server: %v", err)
	}

	return application
}

type callbackStub struct{}

func (callbackStub) Invoke(context.Context, runtime.Request, string) (map[string]any, error) {
	return map[string]any{}, nil
}

type facilitatorStub struct{}

func (facilitatorStub) Verify(context.Context, []byte, []byte) (*x402.VerifyResponse, error) {
	return nil, nil
}

func (facilitatorStub) Settle(context.Context, []byte, []byte) (*x402.SettleResponse, error) {
	return nil, nil
}

func (facilitatorStub) GetSupported(context.Context) (x402.SupportedResponse, error) {
	return x402.SupportedResponse{
		Kinds:      []x402.SupportedKind{{X402Version: 2, Scheme: "exact", Network: config.BaseSepoliaNetwork}},
		Extensions: []string{},
		Signers:    map[string][]string{},
	}, nil
}
