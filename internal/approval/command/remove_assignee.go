package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// RemoveAssigneeCmd removes an assignee by canceling their task.
type RemoveAssigneeCmd struct {
	cqrs.BaseCommand

	TaskID   string
	Operator approval.OperatorInfo
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
	publisher *dispatcher.EventPublisher,
) *RemoveAssigneeHandler {
	return &RemoveAssigneeHandler{
		db: db, taskSvc: taskSvc, nodeSvc: nodeSvc, engine: eng, publisher: publisher,
	}
}

func (h *RemoveAssigneeHandler) Handle(ctx context.Context, cmd RemoveAssigneeCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var task approval.Task
	task.ID = cmd.TaskID

	if err := db.NewSelect().
		Model(&task).
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, shared.ErrTaskNotFound
	}

	var node approval.FlowNode
	node.ID = task.NodeID

	if err := db.NewSelect().
		Model(&node).
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load node: %w", err)
	}

	if !node.IsRemoveAssigneeAllowed {
		return cqrs.Unit{}, shared.ErrRemoveAssigneeNotAllowed
	}

	if !h.taskSvc.IsAuthorizedForNodeOperation(ctx, db, task, cmd.Operator.ID) {
		return cqrs.Unit{}, shared.ErrNotAssignee
	}

	canRemove, err := h.taskSvc.CanRemoveAssigneeTask(ctx, db, h.engine, &node, task)
	if err != nil {
		return cqrs.Unit{}, err
	}
	if !canRemove {
		return cqrs.Unit{}, shared.ErrLastAssigneeRemoval
	}

	originalStatus := task.Status
	if err := h.taskSvc.FinishTask(ctx, db, &task, approval.TaskRemoved); err != nil {
		return cqrs.Unit{}, err
	}

	var instance approval.Instance
	instance.ID = task.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load instance: %w", err)
	}

	if node.ApprovalMethod == approval.ApprovalSequential && originalStatus == approval.TaskPending {
		if err := h.taskSvc.ActivateNextSequentialTask(ctx, db, &instance, &node); err != nil {
			return cqrs.Unit{}, err
		}
	}

	actionLog := cmd.Operator.NewActionLog(task.InstanceID, approval.ActionRemoveAssignee)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)
	actionLog.RemovedAssigneeIDs = []string{task.AssigneeID}
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

	if _, err := db.NewUpdate().
		Model(&instance).
		Select("current_node_id", "status", "finished_at").
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, events); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
