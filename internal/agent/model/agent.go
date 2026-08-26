package model

import "context"

type Invocation struct {
	Input               any
	DependencyResults   map[string]any
	ResolveDependencies func(context.Context) (map[string]any, error)
}

type Result struct {
	Output            any
	DependencyResults map[string]any
}

type Agent interface {
	Code() string
	Invoke(context.Context, Invocation) (Result, error)
}
