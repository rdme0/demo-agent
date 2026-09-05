package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"demo-agent/internal/agent/model"
	"demo-agent/internal/agent/service"
	runtimeDTO "demo-agent/internal/runtime/dto"

	"github.com/gin-gonic/gin"
)

func TestInvokeForwardsAuthorizationToRuntimeCallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	callback := &authorizationCallback{}
	registry, err := service.NewAgentRegistry(callbackAgent{})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	agentService, err := service.NewAgentService(registry, callback)
	if err != nil {
		t.Fatalf("new agent service: %v", err)
	}
	agentController, err := NewAgentController(agentService)
	if err != nil {
		t.Fatalf("new agent controller: %v", err)
	}

	router := gin.New()
	agentController.RegisterRoutes(router)
	request := httptest.NewRequest(
		http.MethodPost,
		"/agents/root/invoke",
		strings.NewReader(`{"runtime":{"callbackUrl":"http://127.0.0.1:8080/runtime"}}`),
	)
	request.Header.Set("Authorization", "Bearer invocation-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}
	if callback.authorization != "Bearer invocation-token" {
		t.Fatalf("unexpected callback authorization: %q", callback.authorization)
	}
}

type authorizationCallback struct {
	authorization string
}

func (callback *authorizationCallback) Invoke(
	_ context.Context,
	_ runtimeDTO.Request,
	authorization string,
) (map[string]any, error) {
	callback.authorization = authorization
	return map[string]any{}, nil
}

type callbackAgent struct{}

func (callbackAgent) Code() string {
	return "root"
}

func (callbackAgent) Invoke(ctx context.Context, invocation model.Invocation) (model.Result, error) {
	dependencies, err := invocation.ResolveDependencies(ctx)
	if err != nil {
		return model.Result{}, err
	}

	return model.Result{Output: dependencies}, nil
}
