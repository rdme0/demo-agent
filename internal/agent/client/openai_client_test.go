package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go/option"
)

func TestOpenAIClientGeneratesStructuredWebSearchResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/responses" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header")
		}

		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "gpt-5.6-luna" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		if instructions, ok := payload["instructions"].(string); !ok || !strings.Contains(instructions, "당신은") {
			t.Fatalf("expected Korean instructions: %#v", payload["instructions"])
		}
		text, exists := payload["text"].(map[string]any)
		if !exists {
			t.Fatalf("missing structured output config: %#v", payload)
		}
		format, exists := text["format"].(map[string]any)
		if !exists || format["type"] != "json_schema" {
			t.Fatalf("unexpected structured output config: %#v", text)
		}
		tools, exists := payload["tools"].([]any)
		if !exists || len(tools) != 1 || tools[0].(map[string]any)["type"] != "web_search_preview" {
			t.Fatalf("expected web search tool: %#v", payload["tools"])
		}
		if payload["tool_choice"] != "required" {
			t.Fatalf("expected required web search tool choice: %#v", payload["tool_choice"])
		}
		include, exists := payload["include"].([]any)
		if !exists || len(include) != 1 || include[0] != webSearchSourcesInclude {
			t.Fatalf("expected web search sources include: %#v", payload["include"])
		}

		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{
            "id":"resp_test",
            "object":"response",
            "created_at":0,
            "status":"completed",
            "model":"gpt-5.6-luna",
            "output":[{
                "type":"web_search_call",
                "id":"web_search_test",
                "status":"completed",
                "action":{"type":"search","sources":[{"type":"url","url":"https://example.com/search-source"}]}
            }, {
                "type":"message",
                "id":"msg_test",
                "status":"completed",
                "role":"assistant",
                "content":[{
                    "type":"output_text",
                    "text":"{\"recommendation\":\"balanced\",\"score\":0.82}",
                    "annotations":[{
                        "type":"url_citation",
                        "title":"공식 공시",
                        "url":"https://example.com/disclosure"
                    }]
                }]
            }]
        }`))
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key", "gpt-5.6-luna", option.WithBaseURL(server.URL))
	output, err := client.Generate(context.Background(), ResponseRequest{
		Instructions:      "당신은 투자 분석 에이전트입니다. JSON만 반환하세요.",
		Input:             `{"input":"test"}`,
		RequiresWebSearch: true,
		Schema: map[string]any{
			"type": "object",
		},
	})
	if err != nil {
		t.Fatalf("generate response: %v", err)
	}
	if output.Output != `{"recommendation":"balanced","score":0.82}` {
		t.Fatalf("unexpected output: %s", output.Output)
	}
	if len(output.Sources) != 2 || output.Sources[0].URL != "https://example.com/search-source" || output.Sources[1].URL != "https://example.com/disclosure" {
		t.Fatalf("unexpected sources: %#v", output.Sources)
	}
}

func TestNormalizeSchemaForOpenAIRemovesUnsupportedFormatsWithoutMutatingContract(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sources": []any{
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"url": map[string]any{
							"type":   "string",
							"format": "uri",
						},
					},
				},
			},
		},
	}

	normalized := normalizeSchemaForOpenAI(schema)
	normalizedProperties := normalized["properties"].(map[string]any)
	normalizedSources := normalizedProperties["sources"].([]any)
	normalizedSource := normalizedSources[0].(map[string]any)
	normalizedSourceProperties := normalizedSource["properties"].(map[string]any)
	normalizedURL := normalizedSourceProperties["url"].(map[string]any)
	if _, exists := normalizedURL["format"]; exists {
		t.Fatalf("OpenAI schema retained unsupported format: %#v", normalizedURL)
	}

	originalProperties := schema["properties"].(map[string]any)
	originalSources := originalProperties["sources"].([]any)
	originalSource := originalSources[0].(map[string]any)
	originalURL := originalSource["properties"].(map[string]any)["url"].(map[string]any)
	if originalURL["format"] != "uri" {
		t.Fatalf("contract schema was mutated: %#v", originalURL)
	}
}

func TestOpenAIClientRejectsWebSearchWithoutCompletedCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{
            "id":"resp_test",
            "object":"response",
            "created_at":0,
            "status":"completed",
            "model":"gpt-5.6-luna",
            "output":[{
                "type":"message",
                "id":"msg_test",
                "status":"completed",
                "role":"assistant",
                "content":[{
                    "type":"output_text",
                    "text":"{}",
                    "annotations":[]
                }]
            }]
        }`))
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key", "gpt-5.6-luna", option.WithBaseURL(server.URL))
	_, err := client.Generate(context.Background(), ResponseRequest{
		Instructions:      "웹 검색을 수행하세요.",
		Input:             "{}",
		RequiresWebSearch: true,
	})
	if err == nil || !strings.Contains(err.Error(), "did not complete web search") {
		t.Fatalf("expected completed web search error, got %v", err)
	}
}

func TestOpenAIClientRejectsIncompleteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{
            "id":"resp_test",
            "object":"response",
            "created_at":0,
            "status":"incomplete",
            "incomplete_details":{"reason":"max_output_tokens"},
            "model":"gpt-5.6-luna",
            "output":[{
                "type":"message",
                "id":"msg_test",
                "status":"completed",
                "role":"assistant",
                "content":[{"type":"output_text","text":"# 잘린 분석"}]
            }]
        }`))
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key", "gpt-5.6-luna", option.WithBaseURL(server.URL))
	_, err := client.Generate(context.Background(), ResponseRequest{
		Instructions: "한국어 Markdown만 반환하세요.",
		Input:        "{}",
	})
	if err == nil || !strings.Contains(err.Error(), "did not complete") || !strings.Contains(err.Error(), "max_output_tokens") {
		t.Fatalf("expected incomplete response error, got %v", err)
	}
}

func TestSourceFromAnnotationRejectsUnsafeURLsAndNormalizesFragments(t *testing.T) {
	if _, ok := sourceFromAnnotation("unsafe", "http://example.com"); ok {
		t.Fatal("expected non-HTTPS source to be rejected")
	}

	source, ok := sourceFromAnnotation("", "https://example.com/report#section")
	if !ok || source.Title != "example.com" || source.URL != "https://example.com/report" {
		t.Fatalf("unexpected normalized source: %#v, %t", source, ok)
	}

	source, ok = sourceFromAnnotation("[공식]\n자료", "https://example.com/report")
	if !ok || source.Title != "공식 자료" {
		t.Fatalf("unexpected normalized source title: %#v, %t", source, ok)
	}
	if _, ok := sourceFromAnnotation("\x00", "https://example.com/report"); ok {
		t.Fatal("expected control-character source title to be rejected")
	}
}
