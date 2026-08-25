package catalog

import "sort"

const (
	ContractVersion = "1.0.0"
	AgentVersion    = "1.0.0"
	Network         = "eip155:84532"
	Asset           = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
)

type Dependency struct {
	FunctionCode   string
	MaxPriceAtomic string
}

type Definition struct {
	Code                string
	Name                string
	Description         string
	DeveloperID         string
	DeveloperName       string
	FunctionCode        string
	FunctionName        string
	FunctionDescription string
	ResponseFormat      string
	UsageType           string
	PriceAtomic         string
	PayTo               string
	Prompt              string
	Fixture             any
	InputSchema         map[string]any
	OutputSchema        map[string]any
	RequiresWebSearch   bool
	AggregateMarkdown   bool
	MinimumSources      int
	MaxOutputTokens     int64
	Dependencies        []Dependency
}

func Definitions() []Definition {
	input := inputSchema()
	financialOutput := objectSchema(map[string]any{
		"summary":    stringSchema(),
		"keyMetrics": stringArraySchema(),
		"risks":      stringArraySchema(),
		"sources":    sourcesSchema(),
	})
	newsOutput := objectSchema(map[string]any{
		"summary":    stringSchema(),
		"sentiment":  enumSchema("positive", "neutral", "negative", "mixed"),
		"highlights": stringArraySchema(),
		"sources":    sourcesSchema(),
	})
	riskOutput := objectSchema(map[string]any{
		"summary":   stringSchema(),
		"riskLevel": enumSchema("low", "medium", "high"),
		"factors":   stringArraySchema(),
		"sources":   sourcesSchema(),
	})
	productOutput := objectSchema(map[string]any{
		"summary":  stringSchema(),
		"products": map[string]any{"type": "array", "items": objectSchema(map[string]any{"name": stringSchema(), "keyFeatures": stringArraySchema()})},
		"sources":  sourcesSchema(),
	})
	reviewOutput := objectSchema(map[string]any{
		"summary":    stringSchema(),
		"strengths":  stringArraySchema(),
		"weaknesses": stringArraySchema(),
		"sources":    sourcesSchema(),
	})
	priceOutput := objectSchema(map[string]any{
		"summary": stringSchema(),
		"offers":  map[string]any{"type": "array", "items": objectSchema(map[string]any{"seller": stringSchema(), "priceText": stringSchema(), "condition": stringSchema()})},
		"sources": sourcesSchema(),
	})
	destinationOutput := objectSchema(map[string]any{
		"summary":    stringSchema(),
		"highlights": stringArraySchema(),
		"cautions":   stringArraySchema(),
		"sources":    sourcesSchema(),
	})
	weatherOutput := objectSchema(map[string]any{
		"summary": stringSchema(),
		"periods": map[string]any{"type": "array", "items": objectSchema(map[string]any{"period": stringSchema(), "conditions": stringSchema()})},
		"sources": sourcesSchema(),
	})
	safetyOutput := objectSchema(map[string]any{
		"summary":     stringSchema(),
		"riskLevel":   enumSchema("low", "medium", "high"),
		"precautions": stringArraySchema(),
		"sources":     sourcesSchema(),
	})

	return []Definition{
		root("investment-analysis", "투자 분석", "재무·시장 뉴스·위험 정보를 종합해 투자 판단 근거를 정리합니다.", "investment-analysis", "투자 분석", "여러 분석 결과를 종합하는 투자 분석 기능입니다.", "투자 분석 결과는 요약, 재무·시장 근거, 위험, 유의사항 순서의 한국어 Markdown으로 작성하세요. 매수·매도를 단정하지 마세요.", "# 투자 분석\n\n## 요약\n투자 판단 전 여러 근거를 함께 확인하세요.\n\n## 재무·시장 근거\n재무와 시장 흐름을 비교했습니다.\n\n## 위험\n변동성과 정보 한계를 고려하세요.\n\n## 유의사항\n이 결과는 데모입니다.\n\n## 출처\n- [재무 데모](https://example.com/investment/financial)\n- [시장 데모](https://example.com/investment/market)\n- [위험 데모](https://example.com/investment/risk)", "1000", "0101", []Dependency{{FunctionCode: "financial-analysis", MaxPriceAtomic: "1000"}, {FunctionCode: "market-news-analysis", MaxPriceAtomic: "1200"}, {FunctionCode: "investment-risk-analysis", MaxPriceAtomic: "900"}}, input),
		specialist("financial-analysis", "재무 분석", "기업의 재무 신호와 위험을 조사합니다.", "financial-analysis", "재무 분석", "재무 지표와 위험을 구조화해 제공하는 기능입니다.", financialOutput, "재무 분석 에이전트입니다. 질문과 입력에 맞는 최신 재무 근거를 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 결과만 반환하세요.", map[string]any{"summary": "재무 건전성은 보통 수준입니다.", "keyMetrics": []string{"매출 흐름 확인", "부채 비율 점검"}, "risks": []string{"실적 변동 가능성"}, "sources": sources("financial-analysis")}, "1000", "0102"),
		specialist("market-news-fast", "빠른 시장 뉴스", "짧은 최신 시장 뉴스 분석을 제공합니다.", "market-news-analysis", "시장 뉴스 분석", "시장 뉴스 흐름과 핵심 근거를 구조화해 제공하는 기능입니다.", newsOutput, "시장 뉴스 분석 에이전트입니다. 최신 뉴스를 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 간결한 결과만 반환하세요.", map[string]any{"summary": "최근 뉴스 흐름은 중립적입니다.", "sentiment": "neutral", "highlights": []string{"시장 변동성 확대", "실적 발표 대기"}, "sources": sources("market-news-fast")}, "600", "0103"),
		specialist("market-news-deep", "심층 시장 뉴스", "상세한 최신 시장 뉴스 분석을 제공합니다.", "market-news-analysis", "시장 뉴스 분석", "시장 뉴스 흐름과 핵심 근거를 구조화해 제공하는 기능입니다.", newsOutput, "심층 시장 뉴스 분석 에이전트입니다. 최신 뉴스를 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 상세한 결과만 반환하세요.", map[string]any{"summary": "최근 뉴스 흐름은 중립적이며 변동성이 있습니다.", "sentiment": "neutral", "highlights": []string{"시장 변동성 확대", "실적 발표 대기", "정책 변수 확인"}, "sources": sources("market-news-deep")}, "1200", "0104"),
		specialist("investment-risk", "투자 위험 분석", "투자 판단의 주요 위험 요소를 조사합니다.", "investment-risk-analysis", "투자 위험 분석", "투자 위험 수준과 요인을 구조화해 제공하는 기능입니다.", riskOutput, "투자 위험 분석 에이전트입니다. 질문과 입력에 맞는 위험 요인을 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 결과만 반환하세요.", map[string]any{"summary": "변동성 위험을 주의해야 합니다.", "riskLevel": "medium", "factors": []string{"시장 변동성", "정보 지연"}, "sources": sources("investment-risk")}, "900", "0105"),
		root("shopping-assistant", "상품 구매 도우미", "제품·리뷰·가격 정보를 비교해 구매 판단을 돕습니다.", "shopping-advice", "상품 구매 조언", "상품 후보와 가격·리뷰 근거를 종합하는 기능입니다.", "상품 구매 도우미입니다. 의존성 결과를 바탕으로 추천 요약, 후보 비교, 가격·리뷰 근거, 구매 유의사항 순서의 한국어 Markdown을 작성하세요.", "# 상품 구매 비교\n\n## 추천 요약\n용도와 예산에 맞는 후보를 비교하세요.\n\n## 후보 비교\n기능과 후기를 함께 확인했습니다.\n\n## 가격·리뷰 근거\n판매 조건과 리뷰를 비교했습니다.\n\n## 구매 유의사항\n최종 가격과 보증 조건을 확인하세요.\n\n## 출처\n- [제품 데모](https://example.com/shopping/products)\n- [리뷰 데모](https://example.com/shopping/reviews)\n- [가격 데모](https://example.com/shopping/prices)", "1000", "0106", []Dependency{{FunctionCode: "product-search", MaxPriceAtomic: "800"}, {FunctionCode: "review-analysis", MaxPriceAtomic: "900"}, {FunctionCode: "price-comparison", MaxPriceAtomic: "700"}}, input),
		specialist("product-search", "상품 탐색", "요청에 맞는 제품 후보와 특징을 찾습니다.", "product-search", "상품 탐색", "제품 후보와 핵심 특징을 구조화해 제공하는 기능입니다.", productOutput, "상품 탐색 에이전트입니다. 질문과 입력에 맞는 제품 정보를 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 결과만 반환하세요.", map[string]any{"summary": "세 가지 제품 후보를 찾았습니다.", "products": []map[string]any{{"name": "Demo One", "keyFeatures": []string{"휴대성", "긴 배터리"}}, {"name": "Demo Two", "keyFeatures": []string{"성능", "보증"}}}, "sources": sources("product-search")}, "800", "0107"),
		specialist("review-analysis", "리뷰 분석", "사용자 후기에서 장점과 단점을 정리합니다.", "review-analysis", "리뷰 분석", "제품 리뷰의 장점과 단점을 구조화해 제공하는 기능입니다.", reviewOutput, "리뷰 분석 에이전트입니다. 최신 사용자 리뷰를 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 결과만 반환하세요.", map[string]any{"summary": "후기는 편의성과 가격을 함께 언급합니다.", "strengths": []string{"사용 편의성", "디자인"}, "weaknesses": []string{"부가 기능 제한"}, "sources": sources("review-analysis")}, "900", "0108"),
		specialist("price-comparison", "가격 비교", "판매처별 가격과 구매 조건을 비교합니다.", "price-comparison", "가격 비교", "판매처별 가격과 조건을 구조화해 제공하는 기능입니다.", priceOutput, "가격 비교 에이전트입니다. 판매처와 가격 조건을 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 결과만 반환하세요.", map[string]any{"summary": "판매처별 조건이 다릅니다.", "offers": []map[string]any{{"seller": "Demo Store A", "priceText": "₩100,000", "condition": "무료 배송"}, {"seller": "Demo Store B", "priceText": "₩98,000", "condition": "멤버십 필요"}}, "sources": sources("price-comparison")}, "700", "0109"),
		root("travel-planner", "여행 계획", "장소·날씨·안전 정보를 종합해 여행 계획을 만듭니다.", "travel-plan", "여행 계획", "여행 장소, 날씨, 안전 정보를 종합하는 기능입니다.", "여행 계획 에이전트입니다. 의존성 결과를 바탕으로 일정 요약, 추천 장소, 날씨·안전, 유의사항 순서의 한국어 Markdown을 작성하세요.", "# 여행 계획\n\n## 일정 요약\n여행 목적과 기간을 먼저 정하세요.\n\n## 추천 장소\n관심사에 맞는 장소를 비교했습니다.\n\n## 날씨·안전\n날씨와 안전 정보를 확인하세요.\n\n## 유의사항\n출발 전 최신 공지를 확인하세요.\n\n## 출처\n- [여행지 데모](https://example.com/travel/destination)\n- [날씨 데모](https://example.com/travel/weather)\n- [안전 데모](https://example.com/travel/safety)", "1000", "0110", []Dependency{{FunctionCode: "destination-research", MaxPriceAtomic: "800"}, {FunctionCode: "weather-forecast", MaxPriceAtomic: "600"}, {FunctionCode: "travel-safety-analysis", MaxPriceAtomic: "900"}}, input),
		specialist("destination-research", "여행지 조사", "여행지의 볼거리와 주의점을 조사합니다.", "destination-research", "여행지 조사", "여행지의 추천 지점과 주의점을 구조화해 제공하는 기능입니다.", destinationOutput, "여행지 조사 에이전트입니다. 최신 여행지 정보를 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 결과만 반환하세요.", map[string]any{"summary": "도보와 대중교통으로 둘러보기 좋습니다.", "highlights": []string{"대표 명소", "지역 음식"}, "cautions": []string{"혼잡 시간 확인"}, "sources": sources("destination-research")}, "800", "0111"),
		specialist("weather-forecast", "날씨 예보", "여행 기간의 날씨 조건을 조사합니다.", "weather-forecast", "날씨 예보", "여행 기간별 날씨 조건을 구조화해 제공하는 기능입니다.", weatherOutput, "날씨 예보 에이전트입니다. 최신 예보를 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 결과만 반환하세요.", map[string]any{"summary": "가벼운 우산을 준비하세요.", "periods": []map[string]any{{"period": "첫째 날", "conditions": "맑음"}, {"period": "둘째 날", "conditions": "약한 비 가능"}}, "sources": sources("weather-forecast")}, "600", "0112"),
		specialist("travel-safety", "여행 안전", "여행지의 안전 정보와 주의 사항을 조사합니다.", "travel-safety-analysis", "여행 안전 분석", "여행 안전 수준과 행동 요령을 구조화해 제공하는 기능입니다.", safetyOutput, "여행 안전 에이전트입니다. 최신 안전 정보를 웹 검색으로 확인하고, 주어진 JSON Schema에 맞는 결과만 반환하세요.", map[string]any{"summary": "일반적인 여행 안전 수칙을 지키세요.", "riskLevel": "low", "precautions": []string{"야간 이동 전 교통편 확인", "현지 긴급 연락처 저장"}, "sources": sources("travel-safety")}, "900", "0113"),
	}
}

