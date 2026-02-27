package command

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

// WithdrawCmd withdraws an approval instance.
type WithdrawCmd struct {
	cqrs.BaseCommand
	InstanceID string
	OperatorID string
	Reason     string
}

// WithdrawHandler handles the WithdrawCmd command.
type WithdrawHandler struct {
	db        orm.DB
	taskSvc   *service.TaskService
	publisher *dispatcher.EventPublisher
}

// NewWithdrawHandler creates a new WithdrawHandler.
func NewWithdrawHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	pub *dispatcher.EventPublisher,
) *WithdrawHandler {
	return &WithdrawHandler{db: db, taskSvc: taskSvc, publisher: pub}
}

func (h *WithdrawHandler) Handle(ctx context.Context, cmd WithdrawCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var instance approval.Instance
	if err := db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.InstanceID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, shared.ErrInstanceNotFound
	}

	if instance.ApplicantID != cmd.OperatorID {
		return cqrs.Unit{}, shared.ErrNotApplicant
	}

	if !engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceWithdrawn) {
		return cqrs.Unit{}, shared.ErrWithdrawNotAllowed
	}

	now := timex.Now()
	instance.Status = approval.InstanceWithdrawn
	instance.FinishedAt = &now

	if _, err := db.NewUpdate().Model(&instance).WherePK().Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	if err := h.taskSvc.CancelInstanceTasks(ctx, db, cmd.InstanceID); err != nil {
		return cqrs.Unit{}, fmt.Errorf("cancel tasks on withdraw: %w", err)
	}

	actionLog := &approval.ActionLog{
		InstanceID: cmd.InstanceID,
		Action:     approval.ActionWithdraw,
		OperatorID: cmd.OperatorID,
	}
	if cmd.Reason != "" {
		actionLog.Opinion = new(cmd.Reason)
	}
	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewInstanceWithdrawnEvent(cmd.InstanceID, cmd.OperatorID),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
