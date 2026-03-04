package engine

import (
	"context"
	"fmt"

	collections "github.com/ilxqx/go-collections"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

// saveFormSnapshot persists a snapshot of the form data at the current node.
func saveFormSnapshot(ctx context.Context, pc *ProcessContext) error {
	if _, err := pc.DB.NewInsert().
		Model(&approval.FormSnapshot{
			InstanceID: pc.Instance.ID,
			NodeID:     pc.Node.ID,
			FormData:   pc.FormData.ToMap(),
		}).
		Exec(ctx); err != nil {
		return fmt.Errorf("save form snapshot: %w", err)
	}

	return nil
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
// Returns the original slice unchanged if the node's DuplicateAssigneeAction is "none".
func deduplicateAssignees(node *approval.FlowNode, assignees []approval.ResolvedAssignee) []approval.ResolvedAssignee {
	if node.DuplicateAssigneeAction == approval.DuplicateAssigneeNone {
		return assignees
	}

	seen := collections.NewHashSet[string]()
	result := make([]approval.ResolvedAssignee, 0, len(assignees))

	for _, a := range assignees {
		if !seen.Add(a.UserID) {
			continue
		}

		result = append(result, a)
	}

	return result
}

// applyDelegation resolves delegation chains for each assignee, replacing delegators with delegates.
func applyDelegation(ctx context.Context, db orm.DB, flowID string, assignees []approval.ResolvedAssignee) ([]approval.ResolvedAssignee, error) {
	categoryID, err := loadFlowCategoryID(ctx, db, flowID)
	if err != nil {
		return nil, err
	}

	result := make([]approval.ResolvedAssignee, len(assignees))

	for i, assignee := range assignees {
		delegateeID, delegatorID, err := resolveDelegationChain(ctx, db, assignee.UserID, flowID, categoryID)
		if err != nil {
			return nil, err
		}

		if delegateeID != assignee.UserID {
			result[i] = approval.ResolvedAssignee{
				UserID:      delegateeID,
				DelegatorID: &delegatorID,
			}
		} else {
			result[i] = assignee
		}
	}

	return result, nil
}

// buildTask creates a base task from the process context with the given assignee ID.
func buildTask(pc *ProcessContext, assigneeID string, deadline *timex.DateTime) *approval.Task {
	return &approval.Task{
		TenantID:   pc.Instance.TenantID,
		InstanceID: pc.Instance.ID,
		NodeID:     pc.Node.ID,
		AssigneeID: assigneeID,
		SortOrder:  0,
		Status:     approval.TaskPending,
		Deadline:   deadline,
	}
}

// createTasksForUsers creates pending tasks for a list of user IDs with sortOrder=0.
// Returns a Wait result, or ErrNoAssignee if the list is empty.
func createTasksForUsers(ctx context.Context, pc *ProcessContext, userIDs []string) (*ProcessResult, error) {
	if len(userIDs) == 0 {
		return nil, ErrNoAssignee
	}

	deadline := computeDeadline(pc.Node)

	for _, uid := range userIDs {
		if _, err := pc.DB.NewInsert().Model(buildTask(pc, uid, deadline)).Exec(ctx); err != nil {
			return nil, fmt.Errorf("create task: %w", err)
		}
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

// handleEmptyAssignee handles the case when no assignees are resolved.
// The behavior depends on the node's EmptyAssigneeAction configuration.
func handleEmptyAssignee(ctx context.Context, pc *ProcessContext, assigneeService approval.AssigneeService) (*ProcessResult, error) {
	switch pc.Node.EmptyAssigneeAction {
	case approval.EmptyAssigneeAutoPass:
		return &ProcessResult{Action: NodeActionContinue}, nil

	case approval.EmptyAssigneeTransferAdmin:
		return createTasksForUsers(ctx, pc, pc.Node.AdminUserIDs)

	case approval.EmptyAssigneeTransferApplicant:
		return createTasksForUsers(ctx, pc, []string{pc.ApplicantID})

	case approval.EmptyAssigneeTransferSpecified:
		return createTasksForUsers(ctx, pc, pc.Node.FallbackUserIDs)

	case approval.EmptyAssigneeTransferSuperior:
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

// createTasksWithDelegation creates tasks for resolved assignees, setting DelegatorID when applicable.
func createTasksWithDelegation(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) error {
	deadline := computeDeadline(pc.Node)

	for _, assignee := range assignees {
		task := buildTask(pc, assignee.UserID, deadline)
		task.DelegatorID = assignee.DelegatorID

		if _, err := pc.DB.NewInsert().Model(task).Exec(ctx); err != nil {
			return fmt.Errorf("create task: %w", err)
		}
	}

	return nil
}

// getSuperior retrieves the superior user ID. Returns ErrAssigneeServiceNotConfigured if assigneeService is nil.
func getSuperior(ctx context.Context, assigneeService approval.AssigneeService, userID string) (string, error) {
	if assigneeService == nil {
		return "", ErrAssigneeServiceNotConfigured
	}

	return assigneeService.GetSuperior(ctx, userID)
}

// computeDeadline returns a deadline based on the node's TimeoutHours configuration.
// Returns nil if TimeoutHours is not set.
func computeDeadline(node *approval.FlowNode) *timex.DateTime {
	if node.TimeoutHours <= 0 {
		return nil
	}

	return new(timex.Now().AddHours(node.TimeoutHours))
}

// loadFlowCategoryID loads the category ID for a flow.
func loadFlowCategoryID(ctx context.Context, db orm.DB, flowID string) (string, error) {
	var flow approval.Flow
	flow.ID = flowID

	if err := db.NewSelect().
		Model(&flow).
		Select("category_id").
		WherePK().
		Scan(ctx); err != nil {
		return "", fmt.Errorf("load flow category: %w", err)
	}

	return flow.CategoryID, nil
}

// resolveDelegationChain resolves a delegation chain A->B->C with cycle detection.
// Matching priority: flow-specific > category-specific > global (by created_at DESC).
func resolveDelegationChain(ctx context.Context, db orm.DB, userID, flowID, flowCategoryID string) (string, string, error) {
	const maxDepth = 10

	var (
		currentID  = userID
		originalID = userID
		visited    = collections.NewHashSetFrom(userID)
		now        = timex.Now()
	)

	for range maxDepth {
		var delegations []approval.Delegation

		if err := db.NewSelect().
			Model(&delegations).
			Select("delegatee_id", "start_time", "end_time", "flow_category_id", "flow_id").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("delegator_id", currentID).
					IsTrue("is_active")
			}).
			OrderByDesc("created_at").
			Limit(100).
			Scan(ctx); err != nil {
			return "", "", fmt.Errorf("load delegations for user %s: %w", currentID, err)
		}

		if len(delegations) == 0 {
			break
		}

		matched := matchDelegation(delegations, now, flowID, flowCategoryID)
		if matched == nil {
			break
		}

		nextID := matched.DelegateeID
		if visited.Contains(nextID) {
			break
		}

		visited.Add(nextID)
		currentID = nextID
	}

	if currentID == originalID {
		return currentID, "", nil
	}

	return currentID, originalID, nil
}

// matchDelegation finds the best matching delegation with priority:
// flow-specific > category-specific > global.
func matchDelegation(delegations []approval.Delegation, now timex.DateTime, flowID, flowCategoryID string) *approval.Delegation {
	var categoryMatch, globalMatch *approval.Delegation

	for i := range delegations {
		d := &delegations[i]

		if !d.StartTime.IsZero() && now.Before(d.StartTime) {
			continue
		}

		if !d.EndTime.IsZero() && now.After(d.EndTime) {
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

		if d.FlowCategoryID == nil && globalMatch == nil {
			globalMatch = d
		}
	}

	if categoryMatch != nil {
		return categoryMatch
	}

	return globalMatch
}
