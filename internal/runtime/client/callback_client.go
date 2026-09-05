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
	newHTTPClient  func(string, string) *http.Client
	allowedOrigins map[string]struct{}
}

func NewCallbackClient(origins []string) (*CallbackClient, error) {
	allowedOrigins := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		parsed, err := parseExactOrigin(origin)
		if err != nil {
			return nil, err
		}
		allowedOrigins[parsed.String()] = struct{}{}
	}
	if len(allowedOrigins) == 0 {
		return nil, fmt.Errorf("runtime callback allowed origins are required")
	}
	return &CallbackClient{newHTTPClient: newPinnedHTTPClient, allowedOrigins: allowedOrigins}, nil
}

func (client *CallbackClient) Invoke(ctx context.Context, callback runtimeDTO.Request, authorization string) (map[string]any, error) {
	parsedURL, err := client.validateCallbackURL(callback.CallbackURL)
	if err != nil {
		return nil, err
	}

	results := make(map[string]any, len(callback.Dependencies))
	type dependencyResult struct {
		code   string
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
				dependency,
				authorization,
			)
			code := ""
			if len(dependency.CallPath) > 0 {
				code = dependency.CallPath[len(dependency.CallPath)-1]
			}
			resultChannel <- dependencyResult{code: code, output: output, err: err}
		}(dependency)
	}

	for range callback.Dependencies {
		result := <-resultChannel
		if result.err != nil {
			cancel()
			return nil, result.err
		}
		if result.code != "" {
			results[result.code] = result.output
		}
	}

	return results, nil
}

func (client *CallbackClient) invokeDependency(ctx context.Context, callbackURL *url.URL, dependency runtimeDTO.Dependency, authorization string) (any, error) {
	payload := map[string]any{
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

func (client *CallbackClient) validateCallbackURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("runtime callback URL is invalid")
	}
	if parsedURL.User != nil || parsedURL.Fragment != "" || parsedURL.RawQuery != "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("runtime callback URL is invalid")
	}
	origin, err := parseExactOrigin(parsedURL.Scheme + "://" + parsedURL.Host)
	if err != nil {
		return nil, err
	}
	if _, allowed := client.allowedOrigins[origin.String()]; !allowed {
		return nil, fmt.Errorf("runtime callback URL origin is not allowed")
	}
	return parsedURL, nil
}

func parseExactOrigin(rawOrigin string) (*url.URL, error) {
	parsed, err := url.Parse(rawOrigin)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("runtime callback origin must be an exact HTTP origin")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed, nil
}

func newPinnedHTTPClient(host string, port string) *http.Client {
	pinnedAddress, resolveErr := resolvePinnedAddress(host)
	if port == "" {
		port = "80"
	}

	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, _ string) (net.Conn, error) {
			if resolveErr != nil {
				return nil, resolveErr
			}

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

func resolvePinnedAddress(host string) (string, error) {
	normalizedHost := strings.ToLower(host)
	if normalizedHost == "localhost" || normalizedHost == "127.0.0.1" {
		return "127.0.0.1", nil
	}

	addresses, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("resolve runtime callback host: %w", err)
	}
	for _, address := range addresses {
		if address.To4() != nil {
			return address.String(), nil
		}
	}
	for _, address := range addresses {
		if address.To16() != nil {
			return address.String(), nil
		}
	}

	return "", fmt.Errorf("runtime callback host has no IP address")
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
