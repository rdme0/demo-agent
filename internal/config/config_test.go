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

func TestLoadRequiresX402Configuration(t *testing.T) {
	configuration, err := Load(environment(x402Environment()))
	if err != nil {
		t.Fatalf("load x402 configuration: %v", err)
	}
	if configuration.ListenAddress() != "127.0.0.1:8090" {
		t.Fatalf("unexpected listener address: %s", configuration.ListenAddress())
	}
}

func TestLoadRejectsMissingFacilitatorURL(t *testing.T) {
	values := x402Environment()
	delete(values, "X402_FACILITATOR_URL")

	_, err := Load(environment(values))
	if err == nil {
		t.Fatal("expected missing facilitator URL to fail")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"0", "not-a-port"} {
		values := x402Environment()
		values["DEMO_AGENT_PORT"] = port

		_, err := Load(environment(values))
		if err == nil {
			t.Fatalf("expected invalid port to fail: %s", port)
		}
	}
}

func TestLoadAcceptsOpenAIModeWithExplicitKey(t *testing.T) {
	values := x402Environment()
	values["DEMO_AGENT_MODE"] = AgentModeOpenAI
	values["OPEN_AI_KEY"] = "test-key"

	configuration, err := Load(environment(values))
	if err != nil {
		t.Fatalf("load OpenAI configuration: %v", err)
	}
	if configuration.OpenAI.Model != OpenAIModel {
		t.Fatalf("unexpected OpenAI model: %s", configuration.OpenAI.Model)
	}
}

func TestLoadRejectsOpenAIModeWithoutKey(t *testing.T) {
	values := x402Environment()
	values["DEMO_AGENT_MODE"] = AgentModeOpenAI

	_, err := Load(environment(values))
	if err == nil {
		t.Fatal("expected missing OpenAI key to fail")
	}
}

func TestLoadRejectsInvalidX402Terms(t *testing.T) {
	agents, err := loadAgentTerms()
	if err != nil {
		t.Fatalf("load catalog terms: %v", err)
	}
	if len(agents) != 13 {
		t.Fatalf("expected 13 catalog terms, got %d", len(agents))
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
		"DEMO_AGENT_HOST":      "127.0.0.1",
		"DEMO_AGENT_PORT":      "8090",
		"DEMO_AGENT_MODE":      AgentModeFixture,
		"X402_FACILITATOR_URL": "https://facilitator.test",
	}
}

func environment(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, exists := values[key]
		return value, exists
	}
}
