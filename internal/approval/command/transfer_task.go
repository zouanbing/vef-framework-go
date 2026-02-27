package command

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// TransferTaskCmd transfers a pending task to another user.
type TransferTaskCmd struct {
	cqrs.BaseCommand
	InstanceID   string
	TaskID       string
	OperatorID   string
	Opinion      string
	FormData     map[string]any
	TransferToID string
}

// TransferTaskHandler handles the TransferTaskCmd command.
type TransferTaskHandler struct {
	db        orm.DB
	taskSvc   *service.TaskService
	publisher *dispatcher.EventPublisher
}

// NewTransferTaskHandler creates a new TransferTaskHandler.
func NewTransferTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	pub *dispatcher.EventPublisher,
) *TransferTaskHandler {
	return &TransferTaskHandler{
		db:        db,
		taskSvc:   taskSvc,
		publisher: pub,
	}
}

func (h *TransferTaskHandler) Handle(ctx context.Context, cmd TransferTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := prepareTaskOperation(ctx, db, nil, cmd.InstanceID, cmd.TaskID, cmd.OperatorID, "", cmd.FormData)
	if err != nil {
		return cqrs.Unit{}, err
	}

	instance, task, node := tc.Instance, tc.Task, tc.Node

	if !node.IsTransferAllowed {
		return cqrs.Unit{}, service.ErrTransferNotAllowed
	}

	if cmd.TransferToID == "" {
		return cqrs.Unit{}, fmt.Errorf("transfer target user ID required")
	}

	if err := h.taskSvc.FinishTask(ctx, db, task, approval.TaskTransferred); err != nil {
		return cqrs.Unit{}, err
	}

	newTask := &approval.Task{
		TenantID:   instance.TenantID,
		InstanceID: instance.ID,
		NodeID:     task.NodeID,
		AssigneeID: cmd.TransferToID,
		SortOrder:  task.SortOrder,
		Status:     approval.TaskPending,
		Deadline:   task.Deadline,
	}
	if _, err := db.NewInsert().Model(newTask).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert transfer task: %w", err)
	}

	events := []approval.DomainEvent{
		approval.NewTaskTransferredEvent(task.ID, instance.ID, node.ID, cmd.OperatorID, cmd.TransferToID, cmd.Opinion),
		approval.NewTaskCreatedEvent(newTask.ID, instance.ID, node.ID, cmd.TransferToID, nil),
	}

	if err := insertActionLog(ctx, db, instance.ID, task, cmd.OperatorID, approval.ActionTransfer, cmd.Opinion, cmd.TransferToID, ""); err != nil {
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
