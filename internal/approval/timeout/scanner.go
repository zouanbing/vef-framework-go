package timeout

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

// Scanner scans for timed-out tasks and processes them.
type Scanner struct {
	db        orm.DB
	publisher *dispatcher.EventPublisher
}

// NewScanner creates a new timeout scanner.
func NewScanner(db orm.DB, publisher *dispatcher.EventPublisher) *Scanner {
	return &Scanner{db: db, publisher: publisher}
}

// ScanTimeouts finds tasks that have passed their deadline and processes them.
func (s *Scanner) ScanTimeouts(ctx context.Context) {
	var tasks []approval.Task

	if err := s.db.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("status", []string{string(approval.TaskPending), string(approval.TaskWaiting)}).
				IsNotNull("deadline").
				LessThan("deadline", timex.Now()).
				IsFalse("is_timeout")
		}).
		Scan(ctx); err != nil {
		logger.Errorf("Failed to scan timeout tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	logger.Infof("Found %d timed-out tasks", len(tasks))

	for i := range tasks {
		if err := s.processTimeout(ctx, &tasks[i]); err != nil {
			logger.Errorf("Failed to process timeout for task %s: %v", tasks[i].ID, err)
		}
	}
}

// processTimeout handles a single timed-out task.
func (s *Scanner) processTimeout(ctx context.Context, task *approval.Task) error {
	var node approval.FlowNode

	if err := s.db.NewSelect().
		Model(&node).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", task.NodeID)
		}).
		Scan(ctx); err != nil {
		return fmt.Errorf("load node %s: %w", task.NodeID, err)
	}

	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		// Mark the task as timed out
		task.IsTimeout = true
		if _, err := tx.NewUpdate().
			Model(task).
			WherePK().
			Select("is_timeout").
			Exec(ctx); err != nil {
			return fmt.Errorf("mark timeout: %w", err)
		}

		// Execute timeout action
		events, err := s.executeTimeoutAction(ctx, tx, task, &node)
		if err != nil {
			return fmt.Errorf("execute timeout action: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, events)
	})
}

// executeTimeoutAction executes the configured timeout action for the node.
func (s *Scanner) executeTimeoutAction(ctx context.Context, tx orm.DB, task *approval.Task, node *approval.FlowNode) ([]approval.DomainEvent, error) {
	switch node.TimeoutAction {
	case approval.TimeoutActionNotify:
		return s.recordTimeoutNotify(ctx, tx, task)
	case approval.TimeoutActionAutoPass:
		return s.autoFinishTask(ctx, tx, task, node, approval.TaskApproved)
	case approval.TimeoutActionAutoReject:
		return s.autoFinishTask(ctx, tx, task, node, approval.TaskRejected)
	case approval.TimeoutActionTransferAdmin:
		return s.transferToAdmin(ctx, tx, task, node)
	default:
		return nil, nil
	}
}

// recordTimeoutNotify returns the timeout event for the timed-out task.
// Deduplication is handled by the is_timeout flag set in processTimeout.
func (s *Scanner) recordTimeoutNotify(_ context.Context, _ orm.DB, task *approval.Task) ([]approval.DomainEvent, error) {
	return []approval.DomainEvent{
		approval.NewTaskTimeoutEvent(
			task.ID,
			task.InstanceID,
			task.NodeID,
			task.AssigneeID,
			*task.Deadline,
		),
	}, nil
}

// autoFinishTask finishes a task with the given status and logs the action.
func (s *Scanner) autoFinishTask(ctx context.Context, tx orm.DB, task *approval.Task, node *approval.FlowNode, status approval.TaskStatus) ([]approval.DomainEvent, error) {
	task.Status = status
	task.FinishedAt = new(timex.Now())

	if _, err := tx.NewUpdate().
		Model(task).
		WherePK().
		Select("status", "finished_at").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("finish task: %w", err)
	}

	actionType := approval.ActionApprove
	if status == approval.TaskRejected {
		actionType = approval.ActionReject
	}

	actionLog := &approval.ActionLog{
		InstanceID: task.InstanceID,
		NodeID:     new(task.NodeID),
		TaskID:     new(task.ID),
		Action:     actionType,
		OperatorID: "system",
		Opinion:    new("系统超时自动处理"),
	}
	if _, err := tx.NewInsert().
		Model(actionLog).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert action log: %w", err)
	}

	if status == approval.TaskApproved {
		return []approval.DomainEvent{
			approval.NewTaskApprovedEvent(
				task.ID,
				task.InstanceID,
				node.ID,
				"system",
				"任务处理超时，系统自动通过",
			),
		}, nil
	}

	return []approval.DomainEvent{
		approval.NewTaskRejectedEvent(
			task.ID,
			task.InstanceID,
			node.ID,
			"system",
			"任务处理超时，系统自动驳回",
		),
	}, nil
}

