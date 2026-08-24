package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	agentClient "demo-agent/internal/agent/client"
	"demo-agent/internal/agent/model"
)

const (
	investmentAgentSlug  = "investment"
	minimumSourceCount   = 3
	maximumSourceCount   = 5
	analysisTokenLimit   = 4096
	investmentTokenLimit = 4096
)

type OpenAIAgent struct {
	slug              string
	instructions      string
	outputSchema      map[string]any
	requiresWebSearch bool
	responseClient    agentClient.ResponseClient
}

func NewOpenAIAgents(responseClient agentClient.ResponseClient) ([]model.Agent, error) {
	if responseClient == nil {
		return nil, fmt.Errorf("OpenAI response client is required")
	}

	return []model.Agent{
		newOpenAIAgent(investmentAgentSlug, responseClient),
		newOpenAIAgent("financial", responseClient),
		newOpenAIAgent("news", responseClient),
		newOpenAIAgent("risk", responseClient),
	}, nil
}

func newOpenAIAgent(slug string, responseClient agentClient.ResponseClient) OpenAIAgent {
	return OpenAIAgent{
		slug:              slug,
		instructions:      instructionsFor(slug),
		outputSchema:      schemaFor(slug),
		requiresWebSearch: slug != investmentAgentSlug,
		responseClient:    responseClient,
	}
}

func (agent OpenAIAgent) Slug() string {
	return agent.slug
}

func (agent OpenAIAgent) Invoke(ctx context.Context, invocation model.Invocation) (model.Result, error) {
	input, err := json.Marshal(map[string]any{
		"input":             invocation.Input,
		"dependencyResults": invocation.DependencyResults,
	})
	if err != nil {
		return model.Result{}, fmt.Errorf("encode %s agent input: %w", agent.slug, err)
	}

	response, err := agent.responseClient.Generate(ctx, agentClient.ResponseRequest{
		Instructions:      agent.instructions,
		Input:             string(input),
		Schema:            agent.outputSchema,
		RequiresWebSearch: agent.requiresWebSearch,
		MaxOutputTokens:   maxOutputTokens(agent.slug),
	})
	if err != nil {
		return model.Result{}, fmt.Errorf("invoke %s agent: %w", agent.slug, err)
	}
	if agent.slug == investmentAgentSlug {
		return agent.investmentResult(response.Output, invocation.DependencyResults)
	}

	return structuredResult(agent.slug, response)
}

func (agent OpenAIAgent) investmentResult(output string, dependencyResults map[string]any) (model.Result, error) {
	sources := sourcesFromDependencies(dependencyResults)
	if len(sources) < minimumSourceCount {
		return model.Result{}, fmt.Errorf("investment agent requires at least %d verified sources", minimumSourceCount)
	}

	markdown := strings.TrimSpace(output)
	if !strings.HasPrefix(markdown, "#") {
		return model.Result{}, fmt.Errorf("investment agent did not return Markdown heading")
	}
	if strings.Contains(markdown, "## 출처") {
		return model.Result{}, fmt.Errorf("investment agent included unverified source section")
	}

	return model.Result{Output: markdown + "\n\n" + sourcesMarkdown(sources)}, nil
}

func structuredResult(slug string, response agentClient.ResponseResult) (model.Result, error) {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(response.Output), &decoded); err != nil {
		return model.Result{}, fmt.Errorf("decode %s agent output: %w", slug, err)
	}
	if decoded == nil {
		return model.Result{}, fmt.Errorf("decode %s agent output: expected JSON object", slug)
	}
	if len(response.Sources) == 0 {
		return model.Result{}, fmt.Errorf("%s agent returned no verified web sources", slug)
	}

	decoded["sources"] = response.Sources
	return model.Result{Output: decoded}, nil
}

func maxOutputTokens(slug string) int64 {
	if slug == investmentAgentSlug {
		return investmentTokenLimit
	}

	return analysisTokenLimit
}

func sourcesFromDependencies(dependencyResults map[string]any) []agentClient.Source {
	sources := make([]agentClient.Source, 0)
	seenURLs := map[string]struct{}{}
	for _, dependencyResult := range dependencyResults {
		dependency, ok := dependencyResult.(map[string]any)
		if !ok {
			continue
		}
		output, ok := dependency["output"].(map[string]any)
		if !ok {
			continue
		}
		for _, source := range decodeSources(output["sources"]) {
			source, valid := verifiedSource(source)
			if !valid {
				continue
			}
			if _, exists := seenURLs[source.URL]; exists {
				continue
			}
			seenURLs[source.URL] = struct{}{}
			sources = append(sources, source)
			if len(sources) == maximumSourceCount {
				return sources
			}
		}
	}

	return sources
}

