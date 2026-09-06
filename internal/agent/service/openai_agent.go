package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"demo-agent/catalog"
	agentClient "demo-agent/internal/agent/client"
	"demo-agent/internal/agent/model"
)

const (
	maximumSourceCount   = 5
	webSearchInstruction = "웹 검색을 반드시 수행하고, 검색 결과에서 확인한 출처를 최소 3개 포함하세요. 응답의 sources에는 검색으로 확인한 HTTPS URL만 사용하세요."
	markdownInstruction  = "응답은 한국어 Markdown으로 작성하고, 첫 줄은 반드시 # 제목으로 시작하세요. 검증된 출처 목록은 플랫폼이 추가하므로 별도의 출처 섹션을 만들지 마세요."
)

type OpenAIAgent struct {
	definition     catalog.Definition
	responseClient agentClient.ResponseClient
}

func NewOpenAIAgents(responseClient agentClient.ResponseClient) ([]model.Agent, error) {
	if responseClient == nil {
		return nil, fmt.Errorf("OpenAI response client is required")
	}

	definitions := catalog.Definitions()
	agents := make([]model.Agent, 0, len(definitions))
	for _, definition := range definitions {
		agents = append(agents, newOpenAIAgent(definition, responseClient))
	}
	return agents, nil
}

func newOpenAIAgent(definition catalog.Definition, responseClient agentClient.ResponseClient) OpenAIAgent {
	return OpenAIAgent{
		definition:     definition,
		responseClient: responseClient,
	}
}

func (agent OpenAIAgent) Code() string {
	return agent.definition.Code
}

func (agent OpenAIAgent) Invoke(ctx context.Context, invocation model.Invocation) (model.Result, error) {
	dependencyResults, err := agent.resolveDependencies(ctx, invocation)
	if err != nil {
		return model.Result{}, err
	}
	input, err := json.Marshal(map[string]any{
		"input":             invocation.Input,
		"dependencyResults": dependencyResults,
	})
	if err != nil {
		return model.Result{}, fmt.Errorf("encode %s agent input: %w", agent.Code(), err)
	}

	schema := agent.definition.OutputSchema
	if agent.definition.AggregateMarkdown {
		schema = nil
	}
	instructions := agent.definition.Prompt
	if agent.definition.RequiresWebSearch {
		instructions = strings.TrimSpace(instructions) + "\n" + webSearchInstruction
	}
	if agent.definition.AggregateMarkdown {
		instructions = strings.TrimSpace(instructions) + "\n" + markdownInstruction
	}
	response, err := agent.responseClient.Generate(ctx, agentClient.ResponseRequest{
		Instructions:      instructions,
		Input:             string(input),
		Schema:            schema,
		RequiresWebSearch: agent.definition.RequiresWebSearch,
	})
	if err != nil {
		return model.Result{}, fmt.Errorf("invoke %s agent: %w", agent.Code(), err)
	}
	if agent.definition.AggregateMarkdown {
		result, err := agent.aggregateMarkdownResult(response.Output, dependencyResults)
		if err != nil {
			return model.Result{}, err
		}
		result.DependencyResults = dependencyResults
		return result, nil
	}

	result, err := structuredResult(agent.Code(), agent.definition.MinimumSources, response)
	if err != nil {
		return model.Result{}, err
	}
	result.DependencyResults = dependencyResults
	return result, nil
}

func (agent OpenAIAgent) resolveDependencies(ctx context.Context, invocation model.Invocation) (map[string]any, error) {
	if !agent.definition.AggregateMarkdown || invocation.ResolveDependencies == nil {
		return invocation.DependencyResults, nil
	}

	return invocation.ResolveDependencies(ctx)
}

func (agent OpenAIAgent) aggregateMarkdownResult(output string, dependencyResults map[string]any) (model.Result, error) {
	sources := sourcesFromDependencies(dependencyResults)
	if len(sources) < agent.definition.MinimumSources {
		return model.Result{}, fmt.Errorf("%s agent requires at least %d verified sources", agent.Code(), agent.definition.MinimumSources)
	}

	markdown := strings.TrimSpace(output)
	if !strings.HasPrefix(markdown, "#") {
		return model.Result{}, fmt.Errorf("%s agent did not return Markdown heading", agent.Code())
	}
	if strings.Contains(markdown, "## 출처") {
		return model.Result{}, fmt.Errorf("%s agent included unverified source section", agent.Code())
	}

	return model.Result{Output: markdown + "\n\n" + sourcesMarkdown(sources)}, nil
}

func structuredResult(code string, minimumSources int, response agentClient.ResponseResult) (model.Result, error) {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(response.Output), &decoded); err != nil {
		return model.Result{}, fmt.Errorf("decode %s agent output: %w", code, err)
	}
	if decoded == nil {
		return model.Result{}, fmt.Errorf("decode %s agent output: expected JSON object", code)
	}
	if len(response.Sources) < minimumSources {
		return model.Result{}, fmt.Errorf("%s agent returned %d verified web sources; at least %d are required", code, len(response.Sources), minimumSources)
	}

	decoded["sources"] = response.Sources
	return model.Result{Output: decoded}, nil
}

func sourcesFromDependencies(dependencyResults map[string]any) []agentClient.Source {
	sources := make([]agentClient.Source, 0)
	seenURLs := map[string]struct{}{}
	for _, dependencyResult := range dependencyResults {
		output, ok := dependencyResult.(map[string]any)
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
