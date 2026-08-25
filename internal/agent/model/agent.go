package model

import "context"

type Invocation struct {
	Input             any
	DependencyResults map[string]any
}

type Result struct {
	Output any
}

type Agent interface {
	Code() string
	Invoke(context.Context, Invocation) (Result, error)
}
