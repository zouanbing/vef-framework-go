package timeout

import (
	"context"
	"fmt"
	"time"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/internal/log"
	"github.com/ilxqx/vef-framework-go/null"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

var logger = log.Named("approval:timeout")

// Scanner scans for timed-out tasks and processes them.
type Scanner struct {
	db        orm.DB
	publisher *publisher.EventPublisher
}

// NewScanner creates a new timeout scanner.
func NewScanner(db orm.DB, pub *publisher.EventPublisher) *Scanner {
	return &Scanner{db: db, publisher: pub}
}

// ScanTimeouts finds tasks that have passed their deadline and processes them.
func (s *Scanner) ScanTimeouts(ctx context.Context) {
	var tasks []approval.Task

	err := s.db.NewSelect().Model(&tasks).Where(func(c orm.ConditionBuilder) {
		c.In("status", []string{string(approval.TaskPending), string(approval.TaskWaiting)})
		c.IsNotNull("deadline")
		c.LessThan("deadline", timex.Now())
		c.Equals("is_timeout", false)
	}).Scan(ctx)
	if err != nil {
		logger.Error("Failed to scan timeout tasks: " + err.Error())
		return
	}

	if len(tasks) == 0 {
		return
	}

	logger.Info(fmt.Sprintf("Found %d timed-out tasks", len(tasks)))

	for i := range tasks {
		if err := s.processTimeout(ctx, &tasks[i]); err != nil {
			logger.Error(fmt.Sprintf("Failed to process timeout for task %s: %s", tasks[i].ID, err.Error()))
		}
	}
}

// processTimeout handles a single timed-out task.
func (s *Scanner) processTimeout(ctx context.Context, task *approval.Task) error {
	var node approval.FlowNode

	if err := s.db.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.NodeID)
	}).Scan(ctx); err != nil {
		return fmt.Errorf("load node %s: %w", task.NodeID, err)
	}

	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		// Mark the task as timed out
		task.IsTimeout = true
		if _, err := tx.NewUpdate().Model(task).WherePK().Select("is_timeout").Exec(ctx); err != nil {
			return fmt.Errorf("mark timeout: %w", err)
		}

		// Record timeout notification
		notify := &approval.TimeoutNotify{
			TaskID:     task.ID,
			NotifyType: approval.TimeoutNotifyTimeout,
		}
		notify.ID = id.Generate()
		notify.CreatedBy = "system"

		if _, err := tx.NewInsert().Model(notify).Exec(ctx); err != nil {
			return fmt.Errorf("insert timeout notify: %w", err)
		}

		events := []approval.DomainEvent{
			approval.NewTaskTimeoutEvent(task.ID, task.InstanceID, task.NodeID, task.AssigneeID, time.Time(task.Deadline.V)),
		}

		// Execute timeout action
		actionEvents, err := s.executeTimeoutAction(ctx, tx, task, &node)
		if err != nil {
			return fmt.Errorf("execute timeout action: %w", err)
		}

		events = append(events, actionEvents...)

		return s.publisher.PublishAll(ctx, tx, events)
	})
}

// executeTimeoutAction executes the configured timeout action for the node.
func (s *Scanner) executeTimeoutAction(ctx context.Context, tx orm.DB, task *approval.Task, node *approval.FlowNode) ([]approval.DomainEvent, error) {
	switch node.TimeoutAction {
	case approval.TimeoutActionAutoPass:
		return s.autoFinishTask(ctx, tx, task, node, approval.TaskApproved)
	case approval.TimeoutActionAutoReject:
		return s.autoFinishTask(ctx, tx, task, node, approval.TaskRejected)
	case approval.TimeoutActionTransferAdmin:
		return s.transferToAdmin(ctx, tx, task, node)
	default:
		// TimeoutActionNone, TimeoutActionNotify: no further action
		return nil, nil
	}
}

// autoFinishTask finishes a task with the given status and logs the action.
func (s *Scanner) autoFinishTask(ctx context.Context, tx orm.DB, task *approval.Task, node *approval.FlowNode, status approval.TaskStatus) ([]approval.DomainEvent, error) {
	task.Status = status
	task.FinishedAt = null.DateTimeFrom(timex.Now())

	if _, err := tx.NewUpdate().Model(task).WherePK().Select("status", "finished_at").Exec(ctx); err != nil {
		return nil, fmt.Errorf("finish task: %w", err)
	}

	actionType := approval.ActionApprove
	if status == approval.TaskRejected {
		actionType = approval.ActionReject
	}

	actionLog := &approval.ActionLog{
		InstanceID: task.InstanceID,
		NodeID:     null.StringFrom(task.NodeID),
		TaskID:     null.StringFrom(task.ID),
		Action:     actionType,
		OperatorID: "system",
		Opinion:    null.StringFrom("系统超时自动处理"),
	}
	actionLog.ID = id.Generate()
	actionLog.CreatedBy = "system"

	if _, err := tx.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert action log: %w", err)
	}

	var events []approval.DomainEvent
	if status == approval.TaskApproved {
		events = append(events, approval.NewTaskApprovedEvent(task.ID, task.InstanceID, node.ID, "system", "系统超时自动处理"))
	} else {
		events = append(events, approval.NewTaskRejectedEvent(task.ID, task.InstanceID, node.ID, "system", "系统超时自动处理"))
	}

	return events, nil
}

