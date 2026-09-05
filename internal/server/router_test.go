package server_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"demo-agent/catalog"
	"demo-agent/internal/agent/dto"
	"demo-agent/internal/app"
	"demo-agent/internal/config"

	"github.com/gin-gonic/gin"
	x402 "github.com/x402-foundation/x402/go/v2"
)

func TestServerRequiresX402ForRegisteredFixtureAgents(t *testing.T) {
	application := newX402Server(t)

	for _, code := range []string{"investment-analysis", "financial-analysis", "market-news-fast", "travel-safety"} {
		request := httptest.NewRequest(http.MethodPost, "/agents/"+code+"/invoke", nil)
		response := httptest.NewRecorder()
		application.ServeHTTP(response, request)

		if response.Code != http.StatusPaymentRequired {
			t.Fatalf("expected %s 402, got %d: %s", code, response.Code, response.Body.String())
		}
	}
}

func TestServerReturnsUnknownAgentResponse(t *testing.T) {
	application := newX402Server(t)
	request := httptest.NewRequest(http.MethodPost, "/agents/missing/invoke", nil)
	response := httptest.NewRecorder()

	application.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected unknown agent response, got %d", response.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode unknown agent response: %v", err)
	}
	if payload["transport"] != dto.DemoTransport || payload["output"].(map[string]any)["status"] != "unknown-agent" {
		t.Fatalf("unexpected unknown agent payload: %#v", payload)
	}
}

func TestServerReturnsHealthResponse(t *testing.T) {
	application := newX402Server(t)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	application.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != `{"status":"ok"}` {
		t.Fatalf("unexpected health response: %d %s", response.Code, response.Body.String())
	}
}

func TestServerProtectsConfiguredRoutesWithX402(t *testing.T) {
	application := newX402Server(t)

	tests := []struct {
		code   string
		amount string
		payTo  string
	}{
		{code: "investment-analysis", amount: "1000", payTo: "0xb011996927Bb5e62818Cd39D7B1587dD53Cff2A9"},
		{code: "financial-analysis", amount: "1000", payTo: "0x015AefEd87B781B7da124076d3A668870A5737b5"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodPost, "/agents/"+test.code+"/invoke", nil)
		response := httptest.NewRecorder()
		application.ServeHTTP(response, request)

		if response.Code != http.StatusPaymentRequired {
			t.Fatalf("expected %s 402, got %d: %s", test.code, response.Code, response.Body.String())
		}
		decoded, decodeErr := base64.StdEncoding.DecodeString(response.Header().Get("Payment-Required"))
		if decodeErr != nil {
			t.Fatalf("decode %s payment required: %v", test.code, decodeErr)
		}
		var payload map[string]any
		if decodeErr := json.Unmarshal(decoded, &payload); decodeErr != nil {
			t.Fatalf("decode %s payment required JSON: %v", test.code, decodeErr)
		}
		accepts := payload["accepts"].([]any)
		requirement := accepts[0].(map[string]any)
		if requirement["network"] != config.BaseSepoliaNetwork || requirement["asset"] != config.BaseSepoliaUSDC || requirement["amount"] != test.amount || requirement["payTo"] != test.payTo {
			t.Fatalf("unexpected %s payment requirement: %#v", test.code, requirement)
		}
		extra, exists := requirement["extra"].(map[string]any)
		if !exists || extra["assetTransferMethod"] != "eip3009" {
			t.Fatalf("unexpected %s EIP-712 metadata: %#v", test.code, extra)
		}
	}
}

func newX402Server(t *testing.T) *gin.Engine {
	t.Helper()

	terms := make(map[string]config.PaymentTerms, len(catalog.Definitions()))
	for _, definition := range catalog.Definitions() {
		terms[definition.Code] = config.PaymentTerms{
			AmountAtomic: definition.PriceAtomic,
			Asset:        catalog.Asset,
			PayTo:        definition.PayTo,
		}
	}

	application, err := app.New(config.Config{
		AgentMode: config.AgentModeFixture,
		Payment: config.PaymentConfig{
			FacilitatorURL: "https://facilitator.test",
			Agents:         terms,
		},
		Callback: config.CallbackConfig{AllowedOrigins: []string{"http://127.0.0.1:8080"}},
	}, facilitatorStub{})
	if err != nil {
		t.Fatalf("new x402 server: %v", err)
	}

	return application
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
