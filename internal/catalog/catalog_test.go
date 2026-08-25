package catalog

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestDefinitionsProvideTheCompleteDemoCatalog(t *testing.T) {
	definitions := Definitions()
	if len(definitions) != 13 {
		t.Fatalf("expected 13 catalog agents, got %d", len(definitions))
	}

	expectedCodes := map[string]struct{}{
		"investment-analysis": {}, "financial-analysis": {}, "market-news-fast": {}, "market-news-deep": {}, "investment-risk": {},
		"shopping-assistant": {}, "product-search": {}, "review-analysis": {}, "price-comparison": {},
		"travel-planner": {}, "destination-research": {}, "weather-forecast": {}, "travel-safety": {},
	}
	seenCodes := map[string]struct{}{}
	seenPayTos := map[string]struct{}{}
	address := regexp.MustCompile(`^0x[0-9a-f]{40}$`)
	for _, definition := range definitions {
		if _, exists := expectedCodes[definition.Code]; !exists {
			t.Fatalf("unexpected catalog code %q", definition.Code)
		}
		if _, exists := seenCodes[definition.Code]; exists {
			t.Fatalf("duplicate catalog code %q", definition.Code)
		}
		if _, exists := seenPayTos[definition.PayTo]; exists {
			t.Fatalf("duplicate payTo %q", definition.PayTo)
		}
		if !address.MatchString(definition.PayTo) || definition.PriceAtomic == "" {
			t.Fatalf("invalid public payment terms for %s", definition.Code)
		}
		seenCodes[definition.Code] = struct{}{}
		seenPayTos[definition.PayTo] = struct{}{}

		if definition.AggregateMarkdown {
			if definition.ResponseFormat != "MARKDOWN" || len(definition.Dependencies) != 3 || strings.Count(definition.Fixture.(string), "https://") < 3 {
				t.Fatalf("root %s must provide three dependencies and three fixture sources", definition.Code)
			}
			continue
		}
		fixture, ok := definition.Fixture.(map[string]any)
		if !ok || len(fixture["sources"].([]map[string]string)) < 3 {
			t.Fatalf("specialist %s must provide a deterministic source-backed fixture", definition.Code)
		}
	}
}

func TestInvestmentNewsProvidersShareAContractAndDifferInPrice(t *testing.T) {
	definitions := Definitions()
	var fast Definition
	var deep Definition
	for _, definition := range definitions {
		switch definition.Code {
		case "market-news-fast":
			fast = definition
		case "market-news-deep":
			deep = definition
		}
	}
	if fast.FunctionCode != "market-news-analysis" || fast.FunctionCode != deep.FunctionCode {
		t.Fatalf("market news providers must share their Function Contract")
	}
	fastPrice, _ := strconv.Atoi(fast.PriceAtomic)
	deepPrice, _ := strconv.Atoi(deep.PriceAtomic)
	if fastPrice >= deepPrice {
		t.Fatalf("fast provider must be the lowest-price demo candidate")
	}
}
