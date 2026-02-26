package engine

import (
	"cmp"
	"context"
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
		return nil, ErrNoBranches
	}

	slices.SortFunc(branches, func(a, b approval.ConditionBranch) int {
		return cmp.Compare(a.Priority, b.Priority)
	})

	formData := approval.NewFormData(pc.Instance.FormData)

	var deptID string
	if pc.Instance.ApplicantDeptID != nil {
		deptID = *pc.Instance.ApplicantDeptID
	}

	evalCtx := &approval.EvaluationContext{
		FormData:    formData,
		ApplicantID: pc.Instance.ApplicantID,
		ApplicantDeptID: deptID,
	}

	var defaultBranch *approval.ConditionBranch

	for i := range branches {
		b := &branches[i]
		if b.IsDefault {
			defaultBranch = b
			continue
		}

		match, err := evaluateConditionGroups(pc.Registry, ctx, evalCtx, b.ConditionGroups)
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

	return nil, ErrNoMatchingBranch
}

// evaluateConditionGroups evaluates condition groups using OR between groups, AND within each group.
func evaluateConditionGroups(registry *strategy.StrategyRegistry, ctx context.Context, evalCtx *approval.EvaluationContext, groups []approval.ConditionGroup) (bool, error) {
	if len(groups) == 0 {
		return true, nil
	}
	for _, group := range groups {
		match, err := evaluateGroupConditions(registry, ctx, evalCtx, group.Conditions)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

// evaluateGroupConditions evaluates a set of conditions using AND logic.
func evaluateGroupConditions(registry *strategy.StrategyRegistry, ctx context.Context, evalCtx *approval.EvaluationContext, conditions []approval.Condition) (bool, error) {
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
