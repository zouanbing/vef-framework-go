package engine

import (
	"context"

	"github.com/ilxqx/vef-framework-go/approval"
)

// HandleProcessor handles handle nodes (claim mode).
// Unlike approval nodes, handle nodes create all tasks with sortOrder=0,
// allowing any candidate to claim and complete the task.
type HandleProcessor struct {
	orgService  approval.OrganizationService
	userService approval.UserService
}

// NewHandleProcessor creates a new handle processor.
func NewHandleProcessor(orgService approval.OrganizationService, userService approval.UserService) *HandleProcessor {
	return &HandleProcessor{
		orgService:  orgService,
		userService: userService,
	}
}

func (p *HandleProcessor) NodeKind() approval.NodeKind { return approval.NodeHandle }

func (p *HandleProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	if err := saveFormSnapshot(ctx, pc); err != nil {
		return nil, err
	}

	assignees, err := p.resolveAndDeduplicateAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	if len(assignees) == 0 {
		return handleEmptyAssignee(ctx, pc, p.orgService)
	}

	assignees = applyDelegation(ctx, pc.DB, pc.Instance.FlowID, assignees)

	if err := createTasksWithDelegation(ctx, pc, assignees); err != nil {
		return nil, err
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

// Predict predicts assignees without side effects.
func (p *HandleProcessor) Predict(ctx context.Context, pc *ProcessContext) ([]string, error) {
	assignees, err := p.resolveAndDeduplicateAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	if len(assignees) == 0 {
		return predictEmptyAssignee(pc)
	}

	assignees = applyDelegation(ctx, pc.DB, pc.Instance.FlowID, assignees)

	return extractUserIDs(assignees), nil
}

func (p *HandleProcessor) resolveAndDeduplicateAssignees(ctx context.Context, pc *ProcessContext) ([]approval.ResolvedAssignee, error) {
	assignees, err := resolveAssignees(ctx, pc, p.orgService, p.userService)
	if err != nil {
		return nil, err
	}

	return deduplicateAssignees(pc.Node, assignees), nil
}

