package dto

type InvocationResponse struct {
	Agent             string         `json:"agent"`
	Output            any            `json:"output"`
	DependencyResults map[string]any `json:"dependencyResults"`
}
