package compute

import (
	"context"
)

// ErrComputeNotSupported is returned when compute eval is not supported.
const errComputeNotSupported = "compute: eval not yet supported"

// Request is the compute evaluation request.
type Request struct {
	DashboardSlug string
	WidgetID      string
	ExprID        string
	StateSnapshot map[string]any
}

// Result is the compute evaluation result.
type Result struct {
	Value any
	Error string
}

// Service evaluates Starlark compute expressions.
type Service struct{}

// NewService creates a new compute service.
func NewService() *Service {
	return &Service{}
}

// Eval evaluates a compute expression. Currently a stub.
func (s *Service) Eval(_ context.Context, req Request) Result {
	return Result{Error: errComputeNotSupported + " expr=" + req.ExprID}
}
