package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	PaymentModeSimulated = "simulated"
	PaymentModeX402      = "x402"
	BaseSepoliaNetwork   = "eip155:84532"
	BaseSepoliaUSDC      = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
)

var (
	atomicAmountPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
	addressPattern      = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
)

type Config struct {
	Host    string
	Port    int
	Payment PaymentConfig
}

type PaymentConfig struct {
	Mode           string
	FacilitatorURL string
	Agents         map[string]PaymentTerms
}

type PaymentTerms struct {
	AmountAtomic string
	Asset        string
	PayTo        string
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

	configuration := Config{Host: host, Port: port, Payment: PaymentConfig{Mode: mode}}
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

	agents, err := loadAgentTerms(lookup)
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

func loadAgentTerms(lookup func(string) (string, bool)) (map[string]PaymentTerms, error) {
	slugs := []string{"investment", "financial", "news", "risk"}
	agents := make(map[string]PaymentTerms, len(slugs))

	for _, slug := range slugs {
		prefix := "DEMO_" + strings.ToUpper(slug)
		amount, err := required(lookup, prefix+"_PRICE_ATOMIC")
		if err != nil {
			return nil, err
		}
		payTo, err := required(lookup, prefix+"_PAY_TO")
		if err != nil {
			return nil, err
		}
		asset, _ := lookup(prefix + "_ASSET")

		terms := PaymentTerms{AmountAtomic: amount, Asset: asset, PayTo: payTo}
		if err := validateTerms(terms); err != nil {
			return nil, fmt.Errorf("%s x402 terms: %w", slug, err)
		}

		agents[slug] = terms
	}

	return agents, nil
}

func validateTerms(terms PaymentTerms) error {
	if !atomicAmountPattern.MatchString(terms.AmountAtomic) {
		return fmt.Errorf("amount atomic must be a positive integer")
	}
	if terms.Asset != "" && !strings.EqualFold(terms.Asset, BaseSepoliaUSDC) {
		return fmt.Errorf("asset must be official Base Sepolia USDC")
	}
	if !addressPattern.MatchString(terms.PayTo) {
		return fmt.Errorf("payTo must be an EVM address")
	}

	return nil
}

func validateFacilitatorURL(value string) error {
	parsedURL, err := url.ParseRequestURI(value)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" || parsedURL.User != nil {
		return fmt.Errorf("X402_FACILITATOR_URL must be an HTTP(S) URL without credentials")
	}

	return nil
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
