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
	InstanceID string
	TaskID     string
	UserIDs    []string
	AddType    string // "before", "after", "parallel"
	OperatorID string
}

// AddAssigneeHandler handles the AddAssigneeCmd command.
type AddAssigneeHandler struct {
	db        orm.DB
	publisher *dispatcher.EventPublisher
}

// NewAddAssigneeHandler creates a new AddAssigneeHandler.
func NewAddAssigneeHandler(db orm.DB, pub *dispatcher.EventPublisher) *AddAssigneeHandler {
	return &AddAssigneeHandler{db: db, publisher: pub}
}

func (h *AddAssigneeHandler) Handle(ctx context.Context, cmd AddAssigneeCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var instance approval.Instance
	if err := db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.InstanceID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, shared.ErrInstanceNotFound
	}

	if instance.Status != approval.InstanceRunning {
		return cqrs.Unit{}, shared.ErrInstanceCompleted
	}

	var task approval.Task
	if err := db.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.TaskID)
		c.Equals("instance_id", cmd.InstanceID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, shared.ErrTaskNotFound
	}

	var node approval.FlowNode
	if err := db.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.NodeID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load node: %w", err)
	}

	if !node.IsAddAssigneeAllowed {
		return cqrs.Unit{}, shared.ErrAddAssigneeNotAllowed
	}

	if task.AssigneeID != cmd.OperatorID {
		return cqrs.Unit{}, shared.ErrNotAssignee
	}

	addType := approval.AddAssigneeType(cmd.AddType)
	if !addType.IsValid() {
		return cqrs.Unit{}, shared.ErrInvalidAddAssigneeType
	}

	if len(node.AddAssigneeTypes) > 0 && !slices.Contains(node.AddAssigneeTypes, cmd.AddType) {
		return cqrs.Unit{}, shared.ErrInvalidAddAssigneeType
	}

	// Find current max sort_order for this node
	var lastTask approval.Task
	baseSortOrder := task.SortOrder
	if err := db.NewSelect().Model(&lastTask).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instance.ID)
		c.Equals("node_id", task.NodeID)
	}).OrderByDesc("sort_order").Limit(1).Scan(ctx); err == nil {
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
			AddAssigneeType: new(string(addType)),
		}
		switch addType {
		case approval.AddAssigneeBefore:
			newTask.Status = approval.TaskPending
			if engine.TaskStateMachine.CanTransition(task.Status, approval.TaskWaiting) {
				task.Status = approval.TaskWaiting
				if _, err := db.NewUpdate().Model(&task).WherePK().Exec(ctx); err != nil {
					return cqrs.Unit{}, fmt.Errorf("update original task: %w", err)
				}
			}
		case approval.AddAssigneeAfter:
			newTask.Status = approval.TaskWaiting
		case approval.AddAssigneeParallel:
			newTask.Status = approval.TaskPending
		}

		if _, err := db.NewInsert().Model(newTask).Exec(ctx); err != nil {
			return cqrs.Unit{}, fmt.Errorf("insert assignee task: %w", err)
		}
	}

	// Action log
	actionLog := &approval.ActionLog{
		InstanceID:       instance.ID,
		NodeID:           new(task.NodeID),
		TaskID:           new(task.ID),
		Action:           approval.ActionAddAssignee,
		OperatorID:       cmd.OperatorID,
		AddAssigneeType:  new(cmd.AddType),
		AddAssigneeToIDs: cmd.UserIDs,
	}
	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewAssigneesAddedEvent(instance.ID, task.NodeID, task.ID, addType, cmd.UserIDs),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