// transferToAdmin transfers a timed-out task to the node's admin users.
func (s *Scanner) transferToAdmin(ctx context.Context, tx orm.DB, task *approval.Task, node *approval.FlowNode) ([]approval.DomainEvent, error) {
	if len(node.AdminUserIDs) == 0 {
		return nil, nil
	}

	// Finish the original task as transferred
	task.Status = approval.TaskTransferred
	task.FinishedAt = null.DateTimeFrom(timex.Now())

	if _, err := tx.NewUpdate().Model(task).WherePK().Select("status", "finished_at").Exec(ctx); err != nil {
		return nil, fmt.Errorf("finish transferred task: %w", err)
	}

	var events []approval.DomainEvent

	events = append(events, approval.NewTaskTransferredEvent(
		task.ID, task.InstanceID, task.NodeID, task.AssigneeID, node.AdminUserIDs[0], "系统超时转交管理员",
	))

	// Create new tasks for admin users
	for _, adminID := range node.AdminUserIDs {
		newTask := &approval.Task{
			InstanceID: task.InstanceID,
			NodeID:     task.NodeID,
			AssigneeID: adminID,
			SortOrder:  0,
			Status:     approval.TaskPending,
		}
		newTask.ID = id.Generate()
		newTask.CreatedBy = "system"
		newTask.UpdatedBy = "system"

		if _, err := tx.NewInsert().Model(newTask).Exec(ctx); err != nil {
			return nil, fmt.Errorf("create admin task: %w", err)
		}

		events = append(events, approval.NewTaskCreatedEvent(newTask.ID, task.InstanceID, task.NodeID, adminID, nil))
	}

	// Log the action
	actionLog := &approval.ActionLog{
		InstanceID:   task.InstanceID,
		NodeID:       null.StringFrom(task.NodeID),
		TaskID:       null.StringFrom(task.ID),
		Action:       approval.ActionTransfer,
		OperatorID:   "system",
		TransferToID: null.StringFrom(node.AdminUserIDs[0]),
		Opinion:      null.StringFrom("系统超时转交管理员"),
	}
	actionLog.ID = id.Generate()
	actionLog.CreatedBy = "system"

	if _, err := tx.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert transfer action log: %w", err)
	}

	return events, nil
}

// preWarningRow holds data from the pre-warning scan query.
type preWarningRow struct {
	approval.Task
	TimeoutNotifyBeforeHours int `bun:"timeout_notify_before_hours"`
}

// ScanPreWarnings finds tasks approaching their deadline and sends warning notifications.
func (s *Scanner) ScanPreWarnings(ctx context.Context) {
	now := time.Now()

	var rows []preWarningRow

	err := s.db.NewRaw(`
		SELECT at.*, afn.timeout_notify_before_hours
		FROM apv_task AS at
		JOIN apv_flow_node AS afn ON afn.id = at.node_id
		WHERE at.status IN (?, ?)
		  AND at.deadline IS NOT NULL
		  AND at.is_timeout = ?
		  AND afn.timeout_notify_before_hours > 0
		  AND at.id NOT IN (
		    SELECT task_id FROM apv_timeout_notify WHERE notify_type = ?
		  )`,
		string(approval.TaskPending), string(approval.TaskWaiting),
		false, string(approval.TimeoutNotifyPreWarning),
	).Scan(ctx, &rows)
	if err != nil {
		logger.Error("Failed to scan pre-warning tasks: " + err.Error())
		return
	}

	for _, r := range rows {
		deadline := time.Time(r.Deadline.V)
		warningTime := deadline.Add(-time.Duration(r.TimeoutNotifyBeforeHours) * time.Hour)

		if now.Before(warningTime) {
			continue
		}

		hoursLeft := int(time.Until(deadline).Hours())
		if hoursLeft < 0 {
			hoursLeft = 0
		}

		if err := s.sendPreWarning(ctx, &r.Task, hoursLeft); err != nil {
			logger.Error(fmt.Sprintf("Failed to send pre-warning for task %s: %s", r.ID, err.Error()))
		}
	}
}

// sendPreWarning sends a pre-deadline warning notification for a task.
func (s *Scanner) sendPreWarning(ctx context.Context, task *approval.Task, hoursLeft int) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		notify := &approval.TimeoutNotify{
			TaskID:     task.ID,
			NotifyType: approval.TimeoutNotifyPreWarning,
		}
		notify.ID = id.Generate()
		notify.CreatedBy = "system"

		if _, err := tx.NewInsert().Model(notify).Exec(ctx); err != nil {
			return fmt.Errorf("insert pre-warning notify: %w", err)
		}

		event := approval.NewTaskDeadlineWarningEvent(
			task.ID, task.InstanceID, task.NodeID, task.AssigneeID,
			time.Time(task.Deadline.V), hoursLeft,
		)

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{event})
	})
}
