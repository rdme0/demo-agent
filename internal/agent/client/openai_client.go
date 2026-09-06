package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

const (
	responseFormatName        = "demo_agent_result"
	maxWebSearchCalls         = 3
	webSearchSourcesInclude   = "web_search_call.action.sources"
	maximumSourcesPerResponse = 5
)

type ResponseRequest struct {
	Instructions      string
	Input             string
	Schema            map[string]any
	RequiresWebSearch bool
}

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type ResponseResult struct {
	Output  string
	Sources []Source
}

type ResponseClient interface {
	Generate(context.Context, ResponseRequest) (ResponseResult, error)
}

type OpenAIClient struct {
	client *openai.Client
	model  shared.ResponsesModel
}

func NewOpenAIClient(apiKey string, model string, options ...option.RequestOption) *OpenAIClient {
	requestOptions := append([]option.RequestOption{option.WithAPIKey(apiKey)}, options...)
	openAIClient := openai.NewClient(requestOptions...)

	return &OpenAIClient{
		client: &openAIClient,
		model:  shared.ResponsesModel(model),
	}
}

func (client *OpenAIClient) Generate(ctx context.Context, request ResponseRequest) (ResponseResult, error) {
	parameters := responses.ResponseNewParams{
		Instructions: openai.String(request.Instructions),
		Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(request.Input)},
		Model:        client.model,
		Reasoning: shared.ReasoningParam{
			Effort: shared.ReasoningEffortLow,
		},
	}
	if request.Schema != nil {
		parameters.Text = responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
					Name:   responseFormatName,
					Schema: normalizeSchemaForOpenAI(request.Schema),
					Strict: openai.Bool(true),
					Type:   constant.JSONSchema("json_schema"),
				},
			},
		}
	}
	if request.RequiresWebSearch {
		parameters.Tools = []responses.ToolUnionParam{
			{
				OfWebSearchPreview: &responses.WebSearchToolParam{
					Type: responses.WebSearchToolTypeWebSearchPreview,
				},
			},
		}
		parameters.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsRequired),
		}
		parameters.MaxToolCalls = openai.Int(maxWebSearchCalls)
		parameters.Include = []responses.ResponseIncludable{
			responses.ResponseIncludable(webSearchSourcesInclude),
		}
	}
	response, err := client.client.Responses.New(ctx, parameters)
	if err != nil {
		return ResponseResult{}, fmt.Errorf("create OpenAI response: %w", err)
	}
	if response == nil {
		return ResponseResult{}, fmt.Errorf("create OpenAI response: empty response")
	}
	if response.Status != responses.ResponseStatusCompleted {
		reason := strings.TrimSpace(response.IncompleteDetails.Reason)
		if reason == "" {
			reason = "unknown reason"
		}
		return ResponseResult{}, fmt.Errorf("OpenAI response did not complete: %s (%s)", response.Status, reason)
	}

	output := strings.TrimSpace(response.OutputText())
	if output == "" {
		return ResponseResult{}, fmt.Errorf("OpenAI response did not contain output text")
	}
	if request.RequiresWebSearch && !completedWebSearch(response.Output) {
		return ResponseResult{}, fmt.Errorf("OpenAI response did not complete web search")
	}

	return ResponseResult{
		Output:  output,
		Sources: sourcesFrom(response.Output),
	}, nil
}

func normalizeSchemaForOpenAI(schema map[string]any) map[string]any {
	normalized, ok := normalizeSchemaValue(schema).(map[string]any)
	if !ok {
		return map[string]any{}
	}

	return normalized
}

func normalizeSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, child := range typed {
			if key == "format" {
				continue
			}
			normalized[key] = normalizeSchemaValue(child)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for index, child := range typed {
			normalized[index] = normalizeSchemaValue(child)
		}
		return normalized
	default:
		return value
	}
}

func completedWebSearch(output []responses.ResponseOutputItemUnion) bool {
	for _, item := range output {
		if item.Type == "web_search_call" && item.Status == "completed" {
			return true
		}
	}

	return false
}

func sourcesFrom(output []responses.ResponseOutputItemUnion) []Source {
	sources := make([]Source, 0)
	seenURLs := map[string]struct{}{}
	for _, item := range output {
		for _, source := range sourcesFromWebSearchCall(item) {
			appendSource(&sources, seenURLs, source)
			if len(sources) == maximumSourcesPerResponse {
				return sources
			}
		}
		for _, content := range item.Content {
			for _, annotation := range content.Annotations {
				if annotation.Type != "url_citation" {
					continue
				}
				source, ok := sourceFromAnnotation(annotation.Title, annotation.URL)
				if !ok {
					continue
				}
				appendSource(&sources, seenURLs, source)
				if len(sources) == maximumSourcesPerResponse {
					return sources
				}
			}
		}
	}

	return sources
}

func sourcesFromWebSearchCall(item responses.ResponseOutputItemUnion) []Source {
	if item.Type != "web_search_call" {
		return nil
	}

	var call struct {
		Action struct {
			Sources []struct {
				URL string `json:"url"`
			} `json:"sources"`
		} `json:"action"`
	}
	if err := json.Unmarshal([]byte(item.RawJSON()), &call); err != nil {
		return nil
	}

	sources := make([]Source, 0, len(call.Action.Sources))
	for _, source := range call.Action.Sources {
		verified, ok := sourceFromAnnotation("", source.URL)
		if ok {
			sources = append(sources, verified)
		}
	}

	return sources
}

func appendSource(sources *[]Source, seenURLs map[string]struct{}, source Source) {
	if _, exists := seenURLs[source.URL]; exists {
		return
	}
	seenURLs[source.URL] = struct{}{}
	*sources = append(*sources, source)
}

func sourceFromAnnotation(title string, rawURL string) (Source, bool) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return Source{}, false
	}
	parsedURL.Fragment = ""
	title = strings.NewReplacer("[", "", "]", "", "<", "", ">", "", "\n", " ", "\r", " ").Replace(strings.TrimSpace(title))
	if strings.TrimSpace(title) == "" {
		title = parsedURL.Host
	}
	if strings.IndexFunc(title, unicode.IsControl) >= 0 {
		return Source{}, false
	}

	return Source{Title: strings.TrimSpace(title), URL: parsedURL.String()}, true
}
