package engine

import (
	"context"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/orm"
)

// NodeAction represents the result action of node processing.
type NodeAction int

const (
	NodeActionWait     NodeAction = iota // Wait for user action
	NodeActionContinue                   // Auto-advance to next node
	NodeActionComplete                   // Flow ends
)

// ProcessResult contains the outcome of node processing.
type ProcessResult struct {
	Action      NodeAction
	FinalStatus approval.InstanceStatus // Only used when Action == NodeActionComplete
	BranchID    string               // Condition node: the matched branch ID
}

// ProcessContext provides context for node processing.
type ProcessContext struct {
	DB          orm.DB
	Instance    *approval.Instance
	Node        *approval.FlowNode
	Assignees   []*approval.FlowNodeAssignee
	FormData    approval.FormData
	ApplicantID string
	Registry    *strategy.StrategyRegistry
}

// NodeProcessor processes a specific node kind.
type NodeProcessor interface {
	NodeKind() approval.NodeKind
	Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error)
}

// NodePredictor predicts assignees for a node without side effects.
type NodePredictor interface {
	Predict(ctx context.Context, pc *ProcessContext) ([]string, error)
}
