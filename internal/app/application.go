package app

import (
	"fmt"

	agentClient "demo-agent/internal/agent/client"
	agentController "demo-agent/internal/agent/controller"
	"demo-agent/internal/agent/model"
	agentService "demo-agent/internal/agent/service"
	"demo-agent/internal/config"
	"demo-agent/internal/payment/x402"
	runtimeClient "demo-agent/internal/runtime/client"
	"demo-agent/internal/server"

	"github.com/gin-gonic/gin"
	x402SDK "github.com/x402-foundation/x402/go/v2"
)

func New(configuration config.Config, facilitator x402SDK.FacilitatorClient) (*gin.Engine, error) {
	registeredAgents, err := registeredAgents(configuration)
	if err != nil {
		return nil, err
	}

	registry, err := agentService.NewAgentRegistry(registeredAgents...)
	if err != nil {
		return nil, fmt.Errorf("create agent registry: %w", err)
	}

	callbackClient := runtimeClient.NewCallbackClient()
	agentApplicationService, err := agentService.NewAgentService(registry, callbackClient)
	if err != nil {
		return nil, fmt.Errorf("create agent service: %w", err)
	}
	agentHTTPController, err := agentController.NewAgentController(agentApplicationService)
	if err != nil {
		return nil, fmt.Errorf("create agent controller: %w", err)
	}

	paymentMiddleware := x402.NewMiddleware(configuration.Payment, facilitator)

	return server.NewRouter(agentHTTPController, paymentMiddleware)
}

func registeredAgents(configuration config.Config) ([]model.Agent, error) {
	var roleAgents []model.Agent
	var err error
	switch configuration.AgentMode {
	case config.AgentModeFixture:
		roleAgents = agentService.NewFixtureAgents()
	case config.AgentModeOpenAI:
		responseClient := agentClient.NewOpenAIClient(
			configuration.OpenAI.APIKey,
			configuration.OpenAI.Model,
		)
		roleAgents, err = agentService.NewOpenAIAgents(responseClient)
	default:
		return nil, fmt.Errorf("unsupported DEMO_AGENT_MODE: %s", configuration.AgentMode)
	}
	if err != nil {
		return nil, err
	}

	return roleAgents, nil
}
