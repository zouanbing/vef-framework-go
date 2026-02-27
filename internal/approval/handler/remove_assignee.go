package handler

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// RemoveAssigneeCmd removes an assignee by canceling their task.
type RemoveAssigneeCmd struct {
	cqrs.CommandBase
	TaskID     string
	OperatorID string
}

// RemoveAssigneeHandler handles the RemoveAssigneeCmd command.
type RemoveAssigneeHandler struct {
	db        orm.DB
	taskSvc   *service.TaskService
	nodeSvc   *service.NodeService
	engine    *engine.FlowEngine
	publisher *dispatcher.EventPublisher
}

// NewRemoveAssigneeHandler creates a new RemoveAssigneeHandler.
func NewRemoveAssigneeHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	eng *engine.FlowEngine,
	pub *dispatcher.EventPublisher,
) *RemoveAssigneeHandler {
	return &RemoveAssigneeHandler{
		db: db, taskSvc: taskSvc, nodeSvc: nodeSvc, engine: eng, publisher: pub,
	}
}

func (h *RemoveAssigneeHandler) Handle(ctx context.Context, cmd RemoveAssigneeCmd) (cqrs.Unit, error) {
	db := dbFromCtx(ctx, h.db)

	var task approval.Task
	if err := db.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.TaskID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, service.ErrTaskNotFound
	}

	var node approval.FlowNode
	if err := db.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.NodeID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load node: %w", err)
	}

	if !node.IsRemoveAssigneeAllowed {
		return cqrs.Unit{}, service.ErrRemoveAssigneeNotAllowed
	}

	if !h.taskSvc.IsAuthorizedForNodeOperation(ctx, db, task, cmd.OperatorID) {
		return cqrs.Unit{}, service.ErrNotAssignee
	}

	canRemove, err := h.taskSvc.CanRemoveAssigneeTask(ctx, db, h.engine, &node, task)
	if err != nil {
		return cqrs.Unit{}, err
	}
	if !canRemove {
		return cqrs.Unit{}, service.ErrLastAssigneeRemoval
	}

	originalStatus := task.Status
	if err := h.taskSvc.FinishTask(ctx, db, &task, approval.TaskRemoved); err != nil {
		return cqrs.Unit{}, err
	}

	var instance approval.Instance
	if err := db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.InstanceID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load instance: %w", err)
	}

	if node.ApprovalMethod == approval.ApprovalSequential && originalStatus == approval.TaskPending {
		if err := h.taskSvc.ActivateNextSequentialTask(ctx, db, &instance, &node); err != nil {
			return cqrs.Unit{}, err
		}
	}

	// Action log
	actionLog := &approval.ActionLog{
		InstanceID:        task.InstanceID,
		NodeID:            new(task.NodeID),
		TaskID:            new(task.ID),
		Action:            approval.ActionRemoveAssignee,
		OperatorID:        cmd.OperatorID,
		RemoveAssigneeIDs: []string{task.AssigneeID},
	}
	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	events := []approval.DomainEvent{
		approval.NewAssigneesRemovedEvent(task.InstanceID, task.NodeID, task.ID, []string{task.AssigneeID}),
	}

	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, &instance, &node)
	if err != nil {
		return cqrs.Unit{}, err
	}
	events = append(events, completionEvents...)

	if _, err := db.NewUpdate().Model(&instance).WherePK().Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, events); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
