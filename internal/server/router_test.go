package server_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"demo-agent/internal/app"
	"demo-agent/internal/config"

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
		{slug: "investment", outputKey: "", want: "# 투자 분석 요약"},
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
		var payload map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode %s response: %v", testCase.slug, err)
		}
		output := payload["output"]
		if testCase.outputKey == "" {
			if value, ok := output.(string); !ok || !strings.Contains(value, testCase.want.(string)) {
				t.Fatalf("%s output mismatch: %#v", testCase.slug, output)
			}
			continue
		}
		values, ok := output.(map[string]any)
		if !ok || values[testCase.outputKey] != testCase.want {
			t.Fatalf("%s output mismatch: %#v", testCase.slug, output)
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
	application, err := app.New(config.Config{
		AgentMode: config.AgentModeFixture,
		Payment: config.PaymentConfig{
			Mode: config.PaymentModeX402,
			Agents: map[string]config.PaymentTerms{
				"investment": {AmountAtomic: "1000", PayTo: "0x0000000000000000000000000000000000000001"},
				"financial":  {AmountAtomic: "2000", PayTo: "0x0000000000000000000000000000000000000002"},
			},
		},
	}, facilitatorStub{})
	if err != nil {
		t.Fatalf("new x402 server: %v", err)
	}

	tests := []struct {
		slug   string
		amount string
		payTo  string
	}{
		{slug: "investment", amount: "1000", payTo: "0x0000000000000000000000000000000000000001"},
		{slug: "financial", amount: "2000", payTo: "0x0000000000000000000000000000000000000002"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodPost, "/agents/"+test.slug+"/invoke", nil)
		response := httptest.NewRecorder()
		application.ServeHTTP(response, request)

		if response.Code != http.StatusPaymentRequired {
			t.Fatalf("expected %s 402, got %d: %s", test.slug, response.Code, response.Body.String())
		}
		decoded, decodeErr := base64.StdEncoding.DecodeString(response.Header().Get("Payment-Required"))
		if decodeErr != nil {
			t.Fatalf("decode %s payment required: %v", test.slug, decodeErr)
		}
		var payload map[string]any
		if decodeErr := json.Unmarshal(decoded, &payload); decodeErr != nil {
			t.Fatalf("decode %s payment required JSON: %v", test.slug, decodeErr)
		}
		accepts := payload["accepts"].([]any)
		requirement := accepts[0].(map[string]any)
		if requirement["network"] != config.BaseSepoliaNetwork || requirement["asset"] != config.BaseSepoliaUSDC || requirement["amount"] != test.amount || requirement["payTo"] != test.payTo {
			t.Fatalf("unexpected %s payment requirement: %#v", test.slug, requirement)
		}
		extra, exists := requirement["extra"].(map[string]any)
		if !exists || extra["assetTransferMethod"] != "eip3009" {
			t.Fatalf("unexpected %s EIP-712 metadata: %#v", test.slug, extra)
		}
	}
}

func newSimulatedServer(t *testing.T) *gin.Engine {
	t.Helper()

	application, err := app.New(config.Config{
		AgentMode: config.AgentModeFixture,
		Payment:   config.PaymentConfig{Mode: config.PaymentModeSimulated},
	}, nil)
	if err != nil {
		t.Fatalf("new simulated server: %v", err)
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
