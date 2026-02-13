package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/null"
)

var ErrNoAssignee = errors.New("no assignee found")

// ApprovalProcessor handles approval nodes.
type ApprovalProcessor struct {
	orgService  approval.OrganizationService
	userService approval.UserService
}

// NewApprovalProcessor creates a new approval processor.
func NewApprovalProcessor(orgService approval.OrganizationService, userService approval.UserService) *ApprovalProcessor {
	return &ApprovalProcessor{
		orgService:  orgService,
		userService: userService,
	}
}

func (p *ApprovalProcessor) NodeKind() approval.NodeKind { return approval.NodeApproval }

func (p *ApprovalProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	if err := saveFormSnapshot(ctx, pc); err != nil {
		return nil, err
	}

	assignees, err := p.resolveAndProcessAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	if len(assignees) == 0 {
		return handleEmptyAssignee(ctx, pc, p.orgService)
	}

	if p.isSameApplicant(assignees, pc.ApplicantID) {
		return p.handleSameApplicant(ctx, pc, assignees)
	}

	if err := p.createApprovalTasks(ctx, pc, assignees); err != nil {
		return nil, err
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

// Predict predicts assignees without side effects.
func (p *ApprovalProcessor) Predict(ctx context.Context, pc *ProcessContext) ([]string, error) {
	assignees, err := p.resolveAndProcessAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	if len(assignees) == 0 {
		return predictEmptyAssignee(pc)
	}

	if p.isSameApplicant(assignees, pc.ApplicantID) {
		return p.predictSameApplicant(ctx, pc)
	}

	return extractUserIDs(assignees), nil
}

func (p *ApprovalProcessor) resolveAndProcessAssignees(ctx context.Context, pc *ProcessContext) ([]approval.ResolvedAssignee, error) {
	assignees, err := resolveAssignees(ctx, pc, p.orgService, p.userService)
	if err != nil {
		return nil, err
	}

	assignees = deduplicateAssignees(pc.Node, assignees)
	assignees = applyDelegation(ctx, pc.DB, pc.Instance.FlowID, assignees)

	return assignees, nil
}

// createApprovalTasks creates tasks with sequential ordering support.
func (p *ApprovalProcessor) createApprovalTasks(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) error {
	for i, assignee := range assignees {
		sortOrder := 0
		status := string(approval.TaskPending)

		if pc.Node.ApprovalMethod == approval.ApprovalSequential {
			sortOrder = i + 1

			if i > 0 {
				status = string(approval.TaskWaiting)
			}
		}

		task := &approval.Task{
			InstanceID: pc.Instance.ID,
			NodeID:     pc.Node.ID,
			AssigneeID: assignee.UserID,
			SortOrder:  sortOrder,
			Status:     status,
		}

		if assignee.DelegateFromID != "" {
			task.DelegateFromID = null.StringFrom(assignee.DelegateFromID)
		}

		if _, err := pc.DB.NewInsert().Model(task).Exec(ctx); err != nil {
			return fmt.Errorf("create approval task: %w", err)
		}
	}

	return nil
}

func (p *ApprovalProcessor) handleSameApplicant(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) (*ProcessResult, error) {
	switch pc.Node.SameApplicantAction {
	case approval.SameApplicantAutoPass:
		return &ProcessResult{Action: NodeActionContinue}, nil

	case approval.SameApplicantSelfApprove:
		if err := createTasksWithDelegation(ctx, pc, assignees); err != nil {
			return nil, err
		}

		return &ProcessResult{Action: NodeActionWait}, nil

	case approval.SameApplicantTransferAdmin:
		return createTasksForUsers(ctx, pc, pc.Node.AdminUserIDs)

	case approval.SameApplicantTransferSuperior:
		superiorID, err := getSuperior(ctx, p.orgService, pc.ApplicantID)
		if err != nil {
			return nil, err
		}

		if superiorID == "" {
			return nil, ErrNoAssignee
		}

		return createTasksForUsers(ctx, pc, []string{superiorID})

	default:
		if err := createTasksWithDelegation(ctx, pc, assignees); err != nil {
			return nil, err
		}

		return &ProcessResult{Action: NodeActionWait}, nil
	}
}

func (p *ApprovalProcessor) isSameApplicant(assignees []approval.ResolvedAssignee, applicantID string) bool {
	if len(assignees) == 0 {
		return false
	}

	return !slices.ContainsFunc(assignees, func(a approval.ResolvedAssignee) bool {
		return a.UserID != applicantID
	})
}

func (p *ApprovalProcessor) predictSameApplicant(ctx context.Context, pc *ProcessContext) ([]string, error) {
	switch pc.Node.SameApplicantAction {
	case approval.SameApplicantAutoPass:
		return nil, nil
	case approval.SameApplicantTransferSuperior:
		superiorID, err := getSuperior(ctx, p.orgService, pc.ApplicantID)
		if err != nil {
			return nil, err
		}

		if superiorID == "" {
			return nil, ErrNoAssignee
		}

		return []string{superiorID}, nil
	default:
		return []string{pc.ApplicantID}, nil
	}
}
