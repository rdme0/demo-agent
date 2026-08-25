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

func (agent FixtureAgent) Invoke(_ context.Context, _ model.Invocation) (model.Result, error) {
	return model.Result{Output: agent.definition.Fixture}, nil
}
