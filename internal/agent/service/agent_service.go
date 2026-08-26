package service

import (
	"context"
	"errors"
	"fmt"

	"demo-agent/internal/agent/dto"
	"demo-agent/internal/agent/model"
	runtimeDTO "demo-agent/internal/runtime/dto"
)

var ErrRuntimeCallback = errors.New("runtime callback failed")

type CallbackInvoker interface {
	Invoke(context.Context, runtimeDTO.Request, string) (map[string]any, error)
}

type AgentService struct {
	registry       *AgentRegistry
	callbackClient CallbackInvoker
}

func NewAgentService(registry *AgentRegistry, callbackClient CallbackInvoker) (*AgentService, error) {
	if registry == nil {
		return nil, fmt.Errorf("agent registry is required")
	}
	if callbackClient == nil {
		return nil, fmt.Errorf("runtime callback client is required")
	}

	return &AgentService{registry: registry, callbackClient: callbackClient}, nil
}

func (service *AgentService) Invoke(
	ctx context.Context,
	code string,
	request dto.InvocationRequest,
	authorization string,
) (dto.InvocationResponse, error) {
	resolveDependencies := service.dependencyResolver(request.Runtime, authorization)
	result, err := service.registry.Invoke(ctx, code, model.Invocation{
		Input:               request.Input,
		DependencyResults:   map[string]any{},
		ResolveDependencies: resolveDependencies,
	})
	if err != nil {
		return dto.InvocationResponse{}, err
	}

	return dto.InvocationResponse{
		Transport:         dto.DemoTransport,
		Agent:             code,
		Output:            result.Output,
		DependencyResults: result.DependencyResults,
	}, nil
}

func (service *AgentService) dependencyResolver(runtime *runtimeDTO.Request, authorization string) func(context.Context) (map[string]any, error) {
	if runtime == nil {
		return nil
	}

	return func(ctx context.Context) (map[string]any, error) {
		resolved, err := service.callbackClient.Invoke(ctx, *runtime, authorization)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRuntimeCallback, err)
		}

		return resolved, nil
	}
}
