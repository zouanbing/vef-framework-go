package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

// saveFormSnapshot persists a snapshot of the form data at the current node.
func saveFormSnapshot(ctx context.Context, pc *ProcessContext) error {
	_, err := pc.DB.NewInsert().
		Model(&approval.FormSnapshot{
			InstanceID: pc.Instance.ID,
			NodeID:     pc.Node.ID,
			FormData:   pc.FormData.ToMap(),
		}).
		Exec(ctx)

	return err
}

// resolveAssignees loads the node's assignee configs and resolves them to concrete users.
func resolveAssignees(ctx context.Context, pc *ProcessContext) ([]approval.ResolvedAssignee, error) {
	var assignees []approval.FlowNodeAssignee

	if err := pc.DB.NewSelect().
		Model(&assignees).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", pc.Node.ID)
		}).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load node assignees: %w", err)
	}

	return pc.Registry.CompositeAssigneeResolver().ResolveAll(ctx, assignees, &strategy.ResolveContext{
		DB:              pc.DB,
		ApplicantID:     pc.ApplicantID,
		ApplicantDeptID: pc.Instance.ApplicantDeptID,
		FormData:        pc.FormData,
	})
}

// deduplicateAssignees removes duplicate assignees based on UserID.
// Returns the original slice unchanged if the node's DuplicateHandlerAction is "none".
func deduplicateAssignees(node *approval.FlowNode, assignees []approval.ResolvedAssignee) []approval.ResolvedAssignee {
	if node.DuplicateHandlerAction == approval.DuplicateHandlerNone {
		return assignees
	}

	seen := make(map[string]struct{}, len(assignees))
	result := make([]approval.ResolvedAssignee, 0, len(assignees))

	for _, a := range assignees {
		if _, ok := seen[a.UserID]; ok {
			continue
		}

		seen[a.UserID] = struct{}{}
		result = append(result, a)
	}

	return result
}

// applyDelegation resolves delegation chains for each assignee, replacing delegators with delegates.
func applyDelegation(ctx context.Context, db orm.DB, flowID string, assignees []approval.ResolvedAssignee) []approval.ResolvedAssignee {
	categoryID := loadFlowCategoryID(ctx, db, flowID)
	result := make([]approval.ResolvedAssignee, len(assignees))

	for i, assignee := range assignees {
		finalID, originalID := resolveDelegationChain(ctx, db, assignee.UserID, flowID, categoryID)
		if finalID != assignee.UserID {
			result[i] = approval.ResolvedAssignee{
				UserID:         finalID,
				DelegateFromID: &originalID,
			}
		} else {
			result[i] = assignee
		}
	}

	return result
}

