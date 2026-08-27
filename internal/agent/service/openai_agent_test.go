package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agentClient "demo-agent/internal/agent/client"
	"demo-agent/internal/agent/model"
)

func TestOpenAIAgentEncodesInvocationAndAttachesVerifiedSources(t *testing.T) {
	responseClient := &responseClientStub{
		response: agentClient.ResponseResult{
			Output:  `{"summary":"재무 건전성은 보통 수준입니다.","keyMetrics":["매출"],"risks":["변동성"]}`,
			Sources: []agentClient.Source{{Title: "공식 공시", URL: "https://example.com/disclosure"}},
		},
	}
	agents, err := NewOpenAIAgents(responseClient)
	if err != nil {
		t.Fatalf("new OpenAI agents: %v", err)
	}

	result, err := agents[1].Invoke(context.Background(), model.Invocation{
		Input: map[string]any{
			"question": "삼성전자 투자 분석해줘",
			"input":    map[string]any{"ticker": "005930"},
		},
	})
	if err != nil {
		t.Fatalf("invoke OpenAI agent: %v", err)
	}

	output, ok := result.Output.(map[string]any)
	if !ok || output["summary"] != "재무 건전성은 보통 수준입니다." {
		t.Fatalf("unexpected output: %#v", result.Output)
	}
	if _, ok := output["sources"].([]agentClient.Source); !ok {
		t.Fatalf("expected verified sources: %#v", output)
	}
	if !responseClient.request.RequiresWebSearch {
		t.Fatal("expected financial agent to require web search")
	}
	if responseClient.request.Schema["type"] != "object" {
		t.Fatalf("unexpected schema: %#v", responseClient.request.Schema)
	}
	if !strings.Contains(responseClient.request.Instructions, "웹 검색") {
		t.Fatalf("expected Korean web-search instructions: %s", responseClient.request.Instructions)
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(responseClient.request.Input), &input); err != nil {
		t.Fatalf("decode agent input: %v", err)
	}
	contextInput := input["input"].(map[string]any)
	if contextInput["question"] != "삼성전자 투자 분석해줘" {
		t.Fatalf("question was not forwarded: %#v", input)
	}
}

func TestInvestmentAgentReturnsMarkdownWithThreeToFiveVerifiedSources(t *testing.T) {
	responseClient := &responseClientStub{
		response: agentClient.ResponseResult{Output: "# 삼성전자 투자 분석\n\n## 요약\n중립적으로 검토합니다.\n\n## 핵심 근거\n실적과 시장 흐름을 함께 확인해야 합니다.\n\n## 유의사항\n변동성이 있습니다."},
	}
	agents, err := NewOpenAIAgents(responseClient)
	if err != nil {
		t.Fatalf("new OpenAI agents: %v", err)
	}

	result, err := agents[0].Invoke(context.Background(), model.Invocation{
		Input: map[string]any{"question": "삼성전자 투자 분석해줘"},
		DependencyResults: map[string]any{
			"financial": dependencyResult([]agentClient.Source{{Title: "공시", URL: "https://example.com/a"}}),
			"news":      dependencyResult([]agentClient.Source{{Title: "뉴스", URL: "https://example.com/b"}}),
			"risk":      dependencyResult([]agentClient.Source{{Title: "리스크", URL: "https://example.com/c"}, {Title: "시장", URL: "https://example.com/d"}}),
		},
	})
	if err != nil {
		t.Fatalf("invoke investment agent: %v", err)
	}

	output, ok := result.Output.(string)
	if !ok || !strings.Contains(output, "## 출처") {
		t.Fatalf("expected Markdown investment output: %#v", result.Output)
	}
	if strings.Count(output, "https://example.com/") != 4 {
		t.Fatalf("expected four source links: %s", output)
	}
	if responseClient.request.RequiresWebSearch {
		t.Fatal("investment agent must aggregate dependency sources without searching again")
	}
	if responseClient.request.Schema != nil {
		t.Fatalf("investment Markdown response must not request JSON schema: %#v", responseClient.request.Schema)
	}
}

func TestInvestmentAgentRejectsInsufficientVerifiedSources(t *testing.T) {
	agents, err := NewOpenAIAgents(&responseClientStub{response: agentClient.ResponseResult{Output: "# 분석"}})
	if err != nil {
		t.Fatalf("new OpenAI agents: %v", err)
	}

	_, err = agents[0].Invoke(context.Background(), model.Invocation{
		DependencyResults: map[string]any{
			"financial": dependencyResult([]agentClient.Source{{Title: "공시", URL: "https://example.com/a"}}),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "at least 3") {
		t.Fatalf("expected insufficient source error, got %v", err)
	}
}

func TestInvestmentAgentRejectsUnsafeDependencySources(t *testing.T) {
	agents, err := NewOpenAIAgents(&responseClientStub{response: agentClient.ResponseResult{Output: "# 분석"}})
	if err != nil {
		t.Fatalf("new OpenAI agents: %v", err)
	}

	_, err = agents[0].Invoke(context.Background(), model.Invocation{
		DependencyResults: map[string]any{
			"financial": dependencyResult([]agentClient.Source{
				{Title: "공시", URL: "https://example.com/a#fragment"},
				{Title: "위조", URL: "http://example.com/b"},
				{Title: "삽입", URL: "https://example.com/c)\n[위조](https://attacker.example)"},
			}),
			"news": dependencyResult([]agentClient.Source{
				{Title: "뉴스", URL: "https://example.com/d"},
			}),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "at least 3") {
		t.Fatalf("expected unsafe sources to be rejected, got %v", err)
	}
}

func TestInvestmentAgentCanonicalizesDependencySourcesBeforeRendering(t *testing.T) {
	agents, err := NewOpenAIAgents(&responseClientStub{response: agentClient.ResponseResult{Output: "# 분석"}})
	if err != nil {
		t.Fatalf("new OpenAI agents: %v", err)
	}

	result, err := agents[0].Invoke(context.Background(), model.Invocation{
		DependencyResults: map[string]any{
			"financial": dependencyResult([]agentClient.Source{{Title: "공시", URL: "https://example.com/a#fragment"}}),
			"news":      dependencyResult([]agentClient.Source{{Title: "뉴스", URL: "https://example.com/b"}}),
			"risk":      dependencyResult([]agentClient.Source{{Title: "위험", URL: "https://example.com/c"}}),
		},
	})
	if err != nil {
		t.Fatalf("invoke investment agent: %v", err)
	}

	output := result.Output.(string)
	if strings.Contains(output, "#fragment") || !strings.Contains(output, "(<https://example.com/a>)") {
		t.Fatalf("expected canonical Markdown source URL, got %s", output)
	}
}

func TestOpenAIAgentRejectsInvalidJSONOutput(t *testing.T) {
	agents, err := NewOpenAIAgents(&responseClientStub{response: agentClient.ResponseResult{
		Output:  "not-json",
		Sources: []agentClient.Source{{Title: "공시", URL: "https://example.com/a"}},
	}})
	if err != nil {
		t.Fatalf("new OpenAI agents: %v", err)
	}

	_, err = agents[1].Invoke(context.Background(), model.Invocation{})
	if err == nil {
		t.Fatal("expected invalid JSON output to fail")
	}
}

func dependencyResult(sources []agentClient.Source) map[string]any {
	return map[string]any{
		"sources": sources,
	}
}

type responseClientStub struct {
	request  agentClient.ResponseRequest
	response agentClient.ResponseResult
}

func (stub *responseClientStub) Generate(_ context.Context, request agentClient.ResponseRequest) (agentClient.ResponseResult, error) {
	stub.request = request
	return stub.response, nil
}
