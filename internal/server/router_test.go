package server_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"demo-agent/internal/agent/dto"
	"demo-agent/internal/app"
	"demo-agent/internal/config"

	"github.com/gin-gonic/gin"
	x402 "github.com/x402-foundation/x402/go/v2"
)

func TestServerReturnsFixtureAndUnknownAgentResponses(t *testing.T) {
	application := newSimulatedServer(t)

	for _, testCase := range []struct {
		code      string
		outputKey string
		want      any
	}{
		{code: "investment-analysis", outputKey: "", want: "# 투자 분석"},
		{code: "financial-analysis", outputKey: "summary", want: "재무 건전성은 보통 수준입니다."},
		{code: "market-news-fast", outputKey: "sentiment", want: "neutral"},
		{code: "travel-safety", outputKey: "riskLevel", want: "low"},
		{code: "missing", outputKey: "status", want: "unknown-agent"},
	} {
		request := httptest.NewRequest(http.MethodPost, "/agents/"+testCase.code+"/invoke", nil)
		response := httptest.NewRecorder()
		application.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d", testCase.code, response.Code)
		}
		var payload map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode %s response: %v", testCase.code, err)
		}
		if payload["transport"] != dto.DemoTransport {
			t.Fatalf("%s transport mismatch: %#v", testCase.code, payload)
		}
		output := payload["output"]
		if testCase.outputKey == "" {
			if value, ok := output.(string); !ok || !strings.Contains(value, testCase.want.(string)) {
				t.Fatalf("%s output mismatch: %#v", testCase.code, output)
			}
			continue
		}
		values, ok := output.(map[string]any)
		if !ok || values[testCase.outputKey] != testCase.want {
			t.Fatalf("%s output mismatch: %#v", testCase.code, output)
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
				"investment-analysis": {AmountAtomic: "1000", PayTo: "0x0000000000000000000000000000000000000101"},
				"financial-analysis":  {AmountAtomic: "1000", PayTo: "0x0000000000000000000000000000000000000102"},
			},
		},
	}, facilitatorStub{})
	if err != nil {
		t.Fatalf("new x402 server: %v", err)
	}

	tests := []struct {
		code   string
		amount string
		payTo  string
	}{
		{code: "investment-analysis", amount: "1000", payTo: "0x0000000000000000000000000000000000000101"},
		{code: "financial-analysis", amount: "1000", payTo: "0x0000000000000000000000000000000000000102"},
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
