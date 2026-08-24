package dto

import runtimeDTO "demo-agent/internal/runtime/dto"

type InvocationRequest struct {
	Input   any                 `json:"input,omitempty"`
	Runtime *runtimeDTO.Request `json:"runtime,omitempty"`
}
