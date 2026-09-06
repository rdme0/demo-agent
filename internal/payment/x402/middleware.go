package x402

import (
	"strings"

	"demo-agent/internal/config"
	paymentModel "demo-agent/internal/payment/model"

	x402 "github.com/x402-foundation/x402/go/v2"
	x402http "github.com/x402-foundation/x402/go/v2/http"
	ginx402 "github.com/x402-foundation/x402/go/v2/http/gin"
	evm "github.com/x402-foundation/x402/go/v2/mechanisms/evm/exact/server"

	"github.com/gin-gonic/gin"
)

func NewMiddleware(payment config.PaymentConfig, publicBaseURL string, facilitator x402.FacilitatorClient) gin.HandlerFunc {
	if facilitator == nil {
		facilitator = x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{URL: payment.FacilitatorURL})
	}

	routes := make(x402http.RoutesConfig, len(payment.Agents))
	for code, terms := range payment.Agents {
		resourceURL := ""
		if publicBaseURL != "" {
			resourceURL = strings.TrimRight(publicBaseURL, "/") + "/agents/" + code + "/invoke"
		}
		routes["POST /agents/"+code+"/invoke"] = x402http.RouteConfig{
			Resource: resourceURL,
			Accepts: x402http.PaymentOptions{
				{
					Scheme:  "exact",
					Network: paymentModel.BaseSepoliaNetwork,
					PayTo:   terms.PayTo,
					Price: map[string]any{
						"amount": terms.AmountAtomic,
						"asset":  paymentModel.BaseSepoliaUSDC,
						"extra": map[string]any{
							"assetTransferMethod": "eip3009",
						},
					},
				},
			},
			Description: code + " demo agent invocation",
			MimeType:    "application/json",
		}
	}

	return ginx402.PaymentMiddlewareFromConfig(
		routes,
		ginx402.WithFacilitatorClient(facilitator),
		ginx402.WithScheme(paymentModel.BaseSepoliaNetwork, evm.NewExactEvmScheme()),
		ginx402.WithSyncFacilitatorOnStart(true),
		ginx402.WithTimeout(payment.InvocationTimeout),
	)
}
