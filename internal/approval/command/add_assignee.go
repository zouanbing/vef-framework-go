package command

import (
	"context"
	"fmt"
	"slices"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// AddAssigneeCmd dynamically adds assignees to a task.
type AddAssigneeCmd struct {
	cqrs.BaseCommand

	TaskID   string
	UserIDs  []string
	AddType  approval.AddAssigneeType
	Operator approval.OperatorInfo
}

// AddAssigneeHandler handles the AddAssigneeCmd command.
type AddAssigneeHandler struct {
	db        orm.DB
	publisher *dispatcher.EventPublisher
}

// NewAddAssigneeHandler creates a new AddAssigneeHandler.
func NewAddAssigneeHandler(db orm.DB, publisher *dispatcher.EventPublisher) *AddAssigneeHandler {
	return &AddAssigneeHandler{db: db, publisher: publisher}
}

func (h *AddAssigneeHandler) Handle(ctx context.Context, cmd AddAssigneeCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var task approval.Task
	task.ID = cmd.TaskID

	if err := db.NewSelect().
		Model(&task).
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, shared.ErrTaskNotFound
	}

	var instance approval.Instance
	instance.ID = task.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		Select("status", "tenant_id").
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, shared.ErrInstanceNotFound
	}

	if instance.Status != approval.InstanceRunning {
		return cqrs.Unit{}, shared.ErrInstanceCompleted
	}

	var node approval.FlowNode
	node.ID = task.NodeID

	if err := db.NewSelect().
		Model(&node).
		Select("is_add_assignee_allowed", "add_assignee_types").
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load node: %w", err)
	}

	if !node.IsAddAssigneeAllowed {
		return cqrs.Unit{}, shared.ErrAddAssigneeNotAllowed
	}

	if task.AssigneeID != cmd.Operator.ID {
		return cqrs.Unit{}, shared.ErrNotAssignee
	}

	if !cmd.AddType.IsValid() {
		return cqrs.Unit{}, shared.ErrInvalidAddAssigneeType
	}

	if len(node.AddAssigneeTypes) > 0 && !slices.Contains(node.AddAssigneeTypes, string(cmd.AddType)) {
		return cqrs.Unit{}, shared.ErrInvalidAddAssigneeType
	}

	// Find current max sort_order for this node
	var lastTask approval.Task
	baseSortOrder := task.SortOrder
	if err := db.NewSelect().
		Model(&lastTask).
		Select("sort_order").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID).
				Equals("node_id", task.NodeID)
		}).
		OrderByDesc("sort_order").
		Limit(1).
		Scan(ctx); err == nil {
		baseSortOrder = lastTask.SortOrder
	}

	for i, userID := range cmd.UserIDs {
		newTask := &approval.Task{
			TenantID:        instance.TenantID,
			InstanceID:      instance.ID,
			NodeID:          task.NodeID,
			AssigneeID:      userID,
			SortOrder:       baseSortOrder + i + 1,
			ParentTaskID:    new(task.ID),
			AddAssigneeType: &cmd.AddType,
		}
		switch cmd.AddType {
		case approval.AddAssigneeBefore:
			newTask.Status = approval.TaskPending
			if engine.TaskStateMachine.CanTransition(task.Status, approval.TaskWaiting) {
				task.Status = approval.TaskWaiting
				if _, err := db.NewUpdate().
					Model(&task).
					Select("status").
					WherePK().
					Exec(ctx); err != nil {
					return cqrs.Unit{}, fmt.Errorf("update original task: %w", err)
				}
			}
		case approval.AddAssigneeAfter:
			newTask.Status = approval.TaskWaiting
		case approval.AddAssigneeParallel:
			newTask.Status = approval.TaskPending
		}

		if _, err := db.NewInsert().
			Model(newTask).
			Exec(ctx); err != nil {
			return cqrs.Unit{}, fmt.Errorf("insert assignee task: %w", err)
		}
	}

	actionLog := cmd.Operator.NewActionLog(instance.ID, approval.ActionAddAssignee)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)
	actionLog.AddAssigneeType = &cmd.AddType
	actionLog.AddedAssigneeIDs = cmd.UserIDs
	if _, err := db.NewInsert().
		Model(actionLog).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewAssigneesAddedEvent(instance.ID, task.NodeID, task.ID, cmd.AddType, cmd.UserIDs),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
