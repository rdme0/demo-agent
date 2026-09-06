package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadReadsCheckedInConfigurationAndAppliesFlags(t *testing.T) {
	configuration, err := Load(configFile(t, baseConfig()), Overrides{Host: "0.0.0.0", Port: 9090, AgentMode: AgentModeFixture}, environment(nil))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if configuration.ListenAddress() != "0.0.0.0:9090" {
		t.Fatalf("unexpected listener address: %s", configuration.ListenAddress())
	}
	if configuration.PublicBaseURL != "" {
		t.Fatalf("unexpected default public base URL: %q", configuration.PublicBaseURL)
	}
	if len(configuration.Callback.AllowedOrigins) != 3 {
		t.Fatalf("unexpected callback origins: %#v", configuration.Callback.AllowedOrigins)
	}
	if configuration.Payment.PerDepthTimeout != 30*time.Second {
		t.Fatalf("unexpected payment per-depth timeout: %s", configuration.Payment.PerDepthTimeout)
	}
	if configuration.Payment.MaxDependencyDepth != 5 {
		t.Fatalf("unexpected payment maximum dependency depth: %d", configuration.Payment.MaxDependencyDepth)
	}
	if configuration.Payment.InvocationTimeout != 150*time.Second {
		t.Fatalf("unexpected payment invocation timeout: %s", configuration.Payment.InvocationTimeout)
	}
}

func TestLoadReadsPublicBaseURLFromEnvironment(t *testing.T) {
	configuration, err := Load(
		configFile(t, baseConfig()),
		Overrides{},
		environment(map[string]string{"DEMO_AGENT_PUBLIC_BASE_URL": "https://demo-agent.example.test/"}),
	)
	if err != nil {
		t.Fatalf("load public base URL: %v", err)
	}
	if configuration.PublicBaseURL != "https://demo-agent.example.test" {
		t.Fatalf("unexpected public base URL: %q", configuration.PublicBaseURL)
	}
}

func TestLoadRejectsPublicBaseURLPath(t *testing.T) {
	_, err := Load(
		configFile(t, baseConfig()),
		Overrides{},
		environment(map[string]string{"DEMO_AGENT_PUBLIC_BASE_URL": "https://demo-agent.example.test/base"}),
	)
	if err == nil {
		t.Fatal("expected public base URL path to fail")
	}
}

func TestLoadRejectsUnknownYAMLFields(t *testing.T) {
	_, err := Load(configFile(t, baseConfig()+"unknown: value\n"), Overrides{}, environment(nil))
	if err == nil {
		t.Fatal("expected unknown YAML field to fail")
	}
}

func TestLoadRequiresOpenAIKeyOnlyForOpenAIMode(t *testing.T) {
	_, err := Load(configFile(t, baseConfig()), Overrides{AgentMode: AgentModeOpenAI}, environment(nil))
	if err == nil {
		t.Fatal("expected missing OpenAI key to fail")
	}
	configuration, err := Load(configFile(t, baseConfig()), Overrides{AgentMode: AgentModeOpenAI}, environment(map[string]string{"OPEN_AI_KEY": "test-key"}))
	if err != nil || configuration.OpenAI.Model != OpenAIModel {
		t.Fatalf("load OpenAI config: %v %#v", err, configuration.OpenAI)
	}
}

func TestLoadUsesDemoAgentModeEnvironmentWhenFlagIsAbsent(t *testing.T) {
	configuration, err := Load(
		configFile(t, baseConfig()),
		Overrides{},
		environment(map[string]string{
			"DEMO_AGENT_MODE": "openai",
			"OPEN_AI_KEY":     "test-key",
		}),
	)
	if err != nil {
		t.Fatalf("load OpenAI config from environment: %v", err)
	}
	if configuration.AgentMode != AgentModeOpenAI {
		t.Fatalf("unexpected agent mode: %s", configuration.AgentMode)
	}
}

func TestLoadFlagOverridesDemoAgentModeEnvironment(t *testing.T) {
	configuration, err := Load(
		configFile(t, baseConfig()),
		Overrides{AgentMode: AgentModeFixture},
		environment(map[string]string{"DEMO_AGENT_MODE": "openai"}),
	)
	if err != nil {
		t.Fatalf("load fixture config from flag: %v", err)
	}
	if configuration.AgentMode != AgentModeFixture {
		t.Fatalf("unexpected agent mode: %s", configuration.AgentMode)
	}
}

func TestLoadRejectsInvalidCallbackOrigin(t *testing.T) {
	content := baseConfig() + "callback:\n  allowedOrigins: [http://api:8080/path]\n"
	_, err := Load(configFile(t, content), Overrides{}, environment(nil))
	if err == nil {
		t.Fatal("expected callback path to fail")
	}
}

func TestLoadAcceptsHTTPSCallbackOrigin(t *testing.T) {
	content := strings.Replace(
		baseConfig(),
		"    - http://127.0.0.1:8080\n    - http://localhost:8080\n    - http://api:8080",
		"    - https://api.example.test",
		1,
	)
	if _, err := Load(configFile(t, content), Overrides{}, environment(nil)); err != nil {
		t.Fatalf("load HTTPS callback origin: %v", err)
	}
}

func TestLoadRejectsNonPositivePaymentPerDepthTimeout(t *testing.T) {
	content := strings.Replace(baseConfig(), "perDepthTimeoutSeconds: 30", "perDepthTimeoutSeconds: 0", 1)
	_, err := Load(configFile(t, content), Overrides{}, environment(nil))
	if err == nil {
		t.Fatal("expected non-positive payment per-depth timeout to fail")
	}
}

func TestLoadRejectsMismatchedExecutionGraphDepth(t *testing.T) {
	content := strings.Replace(baseConfig(), "maxDependencyDepth: 5", "maxDependencyDepth: 6", 1)
	_, err := Load(configFile(t, content), Overrides{}, environment(nil))
	if err == nil {
		t.Fatal("expected mismatched execution graph depth to fail")
	}
}

func baseConfig() string {
	return "server:\n  host: 127.0.0.1\n  port: 8090\nagent:\n  mode: fixture\nopenai:\n  model: " + OpenAIModel + "\npayment:\n  facilitatorUrl: https://facilitator.test\n  perDepthTimeoutSeconds: 30\n  maxDependencyDepth: 5\ncallback:\n  allowedOrigins:\n    - http://127.0.0.1:8080\n    - http://localhost:8080\n    - http://api:8080\n"
}

func configFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "application.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func environment(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) { value, exists := values[key]; return value, exists }
}
