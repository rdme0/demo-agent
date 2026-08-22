package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"demo-agent/internal/agent"
	"demo-agent/internal/config"
	"demo-agent/internal/runtime"

	"github.com/gin-gonic/gin"
	x402 "github.com/x402-foundation/x402/go/v2"
	x402http "github.com/x402-foundation/x402/go/v2/http"
	ginx402 "github.com/x402-foundation/x402/go/v2/http/gin"
	evm "github.com/x402-foundation/x402/go/v2/mechanisms/evm/exact/server"
)

const maxRequestBodyBytes = 1 << 20

type callbackInvoker interface {
	Invoke(context.Context, runtime.Request, string) (map[string]any, error)
}

type invocationRequest struct {
	Input   any              `json:"input,omitempty"`
	Runtime *runtime.Request `json:"runtime,omitempty"`
}

type invocationResponse struct {
	Agent             string         `json:"agent"`
	Output            any            `json:"output"`
	DependencyResults map[string]any `json:"dependencyResults"`
}

func New(configuration config.Config, agents *agent.Registry, callbackClient callbackInvoker, facilitator x402.FacilitatorClient) (*gin.Engine, error) {
	if agents == nil {
		return nil, fmt.Errorf("agent registry is required")
	}
	if callbackClient == nil {
		return nil, fmt.Errorf("runtime callback client is required")
	}

	application := gin.New()
	application.Use(gin.Recovery())
	application.Use(limitRequestBody())

	if configuration.Payment.Mode == config.PaymentModeX402 {
		if err := addX402Middleware(application, configuration.Payment, facilitator); err != nil {
			return nil, err
		}
	}

	application.GET("/health", func(requestContext *gin.Context) {
		requestContext.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	application.POST("/agents/:agent/invoke", func(requestContext *gin.Context) {
		var request invocationRequest
		if err := requestContext.ShouldBindJSON(&request); err != nil && err != io.EOF {
			requestContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid invocation request"})
			return
		}

		dependencyResults := map[string]any{}
		if request.Runtime != nil {
			resolved, err := callbackClient.Invoke(requestContext.Request.Context(), *request.Runtime, requestContext.GetHeader("Authorization"))
			if err != nil {
				requestContext.JSON(http.StatusBadGateway, gin.H{"error": "runtime callback failed"})
				return
			}
			dependencyResults = resolved
		}

		slug := requestContext.Param("agent")
		result, err := agents.Invoke(requestContext.Request.Context(), slug, agent.Invocation{
			Input:             request.Input,
			DependencyResults: dependencyResults,
		})
		if err != nil {
			requestContext.JSON(http.StatusInternalServerError, gin.H{"error": "agent invocation failed"})
			return
		}

		requestContext.JSON(http.StatusOK, invocationResponse{
			Agent:             slug,
			Output:            result.Output,
			DependencyResults: dependencyResults,
		})
	})

	return application, nil
}

func addX402Middleware(application *gin.Engine, payment config.PaymentConfig, facilitator x402.FacilitatorClient) error {
	if facilitator == nil {
		facilitator = x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{URL: payment.FacilitatorURL})
	}

	routes := make(x402http.RoutesConfig, len(payment.Agents))
	for slug, terms := range payment.Agents {
		routes["POST /agents/"+slug+"/invoke"] = x402http.RouteConfig{
			Accepts: x402http.PaymentOptions{
				{
					Scheme:  "exact",
					Network: config.BaseSepoliaNetwork,
					PayTo:   terms.PayTo,
					Price: map[string]any{
						"amount": terms.AmountAtomic,
						"asset":  config.BaseSepoliaUSDC,
						"extra": map[string]any{
							"assetTransferMethod": "eip3009",
						},
					},
				},
			},
			Description: slug + " demo agent invocation",
			MimeType:    "application/json",
		}
	}

	application.Use(ginx402.PaymentMiddlewareFromConfig(
		routes,
		ginx402.WithFacilitatorClient(facilitator),
		ginx402.WithScheme(config.BaseSepoliaNetwork, evm.NewExactEvmScheme()),
		ginx402.WithSyncFacilitatorOnStart(true),
		ginx402.WithTimeout(30*time.Second),
	))

	return nil
}

func limitRequestBody() gin.HandlerFunc {
	return func(requestContext *gin.Context) {
		requestContext.Request.Body = http.MaxBytesReader(requestContext.Writer, requestContext.Request.Body, maxRequestBodyBytes)
		requestContext.Next()
	}
}
