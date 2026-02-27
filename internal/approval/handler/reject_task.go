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

// RejectTaskCmd rejects a pending task.
type RejectTaskCmd struct {
	cqrs.CommandBase
	InstanceID string
	TaskID     string
	OperatorID string
	Opinion    string
	FormData   map[string]any
}

// RejectTaskHandler handles the RejectTaskCmd command.
type RejectTaskHandler struct {
	db        orm.DB
	taskSvc   *service.TaskService
	nodeSvc   *service.NodeService
	validSvc  *service.ValidationService
	publisher *dispatcher.EventPublisher
}

// NewRejectTaskHandler creates a new RejectTaskHandler.
func NewRejectTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	validSvc *service.ValidationService,
	pub *dispatcher.EventPublisher,
) *RejectTaskHandler {
	return &RejectTaskHandler{
		db:        db,
		taskSvc:   taskSvc,
		nodeSvc:   nodeSvc,
		validSvc:  validSvc,
		publisher: pub,
	}
}

func (h *RejectTaskHandler) Handle(ctx context.Context, cmd RejectTaskCmd) (cqrs.Unit, error) {
	db := dbFromCtx(ctx, h.db)

	instance, task, node, err := loadTaskContext(ctx, db, cmd.InstanceID, cmd.TaskID, cmd.OperatorID)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.validSvc.ValidateOpinion(node, cmd.Opinion); err != nil {
		return cqrs.Unit{}, err
	}

	mergeFormData(instance, cmd.FormData, node.FieldPermissions)

	if err := h.taskSvc.FinishTask(ctx, db, task, approval.TaskRejected); err != nil {
		return cqrs.Unit{}, err
	}

	events := []approval.DomainEvent{
		approval.NewTaskRejectedEvent(task.ID, instance.ID, node.ID, cmd.OperatorID, cmd.Opinion),
	}

	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return cqrs.Unit{}, err
	}
	events = append(events, completionEvents...)

	if err := insertActionLog(ctx, db, instance.ID, task, cmd.OperatorID, approval.ActionReject, cmd.Opinion, "", ""); err != nil {
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
