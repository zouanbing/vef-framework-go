package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/result"
	"github.com/ilxqx/vef-framework-go/timex"
)

// cancelableTaskStatuses lists statuses eligible for bulk cancellation.
var cancelableTaskStatuses = []string{string(approval.TaskPending), string(approval.TaskWaiting)}

// TaskService provides task-level domain operations.
type TaskService struct{}

// NewTaskService creates a new TaskService.
func NewTaskService() *TaskService {
	return &TaskService{}
}

// FinishTask transitions a task to the given status and sets its FinishedAt timestamp.
func (s *TaskService) FinishTask(ctx context.Context, db orm.DB, task *approval.Task, status approval.TaskStatus) error {
	if !engine.TaskStateMachine.CanTransition(task.Status, status) {
		return shared.ErrInvalidTaskTransition
	}

	task.Status = status
	task.FinishedAt = new(timex.Now())

	if _, err := db.NewUpdate().Model(task).WherePK().Exec(ctx); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	return nil
}

// ActivateNextSequentialTask activates the next waiting task in sequential approval.
func (s *TaskService) ActivateNextSequentialTask(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) error {
	var nextTask approval.Task

	err := db.NewSelect().Model(&nextTask).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instance.ID)
		c.Equals("node_id", node.ID)
		c.Equals("status", approval.TaskWaiting)
	}).OrderBy("sort_order").Limit(1).Scan(ctx)

	if err != nil {
		if result.IsRecordNotFound(err) {
			return nil
		}

		return fmt.Errorf("find next sequential task: %w", err)
	}

	if !engine.TaskStateMachine.CanTransition(approval.TaskWaiting, approval.TaskPending) {
		return nil
	}

	nextTask.Status = approval.TaskPending

	_, err = db.NewUpdate().Model(&nextTask).WherePK().Exec(ctx)

	return err
}

// CancelRemainingTasks cancels all pending/waiting tasks on the given node.
func (s *TaskService) CancelRemainingTasks(ctx context.Context, db orm.DB, instanceID, nodeID string) error {
	_, err := db.NewUpdate().Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instanceID)
			c.Equals("node_id", nodeID)
			c.In("status", cancelableTaskStatuses)
		}).Exec(ctx)

	return err
}

// CancelInstanceTasks cancels all pending/waiting tasks for an entire instance.
func (s *TaskService) CancelInstanceTasks(ctx context.Context, db orm.DB, instanceID string) error {
	_, err := db.NewUpdate().Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instanceID)
			c.In("status", cancelableTaskStatuses)
		}).Exec(ctx)

	return err
}

// IsAuthorizedForNodeOperation checks if the operator is authorized to perform
// node-level operations (e.g., remove assignee). Returns true if the operator
// is a peer assignee on the same node or a flow admin.
func (s *TaskService) IsAuthorizedForNodeOperation(ctx context.Context, db orm.DB, task approval.Task, operatorID string) bool {
	peerCount, err := db.NewSelect().Model((*approval.Task)(nil)).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", task.InstanceID)
		c.Equals("node_id", task.NodeID)
		c.Equals("assignee_id", operatorID)
		c.In("status", []string{string(approval.TaskPending), string(approval.TaskWaiting)})
	}).Count(ctx)
	if err == nil && peerCount > 0 {
		return true
	}

	var instance approval.Instance

	if err := db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.InstanceID)
	}).Scan(ctx); err != nil {
		return false
	}

	var flow approval.Flow

	if err := db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", instance.FlowID)
	}).Scan(ctx); err != nil {
		return false
	}

	return slices.Contains(flow.AdminUserIDs, operatorID)
}

// CanRemoveAssigneeTask determines whether removing a task can still drive the
// node to progress (either through remaining actionable tasks or immediate
// completion under pass-rule evaluation).
func (s *TaskService) CanRemoveAssigneeTask(ctx context.Context, db orm.DB, eng *engine.FlowEngine, node *approval.FlowNode, task approval.Task) (bool, error) {
	var tasks []approval.Task
	if err := db.NewSelect().Model(&tasks).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", task.InstanceID)
		c.Equals("node_id", task.NodeID)
	}).Scan(ctx); err != nil {
		return false, fmt.Errorf("query node tasks: %w", err)
	}

	hasOtherActionable := false
	simulatedTasks := make([]approval.Task, 0, len(tasks))
	for _, current := range tasks {
		if current.ID == task.ID {
			current.Status = approval.TaskRemoved
		} else if current.Status == approval.TaskPending || current.Status == approval.TaskWaiting {
			hasOtherActionable = true
		}

		simulatedTasks = append(simulatedTasks, current)
	}

	if hasOtherActionable {
		return true, nil
	}

	evalResult, err := eng.EvaluatePassRuleWithTasks(node, simulatedTasks)
	if err != nil {
		return false, err
	}

	return evalResult != approval.PassRulePending, nil
}
