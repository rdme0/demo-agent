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
	slug string,
	request dto.InvocationRequest,
	authorization string,
) (dto.InvocationResponse, error) {
	dependencyResults := map[string]any{}
	if request.Runtime != nil {
		resolved, err := service.callbackClient.Invoke(ctx, *request.Runtime, authorization)
		if err != nil {
			return dto.InvocationResponse{}, fmt.Errorf("%w: %v", ErrRuntimeCallback, err)
		}
		dependencyResults = resolved
	}

	result, err := service.registry.Invoke(ctx, slug, model.Invocation{
		Input:             request.Input,
		DependencyResults: dependencyResults,
	})
	if err != nil {
		return dto.InvocationResponse{}, err
	}

	return dto.InvocationResponse{
		Agent:             slug,
		Output:            result.Output,
		DependencyResults: dependencyResults,
	}, nil
}