// transferToAdmin transfers a timed-out task to the node's admin users.
func (s *Scanner) transferToAdmin(ctx context.Context, tx orm.DB, task *approval.Task, node *approval.FlowNode) ([]approval.DomainEvent, error) {
	if len(node.AdminUserIDs) == 0 {
		return nil, fmt.Errorf("node %q configured TimeoutActionTransferAdmin but has no admin users", node.NodeKey)
	}

	// Finish the original task as transferred
	task.Status = approval.TaskTransferred
	task.FinishedAt = new(timex.Now())

	if _, err := tx.NewUpdate().
		Model(task).
		WherePK().
		Select("status", "finished_at").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("finish transferred task: %w", err)
	}

	events := []approval.DomainEvent{
		approval.NewTaskTransferredEvent(
			task.ID,
			task.InstanceID,
			task.NodeID,
			task.AssigneeID,
			node.AdminUserIDs[0],
			"任务处理超时，系统自动转交管理员",
		),
	}

	// Create new tasks for admin users
	for _, adminID := range node.AdminUserIDs {
		newTask := &approval.Task{
			TenantID:   task.TenantID,
			InstanceID: task.InstanceID,
			NodeID:     task.NodeID,
			AssigneeID: adminID,
			SortOrder:  0,
			Status:     approval.TaskPending,
		}
		if _, err := tx.NewInsert().
			Model(newTask).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("create admin task: %w", err)
		}

		events = append(events, approval.NewTaskCreatedEvent(
			newTask.ID,
			task.InstanceID,
			task.NodeID,
			adminID,
			nil,
		))
	}

	// Record the action
	actionLog := &approval.ActionLog{
		InstanceID:   task.InstanceID,
		NodeID:       new(task.NodeID),
		TaskID:       new(task.ID),
		Action:       approval.ActionTransfer,
		OperatorID:   "system",
		TransferToID: new(node.AdminUserIDs[0]),
		Opinion:      new("任务处理超时，系统自动转交管理员"),
	}
	if _, err := tx.NewInsert().
		Model(actionLog).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert transfer action log: %w", err)
	}

	return events, nil
}

// PreWarningTask holds data from the pre-warning scan query.
type PreWarningTask struct {
	approval.Task

	TimeoutNotifyBeforeHours int `bun:"timeout_notify_before_hours"`
}

// ScanPreWarnings finds tasks approaching their deadline and sends warning notifications.
func (s *Scanner) ScanPreWarnings(ctx context.Context) {
	var tasks []PreWarningTask

	if err := s.db.NewSelect().
		Model(&tasks).
		SelectModelColumns().
		Select("afn.timeout_notify_before_hours").
		Join((*approval.FlowNode)(nil), func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("afn.id", "at.node_id")
		}).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("status", []approval.TaskStatus{approval.TaskPending, approval.TaskWaiting}).
				IsNotNull("deadline").
				IsFalse("is_timeout").
				GreaterThan("afn.timeout_notify_before_hours", 0).
				// deadline - hours <= NOW(), equivalent to: deadline <= NOW() + hours
				LessThanOrEqualExpr("at.deadline", func(eb orm.ExprBuilder) any {
					return eb.DateAdd(eb.Now(), eb.Column("afn.timeout_notify_before_hours"), orm.UnitHour)
				}).
				IsFalse("is_pre_warning_sent")
		}).
		Scan(ctx); err != nil {
		logger.Errorf("Failed to scan pre-warning tasks: %v", err)
		return
	}

	for _, task := range tasks {
		hoursLeft := max(int(task.Deadline.Until().Hours()), 0)

		if err := s.sendPreWarning(ctx, &task.Task, hoursLeft); err != nil {
			logger.Errorf("Failed to send pre-warning for task %s: %v", task.ID, err)
		}
	}
}

// sendPreWarning marks the task as pre-warning sent and publishes the warning event.
func (s *Scanner) sendPreWarning(ctx context.Context, task *approval.Task, hoursLeft int) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		task.IsPreWarningSent = true
		if _, err := tx.NewUpdate().
			Model(task).
			WherePK().
			Select("is_pre_warning_sent").
			Exec(ctx); err != nil {
			return fmt.Errorf("mark pre-warning sent: %w", err)
		}

		evt := approval.NewTaskDeadlineWarningEvent(
			task.ID,
			task.InstanceID,
			task.NodeID,
			task.AssigneeID,
			*task.Deadline,
			hoursLeft,
		)

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{evt})
	})
}
