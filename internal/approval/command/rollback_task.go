package command

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/result"
)

// RollbackTaskCmd rolls back a task to a previous node.
type RollbackTaskCmd struct {
	cqrs.BaseCommand
	InstanceID   string
	TaskID       string
	OperatorID   string
	Opinion      string
	FormData     map[string]any
	TargetNodeID string
}

// RollbackTaskHandler handles the RollbackTaskCmd command.
type RollbackTaskHandler struct {
	db        orm.DB
	taskSvc   *service.TaskService
	validSvc  *service.ValidationService
	engine    *engine.FlowEngine
	publisher *dispatcher.EventPublisher
}

// NewRollbackTaskHandler creates a new RollbackTaskHandler.
func NewRollbackTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	validSvc *service.ValidationService,
	eng *engine.FlowEngine,
	pub *dispatcher.EventPublisher,
) *RollbackTaskHandler {
	return &RollbackTaskHandler{
		db:        db,
		taskSvc:   taskSvc,
		validSvc:  validSvc,
		engine:    eng,
		publisher: pub,
	}
}

func (h *RollbackTaskHandler) Handle(ctx context.Context, cmd RollbackTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := prepareTaskOperation(ctx, db, nil, cmd.InstanceID, cmd.TaskID, cmd.OperatorID, "", cmd.FormData)
	if err != nil {
		return cqrs.Unit{}, err
	}

	instance, task, node := tc.Instance, tc.Task, tc.Node

	if !node.IsRollbackAllowed {
		return cqrs.Unit{}, service.ErrRollbackNotAllowed
	}

	if cmd.TargetNodeID == "" {
		return cqrs.Unit{}, fmt.Errorf("target node ID required for rollback")
	}

	if err := h.validSvc.ValidateRollbackTarget(ctx, db, instance, node, cmd.TargetNodeID); err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.taskSvc.FinishTask(ctx, db, task, approval.TaskRollback); err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.taskSvc.CancelRemainingTasks(ctx, db, instance.ID, node.ID); err != nil {
		return cqrs.Unit{}, err
	}

	// Restore form snapshot if rollback data strategy is "keep"
	if node.RollbackDataStrategy == approval.RollbackDataKeep {
		var snapshot approval.FormSnapshot

		err := db.NewSelect().Model(&snapshot).Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instance.ID)
			c.Equals("node_id", cmd.TargetNodeID)
		}).Scan(ctx)

		switch {
		case err == nil && snapshot.FormData != nil:
			instance.FormData = snapshot.FormData
		case err != nil && !result.IsRecordNotFound(err):
			return cqrs.Unit{}, fmt.Errorf("load form snapshot: %w", err)
		}
	}

	instance.CurrentNodeID = new(cmd.TargetNodeID)

	var targetNode approval.FlowNode
	if err := db.NewSelect().Model(&targetNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.TargetNodeID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("find target node: %w", err)
	}

	if err := h.engine.ProcessNode(ctx, db, instance, &targetNode); err != nil {
		return cqrs.Unit{}, fmt.Errorf("process rollback target node: %w", err)
	}

	events := []approval.DomainEvent{
		approval.NewInstanceRolledBackEvent(instance.ID, node.ID, cmd.TargetNodeID, cmd.OperatorID),
	}

	if err := insertActionLog(ctx, db, instance.ID, task, cmd.OperatorID, approval.ActionRollback, cmd.Opinion, "", cmd.TargetNodeID); err != nil {
		return cqrs.Unit{}, err
	}

	if _, err := db.NewUpdate().Model(instance).WherePK().Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, events); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
