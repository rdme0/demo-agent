package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	paymentModel "demo-agent/internal/payment/model"
)

const (
	PaymentModeSimulated = "simulated"
	PaymentModeX402      = "x402"
	BaseSepoliaNetwork   = paymentModel.BaseSepoliaNetwork
	BaseSepoliaUSDC      = paymentModel.BaseSepoliaUSDC
)

var (
	atomicAmountPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
	addressPattern      = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
)

type PaymentConfig struct {
	Mode           string
	FacilitatorURL string
	Agents         map[string]paymentModel.PaymentTerms
}

type PaymentTerms = paymentModel.PaymentTerms

func loadAgentTerms(
	lookup func(string) (string, bool),
) (map[string]paymentModel.PaymentTerms, error) {
	slugs := []string{"investment", "financial", "news", "risk"}
	agents := make(map[string]paymentModel.PaymentTerms, len(slugs))

	for _, slug := range slugs {
		prefix := "DEMO_" + strings.ToUpper(strings.ReplaceAll(slug, "-", "_"))
		amount, err := required(lookup, prefix+"_PRICE_ATOMIC")
		if err != nil {
			return nil, err
		}
		payTo, err := required(lookup, prefix+"_PAY_TO")
		if err != nil {
			return nil, err
		}
		asset, err := required(lookup, prefix+"_ASSET")
		if err != nil {
			return nil, err
		}

		terms := paymentModel.PaymentTerms{AmountAtomic: amount, Asset: asset, PayTo: payTo}
		if err := validateTerms(terms); err != nil {
			return nil, fmt.Errorf("%s x402 terms: %w", slug, err)
		}

		agents[slug] = terms
	}

	return agents, nil
}

func validateTerms(terms paymentModel.PaymentTerms) error {
	if !atomicAmountPattern.MatchString(terms.AmountAtomic) {
		return fmt.Errorf("amount atomic must be a positive integer")
	}
	if !strings.EqualFold(terms.Asset, paymentModel.BaseSepoliaUSDC) {
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
