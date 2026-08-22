package agent

import "context"

type FixtureAgent struct {
	slug   string
	output any
}

func NewFixtureAgents() []Agent {
	return []Agent{
		FixtureAgent{slug: "investment", output: map[string]any{"recommendation": "balanced", "score": 0.82}},
		FixtureAgent{slug: "financial", output: map[string]any{"revenueGrowth": 0.14, "debtRatio": 0.31}},
		FixtureAgent{slug: "news", output: map[string]any{"sentiment": "positive", "articles": 12}},
		FixtureAgent{slug: "risk", output: map[string]any{"riskLevel": "medium", "volatility": 0.18}},
	}
}

func (agent FixtureAgent) Slug() string {
	return agent.slug
}

func (agent FixtureAgent) Invoke(_ context.Context, _ Invocation) (Result, error) {
	return Result{Output: agent.output}, nil
}
