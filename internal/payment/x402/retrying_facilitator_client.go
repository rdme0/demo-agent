package x402

import (
	"context"
	"log"
	"time"

	x402SDK "github.com/x402-foundation/x402/go/v2"
)

const retryableSettlementReason = "invalid_exact_evm_transaction_failed"

var settlementRetryDelays = []time.Duration{250 * time.Millisecond, 750 * time.Millisecond}

type RetryingFacilitatorClient struct {
	delegate x402SDK.FacilitatorClient
}

func NewRetryingFacilitatorClient(delegate x402SDK.FacilitatorClient) *RetryingFacilitatorClient {
	return &RetryingFacilitatorClient{delegate: delegate}
}

func (client *RetryingFacilitatorClient) Verify(
	context context.Context,
	payloadBytes []byte,
	requirementsBytes []byte,
) (*x402SDK.VerifyResponse, error) {
	return client.delegate.Verify(context, payloadBytes, requirementsBytes)
}

func (client *RetryingFacilitatorClient) Settle(
	context context.Context,
	payloadBytes []byte,
	requirementsBytes []byte,
) (*x402SDK.SettleResponse, error) {
	for attempt := 0; ; attempt++ {
		response, err := client.delegate.Settle(context, payloadBytes, requirementsBytes)
		if !isRetryableSettlementFailure(response, err) || attempt == len(settlementRetryDelays) {
			return response, err
		}

		log.Printf(
			"x402 facilitator settle retry: attempt=%d/%d reason=%s",
			attempt+2,
			len(settlementRetryDelays)+1,
			retryableSettlementReason,
		)
		if err := waitForSettlementRetry(context, settlementRetryDelays[attempt]); err != nil {
			return nil, err
		}
	}
}

func (client *RetryingFacilitatorClient) GetSupported(context context.Context) (x402SDK.SupportedResponse, error) {
	return client.delegate.GetSupported(context)
}

func isRetryableSettlementFailure(response *x402SDK.SettleResponse, err error) bool {
	return err == nil &&
		response != nil &&
		!response.Success &&
		response.ErrorReason == retryableSettlementReason
}

func waitForSettlementRetry(context context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-context.Done():
		return context.Err()
	case <-timer.C:
		return nil
	}
}