// createTasksForUsers creates pending tasks for a list of user IDs with sortOrder=0.
// Returns a Wait result, or ErrNoAssignee if the list is empty.
func createTasksForUsers(ctx context.Context, pc *ProcessContext, userIDs []string) (*ProcessResult, error) {
	if len(userIDs) == 0 {
		return nil, ErrNoAssignee
	}

	deadline := computeDeadline(pc.Node)

	for _, uid := range userIDs {
		task := &approval.Task{
			TenantID:   pc.Instance.TenantID,
			InstanceID: pc.Instance.ID,
			NodeID:     pc.Node.ID,
			AssigneeID: uid,
			SortOrder:  0,
			Status:     approval.TaskPending,
			Deadline:   deadline,
		}

		if _, err := pc.DB.NewInsert().Model(task).Exec(ctx); err != nil {
			return nil, fmt.Errorf("create task: %w", err)
		}
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

// handleEmptyAssignee handles the case when no assignees are resolved.
// The behavior depends on the node's EmptyHandlerAction configuration.
func handleEmptyAssignee(ctx context.Context, pc *ProcessContext, assigneeService approval.AssigneeService) (*ProcessResult, error) {
	switch pc.Node.EmptyHandlerAction {
	case approval.EmptyHandlerAutoPass:
		return &ProcessResult{Action: NodeActionContinue}, nil

	case approval.EmptyHandlerTransferAdmin:
		return createTasksForUsers(ctx, pc, pc.Node.AdminUserIDs)

	case approval.EmptyHandlerTransferApplicant:
		return createTasksForUsers(ctx, pc, []string{pc.ApplicantID})

	case approval.EmptyHandlerTransferSpecified:
		return createTasksForUsers(ctx, pc, pc.Node.FallbackUserIDs)

	case approval.EmptyHandlerTransferSuperior:
		superiorID, err := getSuperior(ctx, assigneeService, pc.ApplicantID)
		if err != nil {
			return nil, err
		}

		if superiorID == "" {
			return nil, ErrNoAssignee
		}

		return createTasksForUsers(ctx, pc, []string{superiorID})

	default:
		return nil, ErrNoAssignee
	}
}

// createTasksWithDelegation creates tasks for resolved assignees, setting DelegateFromID when applicable.
func createTasksWithDelegation(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) error {
	deadline := computeDeadline(pc.Node)

	for _, assignee := range assignees {
		task := &approval.Task{
			TenantID:   pc.Instance.TenantID,
			InstanceID: pc.Instance.ID,
			NodeID:     pc.Node.ID,
			AssigneeID: assignee.UserID,
			SortOrder:  0,
			Status:     approval.TaskPending,
			Deadline:   deadline,
		}

		if assignee.DelegateFromID != nil {
			task.DelegateFromID = assignee.DelegateFromID
		}

		if _, err := pc.DB.NewInsert().Model(task).Exec(ctx); err != nil {
			return fmt.Errorf("create task: %w", err)
		}
	}

	return nil
}

// getSuperior retrieves the superior user ID, returning empty string if assigneeService is nil.
func getSuperior(ctx context.Context, assigneeService approval.AssigneeService, userID string) (string, error) {
	if assigneeService == nil {
		return "", nil
	}

	return assigneeService.GetSuperior(ctx, userID)
}

// computeDeadline returns a deadline based on the node's TimeoutHours configuration.
// Returns nil if TimeoutHours is not set.
func computeDeadline(node *approval.FlowNode) *timex.DateTime {
	if node.TimeoutHours <= 0 {
		return nil
	}

	d := timex.DateTime(timex.Now().Unwrap().Add(time.Duration(node.TimeoutHours) * time.Hour))

	return &d
}

// loadFlowCategoryID loads the category ID for a flow.
func loadFlowCategoryID(ctx context.Context, db orm.DB, flowID string) string {
	var flow approval.Flow

	if err := db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", flowID)
	}).Scan(ctx); err != nil {
		return ""
	}

	return flow.CategoryID
}

// resolveDelegationChain resolves a delegation chain A->B->C with cycle detection.
// Matching priority: flow-specific > category-specific > global (by created_at DESC).
func resolveDelegationChain(ctx context.Context, db orm.DB, userID, flowID, flowCategoryID string) (string, string) {
	const maxDepth = 10

	currentID := userID
	originalID := userID
	visited := map[string]bool{userID: true}
	now := time.Now()

	for range maxDepth {
		var delegations []approval.Delegation

		err := db.NewSelect().Model(&delegations).Where(func(c orm.ConditionBuilder) {
			c.Equals("delegator_id", currentID)
			c.IsTrue("is_active")
		}).OrderByDesc("created_at").Limit(100).Scan(ctx)
		if err != nil {
			// DB error during delegation lookup; fall back to original assignee
			break
		}

		if len(delegations) == 0 {
			break
		}

		matched := matchDelegation(delegations, now, flowID, flowCategoryID)
		if matched == nil {
			break
		}

		nextID := matched.DelegateeID
		if visited[nextID] {
			break
		}

		visited[nextID] = true
		currentID = nextID
	}

	if currentID == originalID {
		return currentID, ""
	}

	return currentID, originalID
}

// matchDelegation finds the best matching delegation with priority:
// flow-specific > category-specific > global.
func matchDelegation(delegations []approval.Delegation, now time.Time, flowID, flowCategoryID string) *approval.Delegation {
	var categoryMatch, globalMatch *approval.Delegation

	for i := range delegations {
		d := &delegations[i]

		if !d.StartTime.IsZero() && now.Before(d.StartTime.Unwrap()) {
			continue
		}

		if !d.EndTime.IsZero() && now.After(d.EndTime.Unwrap()) {
			continue
		}

		if d.FlowCategoryID != nil && *d.FlowCategoryID != flowCategoryID {
			continue
		}

		if d.FlowID != nil && *d.FlowID != flowID {
			continue
		}

		if d.FlowID != nil {
			return d
		}

		if d.FlowCategoryID != nil && categoryMatch == nil {
			categoryMatch = d
		}

		if d.FlowID == nil && d.FlowCategoryID == nil && globalMatch == nil {
			globalMatch = d
		}
	}

	if categoryMatch != nil {
		return categoryMatch
	}

	return globalMatch
}
