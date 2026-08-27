package service

import (
	"strings"
	"testing"

	"demo-agent/catalog"
)

func TestFixtureRootOutputSummarizesResolvedDependencies(t *testing.T) {
	agent := FixtureAgent{definition: catalog.Definition{
		AggregateMarkdown: true,
		Fixture:           "# 분석 결과",
	}}

	output := agent.output(map[string]any{
		"news": map[string]any{"summary": "시장 흐름은 중립적입니다."},
		"risk": map[string]any{"summary": "변동성을 주의해야 합니다."},
	})
	markdown, ok := output.(string)
	if !ok {
		t.Fatalf("expected Markdown output, got %#v", output)
	}
	if !strings.Contains(markdown, "## 함께 확인한 내용") || !strings.Contains(markdown, "news: 시장 흐름은 중립적입니다.") || !strings.Contains(markdown, "risk: 변동성을 주의해야 합니다.") {
		t.Fatalf("expected dependency summaries, got %s", markdown)
	}
}