func decodeSources(value any) []agentClient.Source {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}

	var sources []agentClient.Source
	if err := json.Unmarshal(encoded, &sources); err != nil {
		return nil
	}

	return sources
}

func sourcesMarkdown(sources []agentClient.Source) string {
	lines := []string{"## 출처"}
	for _, source := range sources {
		lines = append(lines, fmt.Sprintf("- [%s](<%s>)", source.Title, source.URL))
	}

	return strings.Join(lines, "\n")
}

func verifiedSource(source agentClient.Source) (agentClient.Source, bool) {
	parsedURL, err := url.Parse(source.URL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Hostname() == "" || parsedURL.User != nil {
		return agentClient.Source{}, false
	}
	if hasControlCharacter(source.URL) || strings.ContainsAny(source.URL, "<>") {
		return agentClient.Source{}, false
	}

	parsedURL.Fragment = ""
	canonicalURL := parsedURL.String()
	if hasControlCharacter(canonicalURL) || strings.ContainsAny(canonicalURL, "<>") {
		return agentClient.Source{}, false
	}

	title := strings.NewReplacer("[", "", "]", "", "\n", " ", "\r", " ").Replace(strings.TrimSpace(source.Title))
	if title == "" || hasControlCharacter(title) {
		return agentClient.Source{}, false
	}

	return agentClient.Source{Title: title, URL: canonicalURL}, true
}

func hasControlCharacter(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

func instructionsFor(slug string) string {
	if slug == investmentAgentSlug {
		return "당신은 일반 사용자를 위한 투자 분석 에이전트입니다. 제공된 사용자 질문과 입력, 그리고 news·financial·risk 의존성 결과를 종합하세요. 한국어 Markdown만 반환하세요. 첫 줄은 # 제목으로 시작하고, ## 요약, ## 핵심 근거, ## 유의사항을 포함하세요. 투자 매수·매도를 단정하지 말고 데이터 한계와 변동성 위험을 명시하세요. 출처 링크나 ## 출처 섹션은 작성하지 마세요. 서비스가 검증된 출처를 별도로 붙입니다."
	}

	domainName := map[string]string{
		"financial": "재무 분석",
		"news":      "뉴스 분석",
		"risk":      "위험 분석",
	}[slug]
	return fmt.Sprintf("당신은 기술 시연용 리소스 서버의 %s 에이전트입니다. 제공된 사용자 질문과 입력을 바탕으로 반드시 웹 검색을 수행해 최신 근거를 확인하세요. 지정된 JSON Schema에 정확히 맞는 JSON 객체 하나만 반환하세요. Markdown, 설명, 주석 또는 추가 필드는 절대 포함하지 마세요. 근거가 없는 수치를 만들지 말고, 검색 결과로 확인 가능한 사실만 간결하게 사용하세요.", domainName)
}

func schemaFor(slug string) map[string]any {
	properties := map[string]any{}

	switch slug {
	case "financial":
		properties = map[string]any{
			"revenueGrowth": map[string]any{
				"type":        "number",
				"description": "검색 근거로 산출하거나 확인한 매출 성장률입니다.",
			},
			"debtRatio": map[string]any{
				"type":        "number",
				"description": "검색 근거로 확인한 부채 비율입니다.",
			},
		}
	case "news":
		properties = map[string]any{
			"sentiment": map[string]any{
				"type":        "string",
				"description": "최근 뉴스의 긍정·중립·부정 흐름입니다.",
			},
			"articles": map[string]any{
				"type":        "integer",
				"description": "분석에 사용한 기사 수입니다.",
			},
		}
	case "risk":
		properties = map[string]any{
			"riskLevel": map[string]any{
				"type":        "string",
				"description": "검색 근거를 반영한 위험 수준입니다.",
			},
			"volatility": map[string]any{
				"type":        "number",
				"description": "변동성 수준을 나타내는 수치입니다.",
			},
		}
	default:
		return nil
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
		"required":             requiredProperties(properties),
	}
}

func requiredProperties(properties map[string]any) []string {
	result := make([]string, 0, len(properties))
	for property := range properties {
		result = append(result, property)
	}

	return result
}
