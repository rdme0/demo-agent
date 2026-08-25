package controller

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"demo-agent/internal/agent/dto"
	"demo-agent/internal/agent/service"

	"github.com/gin-gonic/gin"
)

type AgentController struct {
	service *service.AgentService
}

func NewAgentController(agentService *service.AgentService) (*AgentController, error) {
	if agentService == nil {
		return nil, fmt.Errorf("agent service is required")
	}

	return &AgentController{service: agentService}, nil
}

func (controller *AgentController) RegisterRoutes(router gin.IRouter) {
	router.POST("/agents/:code/invoke", controller.invoke)
}

func (controller *AgentController) invoke(requestContext *gin.Context) {
	var request dto.InvocationRequest
	if err := requestContext.ShouldBindJSON(&request); err != nil && err != io.EOF {
		requestContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid invocation request"})
		return
	}

	response, err := controller.service.Invoke(
		requestContext.Request.Context(),
		requestContext.Param("code"),
		request,
		requestContext.GetHeader("Authorization"),
	)
	if err != nil {
		log.Printf("agent invocation failed agentCode=%s: %v", requestContext.Param("code"), err)
		if errors.Is(err, service.ErrRuntimeCallback) {
			requestContext.JSON(http.StatusBadGateway, gin.H{"error": "runtime callback failed"})
			return
		}

		requestContext.JSON(http.StatusInternalServerError, gin.H{"error": "agent invocation failed"})
		return
	}

	requestContext.JSON(http.StatusOK, response)
}
