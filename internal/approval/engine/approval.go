package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ApprovalProcessor handles approval nodes.
type ApprovalProcessor struct {
	assigneeService approval.AssigneeService
}

// NewApprovalProcessor creates a new approval processor.
func NewApprovalProcessor(assigneeService approval.AssigneeService) *ApprovalProcessor {
	return &ApprovalProcessor{assigneeService: assigneeService}
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
		return handleEmptyAssignee(ctx, pc, p.assigneeService)
	}

	if p.isSameApplicant(assignees, pc.ApplicantID) {
		return p.handleSameApplicant(ctx, pc, assignees)
	}

	if err := p.createApprovalTasks(ctx, pc, assignees); err != nil {
		return nil, err
	}

	if pc.Node.ConsecutiveApproverAction == approval.ConsecutiveApproverAutoPass {
		return p.autoPassConsecutiveApprovers(ctx, pc)
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

func (p *ApprovalProcessor) resolveAndProcessAssignees(ctx context.Context, pc *ProcessContext) ([]approval.ResolvedAssignee, error) {
	assignees, err := resolveAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	assignees = deduplicateAssignees(assignees)

	assignees, err = applyDelegation(ctx, pc.DB, pc.Instance.FlowID, assignees)
	if err != nil {
		return nil, err
	}

	return assignees, nil
}

// createApprovalTasks creates tasks with sequential ordering support.
func (p *ApprovalProcessor) createApprovalTasks(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) error {
	for i, assignee := range assignees {
		sortOrder := 0
		status := approval.TaskPending

		if pc.Node.ApprovalMethod == approval.ApprovalSequential {
			sortOrder = i + 1

			if i > 0 {
				status = approval.TaskWaiting
			}
		}

		task := &approval.Task{
			TenantID:    pc.Instance.TenantID,
			InstanceID:  pc.Instance.ID,
			NodeID:      pc.Node.ID,
			AssigneeID:  assignee.UserID,
			DelegatorID: assignee.DelegatorID,
			SortOrder:   sortOrder,
			Status:      status,
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

	case approval.SameApplicantTransferSuperior:
		superiorID, err := getSuperior(ctx, p.assigneeService, pc.ApplicantID)
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

// autoPassConsecutiveApprovers marks tasks as approved for assignees who already
// approved in the immediately preceding approval node.
func (p *ApprovalProcessor) autoPassConsecutiveApprovers(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	prevApprovers, err := findPreviousApprovalApprovers(ctx, pc.DB, pc.Instance, pc.Node.ID)
	if err != nil {
		return nil, err
	}

	if prevApprovers.Size() == 0 {
		return &ProcessResult{Action: NodeActionWait}, nil
	}

	var tasks []approval.Task

	if err := pc.DB.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", pc.Instance.ID).
				Equals("node_id", pc.Node.ID)
		}).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks for consecutive approver check: %w", err)
	}

	now := timex.Now()
	autoPassedAny := false

	for i := range tasks {
		task := &tasks[i]

		if task.Status != approval.TaskPending || !prevApprovers.Contains(task.AssigneeID) {
			continue
		}

		task.Status = approval.TaskApproved
		task.FinishedAt = new(now)

		if _, err := pc.DB.NewUpdate().
			Model(task).
			Select("status", "finished_at").
			WherePK().
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("auto-pass consecutive approver task: %w", err)
		}

		autoPassedAny = true

		// For sequential approval, activate the next waiting task
		if pc.Node.ApprovalMethod == approval.ApprovalSequential {
			for j := i + 1; j < len(tasks); j++ {
				if tasks[j].Status == approval.TaskWaiting {
					tasks[j].Status = approval.TaskPending

					if _, err := pc.DB.NewUpdate().
						Model(&tasks[j]).
						Select("status").
						WherePK().
						Exec(ctx); err != nil {
						return nil, fmt.Errorf("activate next sequential task: %w", err)
					}

					break
				}
			}
		}
	}

	if !autoPassedAny {
		return &ProcessResult{Action: NodeActionWait}, nil
	}

	// If all tasks are now complete, advance to the next node
	allComplete := !slices.ContainsFunc(tasks, func(t approval.Task) bool {
		return t.Status == approval.TaskPending || t.Status == approval.TaskWaiting
	})

	if allComplete {
		return &ProcessResult{Action: NodeActionContinue}, nil
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

func (p *ApprovalProcessor) isSameApplicant(assignees []approval.ResolvedAssignee, applicantID string) bool {
	if len(assignees) == 0 {
		return false
	}

	return !slices.ContainsFunc(assignees, func(a approval.ResolvedAssignee) bool {
		return a.UserID != applicantID
	})
}
