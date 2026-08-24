package server

import (
	"fmt"
	"net/http"

	agentController "demo-agent/internal/agent/controller"

	"github.com/gin-gonic/gin"
)

func NewRouter(controller *agentController.AgentController, paymentMiddleware gin.HandlerFunc) (*gin.Engine, error) {
	if controller == nil {
		return nil, fmt.Errorf("agent controller is required")
	}

	application := gin.New()
	application.Use(gin.Recovery())
	application.Use(limitRequestBody())
	if paymentMiddleware != nil {
		application.Use(paymentMiddleware)
	}

	application.GET("/health", func(requestContext *gin.Context) {
		requestContext.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	controller.RegisterRoutes(application)

	return application, nil
}
