package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	runtimeDTO "demo-agent/internal/runtime/dto"
)

func TestCallbackClientPropagatesAuthorizationAndOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer invocation-token" {
			t.Fatalf("unexpected authorization: %s", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Idempotency-Key") == "" {
			t.Fatal("expected idempotency key")
		}

		payload := make(map[string]any)
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode callback payload: %v", err)
		}
		if _, exists := payload["parentStepId"]; exists {
			t.Fatal("callback payload must not include parentStepId; Spring derives the parent from the invocation token")
		}
		if payload["agentVersionId"] != "financial-v1" {
			t.Fatalf("unexpected agentVersionId: %#v", payload["agentVersionId"])
		}
		if _, exists := payload["callPath"]; !exists {
			t.Fatal("expected callback callPath")
		}
		_, _ = writer.Write([]byte(`{"isSuccess":true,"message":"success","errorCode":null,"result":{"stepId":"child-step","output":{"child":true},"costAtomic":"1000"}}`))
	}))
	defer server.Close()

	client := testCallbackClient(t, server.URL)
	results, err := client.Invoke(context.Background(), runtimeDTO.Request{
		ParentStepID: "step-1",
		CallbackURL:  server.URL,
		Dependencies: []runtimeDTO.Dependency{{AgentVersionID: "financial-v1", CallPath: []string{"investment", "financial"}}},
	}, "Bearer invocation-token")
	if err != nil {
		t.Fatalf("invoke callback: %v", err)
	}
	if results["financial"].(map[string]any)["child"] != true {
		t.Fatalf("unexpected callback output: %#v", results)
	}
}

func TestCallbackClientInvokesIndependentDependenciesConcurrently(t *testing.T) {
	var invocationCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		invocationCount.Add(1)
		time.Sleep(200 * time.Millisecond)
		_, _ = writer.Write([]byte(`{"isSuccess":true,"result":{"output":{"completed":true}}}`))
	}))
	defer server.Close()

	client := testCallbackClient(t, server.URL)
	startedAt := time.Now()
	results, err := client.Invoke(context.Background(), runtimeDTO.Request{
		CallbackURL: server.URL,
		Dependencies: []runtimeDTO.Dependency{
			{CallPath: []string{"investment", "financial"}},
			{CallPath: []string{"investment", "news"}},
			{CallPath: []string{"investment", "risk"}},
		},
	}, "")
	if err != nil {
		t.Fatalf("invoke concurrent callbacks: %v", err)
	}
	if invocationCount.Load() != 3 || len(results) != 3 {
		t.Fatalf("unexpected callback results: count=%d results=%#v", invocationCount.Load(), results)
	}
	if elapsed := time.Since(startedAt); elapsed >= 500*time.Millisecond {
		t.Fatalf("dependencies were not invoked concurrently: %s", elapsed)
	}
}

func TestCallbackClientPinsLocalhostToIPv4Loopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"isSuccess":true,"result":{"output":"pinned"}}`))
	}))
	defer server.Close()

	callbackURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	client := testCallbackClient(t, callbackURL)
	results, err := client.Invoke(context.Background(), runtimeDTO.Request{
		CallbackURL:  callbackURL,
		Dependencies: []runtimeDTO.Dependency{{CallPath: []string{"financial"}}},
	}, "")
	if err != nil {
		t.Fatalf("invoke pinned localhost callback: %v", err)
	}
	if results["financial"] != "pinned" {
		t.Fatalf("unexpected callback result: %#v", results)
	}
}

func TestCallbackClientRejectsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/redirect-target", http.StatusFound)
	}))
	defer server.Close()

	client := testCallbackClient(t, server.URL)
	_, err := client.Invoke(context.Background(), runtimeDTO.Request{
		CallbackURL:  server.URL,
		Dependencies: []runtimeDTO.Dependency{{CallPath: []string{"risk"}}},
	}, "")
	if err == nil {
		t.Fatal("expected redirect to fail")
	}
}

func TestCallbackClientRejectsOversizedRequest(t *testing.T) {
	client := testCallbackClient(t, "http://127.0.0.1")
	_, err := client.Invoke(context.Background(), runtimeDTO.Request{
		CallbackURL: "http://127.0.0.1/runtime",
		Dependencies: []runtimeDTO.Dependency{{
			CallPath: []string{"risk"},
			Input:    strings.Repeat("x", maxBodyBytes),
		}},
	}, "")
	if err == nil {
		t.Fatal("expected oversized request to fail")
	}
}

func TestCallbackClientSetsDeadlineOnCallbackRequest(t *testing.T) {
	client := testCallbackClient(t, "http://127.0.0.1")
	client.newHTTPClient = func(string, string) *http.Client {
		return &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
			deadline, exists := request.Context().Deadline()
			if !exists {
				t.Fatal("expected callback request deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > callbackTTL {
				t.Fatalf("unexpected callback deadline: %s", remaining)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"isSuccess":true,"result":{"output":true}}`)),
			}, nil
		})}
	}

	_, err := client.Invoke(context.Background(), runtimeDTO.Request{
		CallbackURL:  "http://127.0.0.1/runtime",
		Dependencies: []runtimeDTO.Dependency{{CallPath: []string{"risk"}}},
	}, "")
	if err != nil {
		t.Fatalf("invoke callback: %v", err)
	}
}

func TestCallbackClientRejectsNonLoopbackURL(t *testing.T) {
	client := testCallbackClient(t, "http://127.0.0.1:8080")
	_, err := client.Invoke(context.Background(), runtimeDTO.Request{CallbackURL: "http://example.com/runtime"}, "")
	if err == nil {
		t.Fatal("expected non-loopback callback to fail")
	}
}

func TestCallbackClientRejectsCredentialAndFragmentURLs(t *testing.T) {
	client := testCallbackClient(t, "http://127.0.0.1")

	for _, callbackURL := range []string{
		"http://token@127.0.0.1/runtime",
		"http://127.0.0.1/runtime#fragment",
	} {
		_, err := client.Invoke(context.Background(), runtimeDTO.Request{CallbackURL: callbackURL}, "")
		if err == nil {
			t.Fatalf("expected callback URL to fail: %s", callbackURL)
		}
	}
}

func TestCallbackClientRejectsOversizedResponse(t *testing.T) {
	client := testCallbackClient(t, "http://127.0.0.1")
	client.newHTTPClient = func(string, string) *http.Client {
		return &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxBodyBytes+1))),
			}, nil
		})}
	}

	_, err := client.Invoke(context.Background(), runtimeDTO.Request{
		CallbackURL:  "http://127.0.0.1/runtime",
		Dependencies: []runtimeDTO.Dependency{{CallPath: []string{"risk"}}},
	}, "")
	if err == nil {
		t.Fatal("expected oversized response to fail")
	}
}

func TestCallbackClientRejectsMissingCommonResponseResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"isSuccess":true,"result":null}`))
	}))
	defer server.Close()

	client := testCallbackClient(t, server.URL)
	_, err := client.Invoke(context.Background(), runtimeDTO.Request{
		CallbackURL:  server.URL,
		Dependencies: []runtimeDTO.Dependency{{CallPath: []string{"risk"}}},
	}, "")
	if err == nil {
		t.Fatal("expected missing callback result to fail")
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func testCallbackClient(t *testing.T, origin string) *CallbackClient {
	t.Helper()
	client, err := NewCallbackClient([]string{origin})
	if err != nil {
		t.Fatalf("new callback client: %v", err)
	}
	return client
}

func (roundTrip roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}
