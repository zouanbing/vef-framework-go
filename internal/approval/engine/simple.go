package engine

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
)

// StartProcessor handles start nodes by auto-advancing to the next node.
type StartProcessor struct{}

// NewStartProcessor creates a StartProcessor.
func NewStartProcessor() NodeProcessor { return &StartProcessor{} }

func (p *StartProcessor) NodeKind() approval.NodeKind { return approval.NodeStart }

func (p *StartProcessor) Process(context.Context, *ProcessContext) (*ProcessResult, error) {
	return &ProcessResult{Action: NodeActionContinue}, nil
}

// EndProcessor handles end nodes by completing the flow as approved.
type EndProcessor struct{}

// NewEndProcessor creates an EndProcessor.
func NewEndProcessor() NodeProcessor { return &EndProcessor{} }

func (p *EndProcessor) NodeKind() approval.NodeKind { return approval.NodeEnd }

func (p *EndProcessor) Process(context.Context, *ProcessContext) (*ProcessResult, error) {
	return &ProcessResult{
		Action:      NodeActionComplete,
		FinalStatus: approval.InstanceApproved,
	}, nil
}

// ConditionProcessor evaluates condition branches and selects the matching branch.
type ConditionProcessor struct{}

// NewConditionProcessor creates a ConditionProcessor.
func NewConditionProcessor() NodeProcessor { return &ConditionProcessor{} }

func (p *ConditionProcessor) NodeKind() approval.NodeKind { return approval.NodeCondition }

func (p *ConditionProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	branches := pc.Node.Branches
	if len(branches) == 0 {
		return nil, errors.New("condition node has no branches")
	}

	slices.SortFunc(branches, func(a, b approval.ConditionBranch) int {
		return cmp.Compare(a.Priority, b.Priority)
	})

	formData := approval.NewFormData(pc.Instance.FormData)
	evalCtx := &approval.EvalContext{
		FormData:    formData,
		ApplicantID: pc.Instance.ApplicantID,
		DeptID:      pc.Instance.ApplicantDeptID.String,
	}

	var defaultBranch *approval.ConditionBranch

	for i := range branches {
		b := &branches[i]
		if b.IsDefault {
			defaultBranch = b
			continue
		}

		match, err := evaluateBranchConditions(pc.Registry, ctx, evalCtx, b.Conditions)
		if err != nil {
			return nil, fmt.Errorf("evaluate branch %q: %w", b.Label, err)
		}

		if match {
			return &ProcessResult{Action: NodeActionContinue, BranchID: b.ID}, nil
		}
	}

	if defaultBranch != nil {
		return &ProcessResult{Action: NodeActionContinue, BranchID: defaultBranch.ID}, nil
	}

	return nil, errors.New("no matching branch and no default branch")
}

// evaluateBranchConditions evaluates a set of conditions using AND logic.
func evaluateBranchConditions(registry *strategy.StrategyRegistry, ctx context.Context, evalCtx *approval.EvalContext, conditions []approval.Condition) (bool, error) {
	if len(conditions) == 0 {
		return true, nil
	}

	for _, cond := range conditions {
		evaluator, err := registry.GetConditionEvaluator(cond.Type)
		if err != nil {
			return false, err
		}

		match, err := evaluator.Evaluate(ctx, cond, evalCtx)
		if err != nil {
			return false, err
		}

		if !match {
			return false, nil
		}
	}

	return true, nil
}
