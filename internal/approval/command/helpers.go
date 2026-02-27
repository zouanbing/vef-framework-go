package command

import (
	"context"
	"fmt"
	"maps"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/orm"
)

// TaskContext holds the validated context for task processing operations.
type TaskContext struct {
	Instance *approval.Instance
	Task     *approval.Task
	Node     *approval.FlowNode
}

// prepareTaskOperation loads task context, validates the opinion, and merges editable form data.
// This is the common preamble for approve, reject, transfer, and rollback handlers.
func prepareTaskOperation(
	ctx context.Context,
	db orm.DB,
	validSvc *service.ValidationService,
	instanceID, taskID, operatorID string,
	opinion string,
	formData map[string]any,
) (*TaskContext, error) {
	instance, task, node, err := loadTaskContext(ctx, db, instanceID, taskID, operatorID)
	if err != nil {
		return nil, err
	}

	if validSvc != nil {
		if err := validSvc.ValidateOpinion(node, opinion); err != nil {
			return nil, err
		}
	}

	mergeFormData(instance, formData, node.FieldPermissions)

	return &TaskContext{Instance: instance, Task: task, Node: node}, nil
}

// loadTaskContext loads and validates the instance, task, and node for task processing.
func loadTaskContext(ctx context.Context, db orm.DB, instanceID, taskID, operatorID string) (*approval.Instance, *approval.Task, *approval.FlowNode, error) {
	var instance approval.Instance
	if err := db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", instanceID)
	}).Scan(ctx); err != nil {
		return nil, nil, nil, shared.ErrInstanceNotFound
	}

	if instance.Status != approval.InstanceRunning {
		return nil, nil, nil, shared.ErrInstanceCompleted
	}

	var task approval.Task
	if err := db.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", taskID)
		c.Equals("instance_id", instanceID)
	}).Scan(ctx); err != nil {
		return nil, nil, nil, shared.ErrTaskNotFound
	}

	if task.AssigneeID != operatorID {
		return nil, nil, nil, shared.ErrNotAssignee
	}

	if task.Status != approval.TaskPending {
		return nil, nil, nil, shared.ErrTaskNotPending
	}

	var node approval.FlowNode
	if err := db.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.NodeID)
	}).Scan(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("load node: %w", err)
	}

	return &instance, &task, &node, nil
}

// mergeFormData filters editable form data and merges it into the instance.
func mergeFormData(instance *approval.Instance, formData map[string]any, permissions map[string]approval.Permission) {
	if len(formData) == 0 {
		return
	}

	editableData := service.FilterEditableFormData(formData, permissions)
	if len(editableData) == 0 {
		return
	}

	if instance.FormData == nil {
		instance.FormData = make(map[string]any, len(editableData))
	}

	maps.Copy(instance.FormData, editableData)
}

// insertActionLog creates and inserts an action log entry.
func insertActionLog(ctx context.Context, db orm.DB, instanceID string, task *approval.Task, operatorID string, action approval.ActionType, opinion, transferToID, rollbackToNodeID string) error {
	actionLog := &approval.ActionLog{
		InstanceID: instanceID,
		NodeID:     new(task.NodeID),
		TaskID:     new(task.ID),
		Action:     action,
		OperatorID: operatorID,
	}

	if opinion != "" {
		actionLog.Opinion = new(opinion)
	}

	if transferToID != "" {
		actionLog.TransferToID = new(transferToID)
	}

	if rollbackToNodeID != "" {
		actionLog.RollbackToNodeID = new(rollbackToNodeID)
	}

	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return fmt.Errorf("insert action log: %w", err)
	}

	return nil
}
