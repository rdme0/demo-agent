package runtime

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCallbackClientPropagatesAuthorizationAndOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer invocation-token" {
			t.Fatalf("unexpected authorization: %s", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Idempotency-Key") == "" {
			t.Fatal("expected idempotency key")
		}
		_, _ = writer.Write([]byte(`{"output":{"child":true}}`))
	}))
	defer server.Close()

	client := NewCallbackClient()
	results, err := client.Invoke(context.Background(), Request{
		ParentStepID: "step-1",
		CallbackURL:  server.URL,
		Dependencies: []Dependency{{AgentVersionID: "financial-v1", CallPath: []string{"investment", "financial"}}},
	}, "Bearer invocation-token")
	if err != nil {
		t.Fatalf("invoke callback: %v", err)
	}
	if results["financial"].(map[string]any)["child"] != true {
		t.Fatalf("unexpected callback output: %#v", results)
	}
}

func TestCallbackClientPinsLocalhostToIPv4Loopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"output":"pinned"}`))
	}))
	defer server.Close()

	callbackURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	client := NewCallbackClient()
	results, err := client.Invoke(context.Background(), Request{
		CallbackURL:  callbackURL,
		Dependencies: []Dependency{{CallPath: []string{"financial"}}},
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

	client := NewCallbackClient()
	_, err := client.Invoke(context.Background(), Request{
		CallbackURL:  server.URL,
		Dependencies: []Dependency{{CallPath: []string{"risk"}}},
	}, "")
	if err == nil {
		t.Fatal("expected redirect to fail")
	}
}

func TestCallbackClientRejectsOversizedRequest(t *testing.T) {
	client := NewCallbackClient()
	_, err := client.Invoke(context.Background(), Request{
		CallbackURL: "http://127.0.0.1/runtime",
		Dependencies: []Dependency{{
			CallPath: []string{"risk"},
			Input:    strings.Repeat("x", maxBodyBytes),
		}},
	}, "")
	if err == nil {
		t.Fatal("expected oversized request to fail")
	}
}

func TestCallbackClientSetsDeadlineOnCallbackRequest(t *testing.T) {
	client := NewCallbackClient()
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
				Body:       io.NopCloser(strings.NewReader(`{"output":true}`)),
			}, nil
		})}
	}

	_, err := client.Invoke(context.Background(), Request{
		CallbackURL:  "http://127.0.0.1/runtime",
		Dependencies: []Dependency{{CallPath: []string{"risk"}}},
	}, "")
	if err != nil {
		t.Fatalf("invoke callback: %v", err)
	}
}

func TestCallbackClientRejectsNonLoopbackURL(t *testing.T) {
	client := NewCallbackClient()
	_, err := client.Invoke(context.Background(), Request{CallbackURL: "http://example.com/runtime"}, "")
	if err == nil {
		t.Fatal("expected non-loopback callback to fail")
	}
}

func TestCallbackClientRejectsCredentialAndFragmentURLs(t *testing.T) {
	client := NewCallbackClient()

	for _, callbackURL := range []string{
		"http://token@127.0.0.1/runtime",
		"http://127.0.0.1/runtime#fragment",
	} {
		_, err := client.Invoke(context.Background(), Request{CallbackURL: callbackURL}, "")
		if err == nil {
			t.Fatalf("expected callback URL to fail: %s", callbackURL)
		}
	}
}

func TestCallbackClientRejectsOversizedResponse(t *testing.T) {
	client := NewCallbackClient()
	client.newHTTPClient = func(string, string) *http.Client {
		return &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxBodyBytes+1))),
			}, nil
		})}
	}

	_, err := client.Invoke(context.Background(), Request{
		CallbackURL:  "http://127.0.0.1/runtime",
		Dependencies: []Dependency{{CallPath: []string{"risk"}}},
	}, "")
	if err == nil {
		t.Fatal("expected oversized response to fail")
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (roundTrip roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}
