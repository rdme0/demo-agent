package service

import (
	"context"
	"testing"

	"demo-agent/internal/agent/dto"
	"demo-agent/internal/agent/model"
	runtimeDTO "demo-agent/internal/runtime/dto"
)

func TestNewAgentRegistryRejectsDuplicateCodes(t *testing.T) {
	_, err := NewAgentRegistry(
		staticAgent{code: "duplicate"},
		staticAgent{code: "duplicate"},
	)
	if err == nil {
		t.Fatal("expected duplicate code to fail")
	}
}

func TestRootAgentResolvesRuntimeDependenciesWhenItNeedsThem(t *testing.T) {
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
	if response.Transport != dto.DemoTransport {
		t.Fatalf("unexpected transport: %#v", response.Transport)
	}
}

func TestGenericServiceDoesNotResolveDependenciesForSpecialists(t *testing.T) {
	registry, err := NewAgentRegistry(staticAgent{code: "specialist"})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	callback := &countingCallback{}
	service, err := NewAgentService(registry, callback)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if _, err := service.Invoke(context.Background(), "specialist", dto.InvocationRequest{Runtime: &runtimeDTO.Request{CallbackURL: "http://127.0.0.1:8080/runtime"}}, "Bearer token"); err != nil {
		t.Fatalf("invoke specialist: %v", err)
	}
	if callback.calls != 0 {
		t.Fatalf("generic service resolved specialist dependencies %d times", callback.calls)
	}
}

type callbackStub struct{}

func (callbackStub) Invoke(context.Context, runtimeDTO.Request, string) (map[string]any, error) {
	return map[string]any{"financial": "resolved"}, nil
}

type countingCallback struct{ calls int }

func (callback *countingCallback) Invoke(context.Context, runtimeDTO.Request, string) (map[string]any, error) {
	callback.calls++
	return nil, nil
}

type dependencyAwareAgent struct{}

type staticAgent struct {
	code string
}

func (agent staticAgent) Code() string {
	return agent.code
}

func (staticAgent) Invoke(context.Context, model.Invocation) (model.Result, error) {
	return model.Result{}, nil
}

func (dependencyAwareAgent) Code() string {
	return "dependency-aware"
}

func (dependencyAwareAgent) Invoke(ctx context.Context, invocation model.Invocation) (model.Result, error) {
	dependencyResults, err := invocation.ResolveDependencies(ctx)
	if err != nil {
		return model.Result{}, err
	}

	return model.Result{Output: dependencyResults["financial"], DependencyResults: dependencyResults}, nil
}
