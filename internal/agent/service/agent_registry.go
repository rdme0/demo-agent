package service

import (
	"context"
	"fmt"

	"demo-agent/internal/agent/model"
)

type AgentRegistry struct {
	agents map[string]model.Agent
}

func NewAgentRegistry(registeredAgents ...model.Agent) (*AgentRegistry, error) {
	agents := make(map[string]model.Agent, len(registeredAgents))

	for _, registeredAgent := range registeredAgents {
		if registeredAgent == nil {
			return nil, fmt.Errorf("registered agent must not be nil")
		}

		slug := registeredAgent.Slug()
		if slug == "" {
			return nil, fmt.Errorf("registered agent slug must not be empty")
		}
		if _, exists := agents[slug]; exists {
			return nil, fmt.Errorf("agent %q is already registered", slug)
		}

		agents[slug] = registeredAgent
	}

	return &AgentRegistry{agents: agents}, nil
}

func (registry *AgentRegistry) Invoke(ctx context.Context, slug string, invocation model.Invocation) (model.Result, error) {
	registeredAgent, exists := registry.agents[slug]
	if !exists {
		return model.Result{Output: map[string]any{"status": "unknown-agent"}}, nil
	}

	return registeredAgent.Invoke(ctx, invocation)
}
