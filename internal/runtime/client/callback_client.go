package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	runtimeDTO "demo-agent/internal/runtime/dto"
)

const (
	maxBodyBytes = 1 << 20
	callbackTTL  = 30 * time.Second
)

type CallbackClient struct {
	newHTTPClient func(string, string) *http.Client
}

func NewCallbackClient() *CallbackClient {
	return &CallbackClient{newHTTPClient: newPinnedHTTPClient}
}

func (client *CallbackClient) Invoke(ctx context.Context, callback runtimeDTO.Request, authorization string) (map[string]any, error) {
	parsedURL, err := validateCallbackURL(callback.CallbackURL)
	if err != nil {
		return nil, err
	}

	results := make(map[string]any, len(callback.Dependencies))
	type dependencyResult struct {
		slug   string
		output any
		err    error
	}
	resultChannel := make(chan dependencyResult, len(callback.Dependencies))
	callbackContext, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, dependency := range callback.Dependencies {
		go func(dependency runtimeDTO.Dependency) {
			output, err := client.invokeDependency(
				callbackContext,
				parsedURL,
				callback.ParentStepID,
				dependency,
				authorization,
			)
			slug := ""
			if len(dependency.CallPath) > 0 {
				slug = dependency.CallPath[len(dependency.CallPath)-1]
			}
			resultChannel <- dependencyResult{slug: slug, output: output, err: err}
		}(dependency)
	}

	for range callback.Dependencies {
		result := <-resultChannel
		if result.err != nil {
			cancel()
			return nil, result.err
		}
		if result.slug != "" {
			results[result.slug] = result.output
		}
	}

	return results, nil
}

func (client *CallbackClient) invokeDependency(ctx context.Context, callbackURL *url.URL, parentStepID string, dependency runtimeDTO.Dependency, authorization string) (any, error) {
	payload := map[string]any{
		"parentStepId":   parentStepID,
		"agentVersionId": dependency.AgentVersionID,
		"callPath":       dependency.CallPath,
		"input":          dependency.Input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal runtime callback payload: %w", err)
	}
	if len(body) > maxBodyBytes {
		return nil, fmt.Errorf("runtime callback request exceeds 1MB")
	}

	requestContext, cancel := context.WithTimeout(ctx, callbackTTL)
	defer cancel()

	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, callbackURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create runtime callback request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Idempotency-Key", newUUID())

	httpClient := client.newHTTPClient(callbackURL.Hostname(), callbackURL.Port())
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("runtime callback request failed: %w", err)
	}
	defer response.Body.Close()

	if response.ContentLength > maxBodyBytes {
		return nil, fmt.Errorf("runtime callback response exceeds 1MB")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("runtime callback failed: %d", response.StatusCode)
	}

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read runtime callback response: %w", err)
	}
	if len(responseBody) > maxBodyBytes {
		return nil, fmt.Errorf("runtime callback response exceeds 1MB")
	}

	var decoded runtimeDTO.CallbackResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode runtime callback response: %w", err)
	}
	if !decoded.IsSuccess || decoded.Result == nil {
		return nil, fmt.Errorf("runtime callback response is unsuccessful or missing result")
	}

	return decoded.Result.Output, nil
}

func validateCallbackURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("runtime callback URL is invalid")
	}

	host := strings.ToLower(parsedURL.Hostname())
	if parsedURL.Scheme != "http" || (host != "127.0.0.1" && host != "localhost") || parsedURL.User != nil || parsedURL.Fragment != "" {
		return nil, fmt.Errorf("runtime callback URL must be an HTTP loopback URL")
	}

	return parsedURL, nil
}

func newPinnedHTTPClient(host string, port string) *http.Client {
	pinnedAddress := "127.0.0.1"
	if strings.ToLower(host) == "127.0.0.1" {
		pinnedAddress = host
	}
	if port == "" {
		port = "80"
	}

	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, net.JoinHostPort(pinnedAddress, port))
		},
	}

	return &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("runtime callback redirects are not allowed")
		},
	}
}

func newUUID() string {
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		panic("secure random UUID generation failed")
	}

	identifier[6] = identifier[6]&0x0f | 0x40
	identifier[8] = identifier[8]&0x3f | 0x80
	encoded := hex.EncodeToString(identifier)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}
