package service

import (
	"context"
	"sort"
	"strings"

	"demo-agent/catalog"
	"demo-agent/internal/agent/model"
)

type FixtureAgent struct {
	definition catalog.Definition
}

func NewFixtureAgents() []model.Agent {
	definitions := catalog.Definitions()
	agents := make([]model.Agent, 0, len(definitions))
	for _, definition := range definitions {
		agents = append(agents, FixtureAgent{definition: definition})
	}
	return agents
}

func (agent FixtureAgent) Code() string {
	return agent.definition.Code
}

func (agent FixtureAgent) Invoke(ctx context.Context, invocation model.Invocation) (model.Result, error) {
	dependencyResults, err := agent.resolveDependencies(ctx, invocation)
	if err != nil {
		return model.Result{}, err
	}

	return model.Result{
		Output:            agent.output(dependencyResults),
		DependencyResults: dependencyResults,
	}, nil
}

func (agent FixtureAgent) resolveDependencies(ctx context.Context, invocation model.Invocation) (map[string]any, error) {
	if !agent.definition.AggregateMarkdown || invocation.ResolveDependencies == nil {
		return invocation.DependencyResults, nil
	}

	return invocation.ResolveDependencies(ctx)
}

func (agent FixtureAgent) output(dependencyResults map[string]any) any {
	if !agent.definition.AggregateMarkdown || len(dependencyResults) == 0 {
		return agent.definition.Fixture
	}

	markdown, ok := agent.definition.Fixture.(string)
	if !ok {
		return agent.definition.Fixture
	}

	codes := make([]string, 0, len(dependencyResults))
	for code := range dependencyResults {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	lines := []string{strings.TrimSpace(markdown), "", "## 함께 확인한 내용"}
	for _, code := range codes {
		lines = append(lines, "- "+code+": "+fixtureSummary(dependencyResults[code]))
	}

	return strings.Join(lines, "\n")
}

func fixtureSummary(result any) string {
	output, ok := result.(map[string]any)
	if !ok {
		return "결과를 확인했어요."
	}
	summary, ok := output["summary"].(string)
	if !ok || strings.TrimSpace(summary) == "" {
		return "결과를 확인했어요."
	}

	return strings.TrimSpace(summary)
}
