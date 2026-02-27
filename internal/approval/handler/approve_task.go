package handler

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// ApproveTaskCmd approves (or handles) a pending task.
type ApproveTaskCmd struct {
	cqrs.CommandBase
	InstanceID string
	TaskID     string
	OperatorID string
	Opinion    string
	FormData   map[string]any
}

// ApproveTaskHandler handles the ApproveTaskCmd command.
type ApproveTaskHandler struct {
	db        orm.DB
	taskSvc   *service.TaskService
	nodeSvc   *service.NodeService
	validSvc  *service.ValidationService
	publisher *dispatcher.EventPublisher
}

// NewApproveTaskHandler creates a new ApproveTaskHandler.
func NewApproveTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	validSvc *service.ValidationService,
	pub *dispatcher.EventPublisher,
) *ApproveTaskHandler {
	return &ApproveTaskHandler{
		db:        db,
		taskSvc:   taskSvc,
		nodeSvc:   nodeSvc,
		validSvc:  validSvc,
		publisher: pub,
	}
}

func (h *ApproveTaskHandler) Handle(ctx context.Context, cmd ApproveTaskCmd) (cqrs.Unit, error) {
	db := dbFromCtx(ctx, h.db)

	instance, task, node, err := loadTaskContext(ctx, db, cmd.InstanceID, cmd.TaskID, cmd.OperatorID)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.validSvc.ValidateOpinion(node, cmd.Opinion); err != nil {
		return cqrs.Unit{}, err
	}

	mergeFormData(instance, cmd.FormData, node.FieldPermissions)

	targetStatus := approval.TaskApproved
	if node.NodeKind == approval.NodeHandle {
		targetStatus = approval.TaskHandled
	}

	if err := h.taskSvc.FinishTask(ctx, db, task, targetStatus); err != nil {
		return cqrs.Unit{}, err
	}

	events := []approval.DomainEvent{
		approval.NewTaskApprovedEvent(task.ID, instance.ID, node.ID, cmd.OperatorID, cmd.Opinion),
	}

	if node.ApprovalMethod == approval.ApprovalSequential {
		if err := h.taskSvc.ActivateNextSequentialTask(ctx, db, instance, node); err != nil {
			return cqrs.Unit{}, err
		}
	}

	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return cqrs.Unit{}, err
	}
	events = append(events, completionEvents...)

	if err := insertActionLog(ctx, db, instance.ID, task, cmd.OperatorID, approval.ActionApprove, cmd.Opinion, "", ""); err != nil {
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
