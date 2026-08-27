package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Host      string
	Port      int
	AgentMode string
	OpenAI    OpenAIConfig
	Payment   PaymentConfig
	Callback  CallbackConfig
}

type CallbackConfig struct {
	AllowedOrigins []string
}

type Overrides struct {
	Host      string
	Port      int
	AgentMode string
}

type fileConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Agent struct {
		Mode string `yaml:"mode"`
	} `yaml:"agent"`
	OpenAI struct {
		Model string `yaml:"model"`
	} `yaml:"openai"`
	Payment struct {
		FacilitatorURL string `yaml:"facilitatorUrl"`
	} `yaml:"payment"`
	Callback struct {
		AllowedOrigins []string `yaml:"allowedOrigins"`
	} `yaml:"callback"`
}

func Load(path string, overrides Overrides, lookup func(string) (string, bool)) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read application config: %w", err)
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	var values fileConfig
	if err := decoder.Decode(&values); err != nil {
		return Config{}, fmt.Errorf("decode application config: %w", err)
	}
	host := values.Server.Host
	if overrides.Host != "" {
		host = overrides.Host
	}
	port := values.Server.Port
	if overrides.Port != 0 {
		port = overrides.Port
	}
	if strings.TrimSpace(host) == "" {
		return Config{}, fmt.Errorf("server.host is required")
	}
	if err := validatePort(port); err != nil {
		return Config{}, err
	}
	agentMode := values.Agent.Mode
	if overrides.AgentMode != "" {
		agentMode = overrides.AgentMode
	}
	if agentMode != AgentModeFixture && agentMode != AgentModeOpenAI {
		return Config{}, fmt.Errorf("agent mode must be fixture or openai")
	}
	configuration := Config{Host: host, Port: port, AgentMode: agentMode}
	if agentMode == AgentModeOpenAI {
		apiKey, err := required(lookup, "OPEN_AI_KEY")
		if err != nil {
			return Config{}, err
		}
		if values.OpenAI.Model == "" {
			return Config{}, fmt.Errorf("openai.model is required")
		}
		configuration.OpenAI = OpenAIConfig{APIKey: apiKey, Model: values.OpenAI.Model}
	}
	if err := validateFacilitatorURL(values.Payment.FacilitatorURL); err != nil {
		return Config{}, err
	}
	if len(values.Callback.AllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("callback.allowedOrigins is required")
	}
	for _, origin := range values.Callback.AllowedOrigins {
		if err := validateCallbackOrigin(origin); err != nil {
			return Config{}, err
		}
	}
	agents, err := loadAgentTerms()
	if err != nil {
		return Config{}, err
	}
	configuration.Payment = PaymentConfig{FacilitatorURL: values.Payment.FacilitatorURL, Agents: agents}
	configuration.Callback = CallbackConfig{AllowedOrigins: append([]string(nil), values.Callback.AllowedOrigins...)}
	return configuration, nil
}

func (configuration Config) ListenAddress() string {
	return net.JoinHostPort(configuration.Host, strconv.Itoa(configuration.Port))
}

func required(lookup func(string) (string, bool), key string) (string, error) {
	value, exists := lookup(key)
	if !exists || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("server.port must be an integer between 1 and 65535")
	}
	return nil
}

func validateCallbackOrigin(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("callback.allowedOrigins must contain exact HTTP origins")
	}
	return nil
}
