package config

import "testing"

func TestLoadRequiresExplicitListenerConfiguration(t *testing.T) {
	_, err := Load(func(string) (string, bool) {
		return "", false
	})
	if err == nil {
		t.Fatal("expected missing listener configuration to fail")
	}
}

func TestLoadAcceptsSimulatedMode(t *testing.T) {
	configuration, err := Load(environment(map[string]string{
		"DEMO_AGENT_HOST":   "127.0.0.1",
		"DEMO_AGENT_PORT":   "8090",
		"DEMO_AGENT_MODE":   AgentModeFixture,
		"DEMO_PAYMENT_MODE": PaymentModeSimulated,
	}))
	if err != nil {
		t.Fatalf("load simulated configuration: %v", err)
	}
	if configuration.ListenAddress() != "127.0.0.1:8090" {
		t.Fatalf("unexpected listener address: %s", configuration.ListenAddress())
	}
}

func TestLoadRejectsInvalidPaymentMode(t *testing.T) {
	_, err := Load(environment(map[string]string{
		"DEMO_AGENT_HOST":   "127.0.0.1",
		"DEMO_AGENT_PORT":   "8090",
		"DEMO_AGENT_MODE":   AgentModeFixture,
		"DEMO_PAYMENT_MODE": "live",
	}))
	if err == nil {
		t.Fatal("expected invalid payment mode to fail")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"0", "not-a-port"} {
		_, err := Load(environment(map[string]string{
			"DEMO_AGENT_HOST":   "127.0.0.1",
			"DEMO_AGENT_PORT":   port,
			"DEMO_AGENT_MODE":   AgentModeFixture,
			"DEMO_PAYMENT_MODE": PaymentModeSimulated,
		}))
		if err == nil {
			t.Fatalf("expected invalid port to fail: %s", port)
		}
	}
}

func TestLoadAcceptsOpenAIModeWithExplicitKey(t *testing.T) {
	configuration, err := Load(environment(map[string]string{
		"DEMO_AGENT_HOST":   "127.0.0.1",
		"DEMO_AGENT_PORT":   "8090",
		"DEMO_AGENT_MODE":   AgentModeOpenAI,
		"DEMO_PAYMENT_MODE": PaymentModeSimulated,
		"OPEN_AI_KEY":       "test-key",
	}))
	if err != nil {
		t.Fatalf("load OpenAI configuration: %v", err)
	}
	if configuration.OpenAI.Model != OpenAIModel {
		t.Fatalf("unexpected OpenAI model: %s", configuration.OpenAI.Model)
	}
}

func TestLoadRejectsOpenAIModeWithoutKey(t *testing.T) {
	_, err := Load(environment(map[string]string{
		"DEMO_AGENT_HOST":   "127.0.0.1",
		"DEMO_AGENT_PORT":   "8090",
		"DEMO_AGENT_MODE":   AgentModeOpenAI,
		"DEMO_PAYMENT_MODE": PaymentModeSimulated,
	}))
	if err == nil {
		t.Fatal("expected missing OpenAI key to fail")
	}
}

func TestLoadRejectsInvalidX402Terms(t *testing.T) {
	values := x402Environment()
	values["DEMO_INVESTMENT_PRICE_ATOMIC"] = "0"

	_, err := Load(environment(values))
	if err == nil {
		t.Fatal("expected invalid atomic amount to fail")
	}
}

func TestLoadRejectsInvalidFacilitatorURL(t *testing.T) {
	values := x402Environment()
	values["X402_FACILITATOR_URL"] = "file:///facilitator"

	_, err := Load(environment(values))
	if err == nil {
		t.Fatal("expected invalid facilitator URL to fail")
	}
}

func x402Environment() map[string]string {
	return map[string]string{
		"DEMO_AGENT_HOST":              "127.0.0.1",
		"DEMO_AGENT_PORT":              "8090",
		"DEMO_AGENT_MODE":              AgentModeFixture,
		"DEMO_PAYMENT_MODE":            PaymentModeX402,
		"X402_FACILITATOR_URL":         "https://facilitator.test",
		"DEMO_INVESTMENT_PRICE_ATOMIC": "1000",
		"DEMO_INVESTMENT_PAY_TO":       "0x0000000000000000000000000000000000000001",
		"DEMO_INVESTMENT_ASSET":        BaseSepoliaUSDC,
		"DEMO_FINANCIAL_PRICE_ATOMIC":  "1000",
		"DEMO_FINANCIAL_PAY_TO":        "0x0000000000000000000000000000000000000002",
		"DEMO_FINANCIAL_ASSET":         BaseSepoliaUSDC,
		"DEMO_NEWS_PRICE_ATOMIC":       "1000",
		"DEMO_NEWS_PAY_TO":             "0x0000000000000000000000000000000000000003",
		"DEMO_NEWS_ASSET":              BaseSepoliaUSDC,
		"DEMO_RISK_PRICE_ATOMIC":       "1000",
		"DEMO_RISK_PAY_TO":             "0x0000000000000000000000000000000000000004",
		"DEMO_RISK_ASSET":              BaseSepoliaUSDC,
	}
}

func environment(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, exists := values[key]
		return value, exists
	}
}
