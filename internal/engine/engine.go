package engine

import "context"

// Engine 3M 核心能力（Phase-0 由 StubEngine 实现）。
type Engine interface {
	ListManaged(ctx context.Context, in ListInput) string
	AddManaged(ctx context.Context, in AddInput) string
	ExecuteStep(ctx context.Context, in ExecuteInput) string
}

// ListInput list_managed_mcps 入参。
type ListInput struct {
	CorrelationID string
}

// AddInput add_managed_mcp 入参。
type AddInput struct {
	Requirement   string
	Constraints   string
	CorrelationID string
}

// ExecuteInput execute_step 入参。
type ExecuteInput struct {
	MCPID         string
	StepGoal      string
	Rawdata       string
	CorrelationID string
}