func root(code, name, description, functionCode, functionName, functionDescription, prompt, fixture, price, payToSuffix string, dependencies []Dependency, input map[string]any) Definition {
	return Definition{Code: code, Name: name, Description: description, DeveloperID: developerID(), DeveloperName: "AgentStore Demo", FunctionCode: functionCode, FunctionName: functionName, FunctionDescription: functionDescription, ResponseFormat: "MARKDOWN", UsageType: "user_facing", PriceAtomic: price, PayTo: payTo(payToSuffix), Prompt: prompt, Fixture: fixture, InputSchema: input, OutputSchema: map[string]any{"type": "string"}, AggregateMarkdown: true, MinimumSources: 3, MaxOutputTokens: 2048, Dependencies: dependencies}
}

func specialist(code, name, description, functionCode, functionName, functionDescription string, outputSchema map[string]any, prompt string, fixture any, price, payToSuffix string) Definition {
	return Definition{Code: code, Name: name, Description: description, DeveloperID: developerID(), DeveloperName: "AgentStore Demo", FunctionCode: functionCode, FunctionName: functionName, FunctionDescription: functionDescription, ResponseFormat: "JSON", UsageType: "internal_component", PriceAtomic: price, PayTo: payTo(payToSuffix), Prompt: prompt, Fixture: fixture, InputSchema: inputSchema(), OutputSchema: outputSchema, RequiresWebSearch: true, MaxOutputTokens: 1536}
}

func developerID() string {
	return "00000000-0000-0000-0000-00000000d001"
}

func payTo(suffix string) string {
	return "0x000000000000000000000000000000000000" + suffix
}

func inputSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": true}
}

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func stringArraySchema() map[string]any {
	return map[string]any{"type": "array", "items": stringSchema()}
}

func sourcesSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": objectSchema(map[string]any{
			"title": stringSchema(),
			"url":   map[string]any{"type": "string", "format": "uri"},
		}),
	}
}

func objectSchema(properties map[string]any) map[string]any {
	required := make([]string, 0, len(properties))
	for key := range properties {
		required = append(required, key)
	}
	sort.Strings(required)

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
		"required":             required,
	}
}

func sources(code string) []map[string]string {
	return []map[string]string{
		{"title": code + " demo source 1", "url": "https://example.com/" + code + "/1"},
		{"title": code + " demo source 2", "url": "https://example.com/" + code + "/2"},
		{"title": code + " demo source 3", "url": "https://example.com/" + code + "/3"},
	}
}
