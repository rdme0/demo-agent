package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsCheckedInConfigurationAndAppliesFlags(t *testing.T) {
	configuration, err := Load(configFile(t, baseConfig()), Overrides{Host: "0.0.0.0", Port: 9090, AgentMode: AgentModeFixture}, environment(nil))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if configuration.ListenAddress() != "0.0.0.0:9090" {
		t.Fatalf("unexpected listener address: %s", configuration.ListenAddress())
	}
	if len(configuration.Callback.AllowedOrigins) != 3 {
		t.Fatalf("unexpected callback origins: %#v", configuration.Callback.AllowedOrigins)
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

func TestLoadRejectsInvalidCallbackOrigin(t *testing.T) {
	content := baseConfig() + "callback:\n  allowedOrigins: [http://api:8080/path]\n"
	_, err := Load(configFile(t, content), Overrides{}, environment(nil))
	if err == nil {
		t.Fatal("expected callback path to fail")
	}
}

func baseConfig() string {
	return "server:\n  host: 127.0.0.1\n  port: 8090\nagent:\n  mode: fixture\nopenai:\n  model: " + OpenAIModel + "\npayment:\n  facilitatorUrl: https://facilitator.test\ncallback:\n  allowedOrigins:\n    - http://127.0.0.1:8080\n    - http://localhost:8080\n    - http://api:8080\n"
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
