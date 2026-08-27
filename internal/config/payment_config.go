package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"demo-agent/catalog"
	paymentModel "demo-agent/internal/payment/model"
)

const (
	BaseSepoliaNetwork = paymentModel.BaseSepoliaNetwork
	BaseSepoliaUSDC    = paymentModel.BaseSepoliaUSDC
)

var (
	atomicAmountPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
	addressPattern      = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
)

type PaymentConfig struct {
	FacilitatorURL string
	Agents         map[string]paymentModel.PaymentTerms
}

type PaymentTerms = paymentModel.PaymentTerms

func loadAgentTerms() (map[string]paymentModel.PaymentTerms, error) {
	definitions := catalog.Definitions()
	agents := make(map[string]paymentModel.PaymentTerms, len(definitions))

	for _, definition := range definitions {
		terms := paymentModel.PaymentTerms{
			AmountAtomic: definition.PriceAtomic,
			Asset:        catalog.Asset,
			PayTo:        definition.PayTo,
		}
		if err := validateTerms(terms); err != nil {
			return nil, fmt.Errorf("%s x402 terms: %w", definition.Code, err)
		}

		agents[definition.Code] = terms
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
		return fmt.Errorf("payment.facilitatorUrl must be an HTTP(S) URL without credentials")
	}

	return nil
}
