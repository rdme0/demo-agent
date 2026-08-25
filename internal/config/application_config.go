package config

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	Host      string
	Port      int
	AgentMode string
	OpenAI    OpenAIConfig
	Payment   PaymentConfig
}

func LoadFromEnvironment() (Config, error) {
	return Load(os.LookupEnv)
}

func Load(lookup func(string) (string, bool)) (Config, error) {
	host, err := required(lookup, "DEMO_AGENT_HOST")
	if err != nil {
		return Config{}, err
	}

	portValue, err := required(lookup, "DEMO_AGENT_PORT")
	if err != nil {
		return Config{}, err
	}
	port, err := parsePort(portValue)
	if err != nil {
		return Config{}, err
	}

	mode, err := required(lookup, "DEMO_PAYMENT_MODE")
	if err != nil {
		return Config{}, err
	}
	if mode != PaymentModeSimulated && mode != PaymentModeX402 {
		return Config{}, fmt.Errorf("DEMO_PAYMENT_MODE must be simulated or x402")
	}

	agentMode, err := required(lookup, "DEMO_AGENT_MODE")
	if err != nil {
		return Config{}, err
	}
	if agentMode != AgentModeFixture && agentMode != AgentModeOpenAI {
		return Config{}, fmt.Errorf("DEMO_AGENT_MODE must be fixture or openai")
	}

	configuration := Config{
		Host:      host,
		Port:      port,
		AgentMode: agentMode,
		Payment:   PaymentConfig{Mode: mode},
	}
	if agentMode == AgentModeOpenAI {
		apiKey, err := required(lookup, "OPEN_AI_KEY")
		if err != nil {
			return Config{}, err
		}
		configuration.OpenAI = OpenAIConfig{
			APIKey: apiKey,
			Model:  OpenAIModel,
		}
	}

	if mode == PaymentModeSimulated {
		return configuration, nil
	}

	facilitatorURL, err := required(lookup, "X402_FACILITATOR_URL")
	if err != nil {
		return Config{}, err
	}
	if err := validateFacilitatorURL(facilitatorURL); err != nil {
		return Config{}, err
	}

	agents, err := loadAgentTerms()
	if err != nil {
		return Config{}, err
	}

	configuration.Payment.FacilitatorURL = facilitatorURL
	configuration.Payment.Agents = agents
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

func parsePort(value string) (int, error) {
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(value) {
		return 0, fmt.Errorf("DEMO_AGENT_PORT must be an integer between 1 and 65535")
	}

	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("DEMO_AGENT_PORT must be an integer between 1 and 65535")
	}

	return port, nil
}
