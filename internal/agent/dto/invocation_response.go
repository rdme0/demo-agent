package dto

const DemoTransport = "agentstore-demo/v1"

type InvocationResponse struct {
	Transport         string         `json:"transport"`
	Agent             string         `json:"agent"`
	Output            any            `json:"output"`
	DependencyResults map[string]any `json:"dependencyResults"`
}
