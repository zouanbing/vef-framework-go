package handler

import (
	"context"
	"fmt"
	"maps"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/orm"
)

// loadTaskContext loads and validates the instance, task, and node for task processing.
func loadTaskContext(ctx context.Context, db orm.DB, instanceID, taskID, operatorID string) (*approval.Instance, *approval.Task, *approval.FlowNode, error) {
	var instance approval.Instance
	if err := db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", instanceID)
	}).Scan(ctx); err != nil {
		return nil, nil, nil, service.ErrInstanceNotFound
	}

	if instance.Status != approval.InstanceRunning {
		return nil, nil, nil, service.ErrInstanceCompleted
	}

	var task approval.Task
	if err := db.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", taskID)
		c.Equals("instance_id", instanceID)
	}).Scan(ctx); err != nil {
		return nil, nil, nil, service.ErrTaskNotFound
	}

	if task.AssigneeID != operatorID {
		return nil, nil, nil, service.ErrNotAssignee
	}

	if task.Status != approval.TaskPending {
		return nil, nil, nil, service.ErrTaskNotPending
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
