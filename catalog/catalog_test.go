package catalog

import (
	"strings"
	"testing"
)

func TestEmbeddedCatalogIsTheSingleValidatedDemoSource(t *testing.T) {
	source, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	if len(source.Agents) != 13 || len(source.FunctionContracts) != 12 {
		t.Fatalf("unexpected catalog shape: agents=%d contracts=%d", len(source.Agents), len(source.FunctionContracts))
	}
	rootCount := 0
	for _, agent := range source.Agents {
		if agent.AggregateMarkdown {
			rootCount++
			if len(agent.Dependencies) != 3 {
				t.Fatalf("root %q must declare three dependencies", agent.Code)
			}
			continue
		}
		if agent.RequiresWebSearch == false || agent.Fixture == nil {
			t.Fatalf("specialist %q must have runtime fixture and web-search role", agent.Code)
		}
	}
	if rootCount != 3 {
		t.Fatalf("expected three user-facing root agents, got %d", rootCount)
	}
}

func TestCatalogRejectsUnknownFieldAndDuplicateCode(t *testing.T) {
	if _, err := Parse(append(append([]byte(nil), embeddedAgents...), []byte("\nunknown: value\n")...)); err == nil {
		t.Fatal("expected unknown YAML field to fail")
	}
	duplicated := strings.Replace(string(embeddedAgents), "  - {code: financial-analysis,", "  - {code: investment-analysis,", 1)
	if _, err := Parse([]byte(duplicated)); err == nil || !strings.Contains(err.Error(), "duplicate agent code") {
		t.Fatalf("expected duplicate code to fail, got %v", err)
	}
}

func TestCatalogRejectsRemovedOrUnknownProviderStrategy(t *testing.T) {
	for _, strategy := range []string{"balanced", "unknown"} {
		invalid := strings.Replace(string(embeddedAgents), "strategy: lowest_price", "strategy: "+strategy, 1)
		if _, err := Parse([]byte(invalid)); err == nil || !strings.Contains(err.Error(), "invalid dependency") {
			t.Fatalf("expected %q strategy to fail, got %v", strategy, err)
		}
	}
}

func TestCatalogRejectsFunctionContractSchemaThatCannotReachSpring(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "non-object input root",
			content: strings.Replace(string(embeddedAgents), "inputSchema: {type: object, additionalProperties: true}", "inputSchema: {type: string}", 1),
		},
		{
			name:    "output format mismatch",
			content: strings.Replace(string(embeddedAgents), "outputSchema: {type: string}", "outputSchema: {type: object}", 1),
		},
		{
			name:    "remote reference",
			content: strings.Replace(string(embeddedAgents), "inputSchema: {type: object, additionalProperties: true}", "inputSchema: {type: object, \"$ref\": https://example.com/schema}", 1),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse([]byte(test.content)); err == nil {
				t.Fatal("expected invalid schema to fail")
			}
		})
	}
}

func TestSchemaValidationRejectsOversizedAndDeepSchemas(t *testing.T) {
	if err := validateSchema(map[string]any{"description": strings.Repeat("x", maxSchemaBytes)}); err == nil {
		t.Fatal("expected oversized schema to fail")
	}

	deep := map[string]any{}
	current := deep
	for range maxSchemaDepth {
		next := map[string]any{}
		current["nested"] = next
		current = next
	}
	if err := validateSchema(deep); err == nil {
		t.Fatal("expected deeply nested schema to fail")
	}
}
