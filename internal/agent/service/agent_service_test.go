package service

import (
	"context"
	"testing"

	"demo-agent/internal/agent/dto"
	"demo-agent/internal/agent/model"
	runtimeDTO "demo-agent/internal/runtime/dto"
)

func TestNewAgentRegistryRejectsDuplicateSlugs(t *testing.T) {
	_, err := NewAgentRegistry(
		FixtureAgent{slug: "investment"},
		FixtureAgent{slug: "investment"},
	)
	if err == nil {
		t.Fatal("expected duplicate slug to fail")
	}
}

func TestAgentServiceResolvesRuntimeDependenciesBeforeInvocation(t *testing.T) {
	registry, err := NewAgentRegistry(dependencyAwareAgent{})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	service, err := NewAgentService(registry, callbackStub{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	response, err := service.Invoke(context.Background(), "dependency-aware", dto.InvocationRequest{
		Runtime: &runtimeDTO.Request{CallbackURL: "http://127.0.0.1/runtime"},
	}, "Bearer token")
	if err != nil {
		t.Fatalf("invoke agent: %v", err)
	}
	if response.DependencyResults["financial"] != "resolved" {
		t.Fatalf("unexpected dependency results: %#v", response.DependencyResults)
	}
	if response.Output != "resolved" {
		t.Fatalf("unexpected output: %#v", response.Output)
	}
}

type callbackStub struct{}

func (callbackStub) Invoke(context.Context, runtimeDTO.Request, string) (map[string]any, error) {
	return map[string]any{"financial": "resolved"}, nil
}

type dependencyAwareAgent struct{}

func (dependencyAwareAgent) Slug() string {
	return "dependency-aware"
}

func (dependencyAwareAgent) Invoke(_ context.Context, invocation model.Invocation) (model.Result, error) {
	return model.Result{Output: invocation.DependencyResults["financial"]}, nil
}
