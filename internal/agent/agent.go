package agent

import (
	"context"
	"fmt"
)

type Invocation struct {
	Input             any
	DependencyResults map[string]any
}

type Result struct {
	Output any
}

type Agent interface {
	Slug() string
	Invoke(context.Context, Invocation) (Result, error)
}

type Registry struct {
	agents map[string]Agent
}

func NewRegistry(registeredAgents ...Agent) (*Registry, error) {
	agents := make(map[string]Agent, len(registeredAgents))

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

	return &Registry{agents: agents}, nil
}

func (registry *Registry) Invoke(ctx context.Context, slug string, invocation Invocation) (Result, error) {
	registeredAgent, exists := registry.agents[slug]
	if !exists {
		return Result{Output: map[string]any{"status": "unknown-agent"}}, nil
	}

	return registeredAgent.Invoke(ctx, invocation)
}
