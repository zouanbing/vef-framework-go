package command

import (
	"context"
	"fmt"
	"time"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/result"
	"github.com/ilxqx/vef-framework-go/timex"
)

// UrgeTaskCmd sends an urge notification for a pending task.
type UrgeTaskCmd struct {
	cqrs.BaseCommand
	InstanceID string
	TaskID     string
	UrgerID    string
	Message    string
}

// UrgeTaskHandler handles the UrgeTaskCmd command.
type UrgeTaskHandler struct {
	db        orm.DB
	publisher *dispatcher.EventPublisher
}

// NewUrgeTaskHandler creates a new UrgeTaskHandler.
func NewUrgeTaskHandler(db orm.DB, pub *dispatcher.EventPublisher) *UrgeTaskHandler {
	return &UrgeTaskHandler{db: db, publisher: pub}
}

func (h *UrgeTaskHandler) Handle(ctx context.Context, cmd UrgeTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	// Load the task
	var task approval.Task
	if err := db.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.TaskID)
		c.Equals("instance_id", cmd.InstanceID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, service.ErrTaskNotFound
	}

	// Only pending or waiting tasks can be urged
	if task.Status != approval.TaskPending && task.Status != approval.TaskWaiting {
		return cqrs.Unit{}, service.ErrTaskNotPending
	}

	// Load node to check cooldown settings
	var node approval.FlowNode
	if err := db.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.NodeID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load node: %w", err)
	}

	// Enforce cooldown
	cooldownMinutes := node.UrgeCooldownMinutes
	if cooldownMinutes <= 0 {
		cooldownMinutes = 30
	}

	cooldownSince := timex.DateTime(time.Now().Add(-time.Duration(cooldownMinutes) * time.Minute))

	existingCount, err := db.NewSelect().Model((*approval.UrgeRecord)(nil)).Where(func(c orm.ConditionBuilder) {
		c.Equals("task_id", cmd.TaskID)
		c.Equals("urger_id", cmd.UrgerID)
		c.GreaterThan("created_at", cooldownSince)
	}).Count(ctx)
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("check urge cooldown: %w", err)
	}

	if existingCount > 0 {
		return cqrs.Unit{}, result.Err(
			fmt.Sprintf("催办操作过于频繁，请 %d 分钟后再试", cooldownMinutes),
			result.WithCode(service.ErrCodeUrgeCooldown),
		)
	}

	// Insert urge record
	record := &approval.UrgeRecord{
		InstanceID:   cmd.InstanceID,
		NodeID:       task.NodeID,
		TaskID:       new(cmd.TaskID),
		UrgerID:      cmd.UrgerID,
		TargetUserID: task.AssigneeID,
		Message:      cmd.Message,
	}
	if _, err := db.NewInsert().Model(record).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert urge record: %w", err)
	}

	// Publish urge event
	event := approval.NewTaskUrgedEvent(
		cmd.InstanceID, task.NodeID, cmd.TaskID,
		cmd.UrgerID, task.AssigneeID, cmd.Message,
	)

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{event}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
