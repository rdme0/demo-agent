package dto

type Dependency struct {
	AgentVersionID string   `json:"agentVersionId"`
	CallPath       []string `json:"callPath"`
	Input          any      `json:"input,omitempty"`
}

type Request struct {
	ParentStepID string       `json:"parentStepId"`
	CallbackURL  string       `json:"callbackUrl"`
	Dependencies []Dependency `json:"dependencies"`
}
