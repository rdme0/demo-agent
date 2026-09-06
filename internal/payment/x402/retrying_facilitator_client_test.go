package x402

import (
	"context"
	"errors"
	"testing"
	"time"

	x402SDK "github.com/x402-foundation/x402/go/v2"
)

func TestRetryingFacilitatorClientRetriesExplicitEvmTransactionFailure(t *testing.T) {
	useFastSettlementRetryDelays(t)
	delegate := &facilitatorFake{
		settle: []settleResult{
			{response: failedExactEvmSettlement()},
			{response: failedExactEvmSettlement()},
			{response: &x402SDK.SettleResponse{Success: true, Transaction: validTransactionHash, Network: "eip155:84532"}},
		},
	}
	client := NewRetryingFacilitatorClient(delegate)

	response, err := client.Settle(context.Background(), []byte("payload"), []byte("requirements"))

	if err != nil {
		t.Fatalf("expected settlement success, got %v", err)
	}
	if response == nil || !response.Success {
		t.Fatalf("expected successful settlement response, got %#v", response)
	}
	if delegate.settleCalls != 3 {
		t.Fatalf("expected three settlement attempts, got %d", delegate.settleCalls)
	}
	if delegate.payloadChanges || delegate.requirementsChanges {
		t.Fatal("expected every retry to preserve the original payment payload and requirements")
	}
}

func TestRetryingFacilitatorClientStopsAfterTwoRetries(t *testing.T) {
	useFastSettlementRetryDelays(t)
	delegate := &facilitatorFake{
		settle: []settleResult{
			{response: failedExactEvmSettlement()},
			{response: failedExactEvmSettlement()},
			{response: failedExactEvmSettlement()},
		},
	}
	client := NewRetryingFacilitatorClient(delegate)

	response, err := client.Settle(context.Background(), []byte("payload"), []byte("requirements"))

	if err != nil || !isRetryableSettlementFailure(response, err) {
		t.Fatalf("expected final explicit settlement failure response, got response=%#v err=%v", response, err)
	}
	if delegate.settleCalls != 3 {
		t.Fatalf("expected three settlement attempts, got %d", delegate.settleCalls)
	}
}

func TestRetryingFacilitatorClientDoesNotRetryUncertainOrNonRetryableFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "transport failure", err: errors.New("facilitator returned 502")},
		{name: "timeout", err: context.DeadlineExceeded},
		{name: "502 with settlement error", err: failedExactEvmSettlementError()},
		{name: "insufficient funds", err: x402SDK.NewSettleError("insufficient_funds", "", "eip155:84532", "", "")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			delegate := &facilitatorFake{settle: []settleResult{{err: test.err}}}
			client := NewRetryingFacilitatorClient(delegate)

			_, err := client.Settle(context.Background(), []byte("payload"), []byte("requirements"))

			if !errors.Is(err, test.err) {
				t.Fatalf("expected original error, got %v", err)
			}
			if delegate.settleCalls != 1 {
				t.Fatalf("expected one settlement attempt, got %d", delegate.settleCalls)
			}
		})
	}
}

func TestRetryingFacilitatorClientStopsWhenRequestContextIsCanceled(t *testing.T) {
	delegate := &facilitatorFake{settle: []settleResult{{response: failedExactEvmSettlement()}}}
	client := NewRetryingFacilitatorClient(delegate)
	invocationContext, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Settle(invocationContext, []byte("payload"), []byte("requirements"))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got %v", err)
	}
	if delegate.settleCalls != 1 {
		t.Fatalf("expected no retry after cancellation, got %d calls", delegate.settleCalls)
	}
}

const validTransactionHash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type facilitatorFake struct {
	settle               []settleResult
	settleCalls          int
	payloadChanges       bool
	requirementsChanges  bool
	expectedPayload      []byte
	expectedRequirements []byte
}

type settleResult struct {
	response *x402SDK.SettleResponse
	err      error
}

func (fake *facilitatorFake) Verify(context.Context, []byte, []byte) (*x402SDK.VerifyResponse, error) {
	return nil, nil
}

func (fake *facilitatorFake) Settle(
	context context.Context,
	payload []byte,
	requirements []byte,
) (*x402SDK.SettleResponse, error) {
	if fake.settleCalls == 0 {
		fake.expectedPayload = append([]byte(nil), payload...)
		fake.expectedRequirements = append([]byte(nil), requirements...)
	} else {
		fake.payloadChanges = fake.payloadChanges || string(fake.expectedPayload) != string(payload)
		fake.requirementsChanges = fake.requirementsChanges || string(fake.expectedRequirements) != string(requirements)
	}

	result := fake.settle[fake.settleCalls]
	fake.settleCalls++
	return result.response, result.err
}

func (fake *facilitatorFake) GetSupported(context.Context) (x402SDK.SupportedResponse, error) {
	return x402SDK.SupportedResponse{}, nil
}

func failedExactEvmSettlement() *x402SDK.SettleResponse {
	return &x402SDK.SettleResponse{
		Success:     false,
		ErrorReason: retryableSettlementReason,
		Network:     "eip155:84532",
	}
}

func failedExactEvmSettlementError() error {
	return x402SDK.NewSettleError(retryableSettlementReason, "", "eip155:84532", "", "")
}

func useFastSettlementRetryDelays(t *testing.T) {
	t.Helper()
	previous := settlementRetryDelays
	settlementRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() {
		settlementRetryDelays = previous
	})
}
