package service

import (
	"context"

	"demo-agent/internal/agent/model"
	"demo-agent/internal/catalog"
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

	return model.Result{Output: agent.definition.Fixture, DependencyResults: dependencyResults}, nil
}

func (agent FixtureAgent) resolveDependencies(ctx context.Context, invocation model.Invocation) (map[string]any, error) {
	if !agent.definition.AggregateMarkdown || invocation.ResolveDependencies == nil {
		return invocation.DependencyResults, nil
	}

	return invocation.ResolveDependencies(ctx)
}
