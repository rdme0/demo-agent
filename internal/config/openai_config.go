package config

const (
	AgentModeFixture = "fixture"
	AgentModeOpenAI  = "openai"
	OpenAIModel      = "gpt-5.6-luna"
)

type OpenAIConfig struct {
	APIKey string
	Model  string
}
