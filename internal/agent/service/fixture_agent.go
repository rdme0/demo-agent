package service

import (
	"context"

	"demo-agent/internal/agent/model"
)

type FixtureAgent struct {
	slug   string
	output any
}

func NewFixtureAgents() []model.Agent {
	return []model.Agent{
		FixtureAgent{slug: "investment", output: "# 투자 분석 요약\n\n지금은 재무·뉴스·위험 정보를 함께 확인한 뒤 판단하는 편이 좋아요.\n\n## 좋은 점\n\n- 재무와 시장 흐름을 한 번에 살펴볼 수 있어요.\n\n## 조심할 점\n\n- 이 결과는 연습용 예시이며, 실제 투자 전에는 최신 정보와 본인의 상황을 꼭 확인해 주세요."},
		FixtureAgent{slug: "financial", output: map[string]any{"revenueGrowth": 0.14, "debtRatio": 0.31}},
		FixtureAgent{slug: "news", output: map[string]any{"sentiment": "positive", "articles": 12}},
		FixtureAgent{slug: "risk", output: map[string]any{"riskLevel": "medium", "volatility": 0.18}},
	}
}

func (agent FixtureAgent) Slug() string {
	return agent.slug
}

func (agent FixtureAgent) Invoke(_ context.Context, _ model.Invocation) (model.Result, error) {
	return model.Result{Output: agent.output}, nil
}
