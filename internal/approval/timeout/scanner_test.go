package timeout

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/internal/database"
	internalORM "github.com/ilxqx/vef-framework-go/internal/orm"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

var allModels = []any{
	(*approval.Flow)(nil),
	(*approval.FlowVersion)(nil),
	(*approval.FlowNode)(nil),
	(*approval.FlowNodeAssignee)(nil),
	(*approval.FlowEdge)(nil),
	(*approval.Instance)(nil),
	(*approval.Task)(nil),
	(*approval.ActionLog)(nil),
	(*approval.FormSnapshot)(nil),
	(*approval.EventOutbox)(nil),
	(*approval.CCRecord)(nil),
	(*approval.Delegation)(nil),
	(*approval.ParallelRecord)(nil),
	(*approval.FlowCategory)(nil),
	(*approval.FlowInitiator)(nil),
	(*approval.FlowNodeCC)(nil),
	(*approval.FlowFormField)(nil),
	(*approval.UrgeRecord)(nil),
	(*approval.TimeoutNotify)(nil),
}

func setupTestDB(t *testing.T) (orm.DB, func()) {
	t.Helper()

	dsConfig := &config.DataSourceConfig{Kind: config.SQLite}
	bunDB, err := database.New(dsConfig)
	require.NoError(t, err, "Should create database")

	bunDB.RegisterModel(allModels...)

	ctx := context.Background()
	for _, m := range allModels {
		_, err := bunDB.NewCreateTable().Model(m).IfNotExists().Exec(ctx)
		require.NoError(t, err, "Should create table")
	}

	db := internalORM.New(bunDB)
	return db, func() { _ = bunDB.Close() }
}

// insertNode inserts a FlowNode for testing.
func insertNode(t *testing.T, ctx context.Context, db orm.DB, versionID string, action approval.TimeoutAction, adminIDs []string) *approval.FlowNode {
	t.Helper()

	node := &approval.FlowNode{
		FlowVersionID: versionID,
		NodeKey:       "approval1",
		NodeKind:      approval.NodeApproval,
		Name:          "Approval",
		TimeoutAction: action,
		AdminUserIDs:  adminIDs,
	}
	_, err := db.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err, "Should insert node")
	return node
}

// insertTask inserts a Task for testing.
func insertTask(t *testing.T, ctx context.Context, db orm.DB, instanceID, nodeID, assigneeID string, deadline *timex.DateTime) *approval.Task {
	t.Helper()

	task := &approval.Task{
		InstanceID: instanceID,
		NodeID:     nodeID,
		AssigneeID: assigneeID,
		Status:     approval.TaskPending,
		Deadline:   deadline,
	}
	_, err := db.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err, "Should insert task")
	return task
}

// TestScanTimeouts_NoTimedOutTasks verifies no processing when no tasks are timed out.
func TestScanTimeouts_NoTimedOutTasks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	ctx := context.Background()
	scanner.ScanTimeouts(ctx) // Should not panic
}

// TestScanTimeouts_MarkTimeout verifies that timed-out tasks are marked as is_timeout=true.
func TestScanTimeouts_MarkTimeout(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	// Create a node with no timeout action
	node := insertNode(t, ctx, db, "v1", approval.TimeoutActionNone, nil)

	// Create a task with deadline in the past
	pastDeadline := new(timex.DateTime(time.Now().Add(-1 * time.Hour)))
	task := insertTask(t, ctx, db, "inst1", node.ID, "user1", pastDeadline)

	scanner.ScanTimeouts(ctx)

	// Verify task is marked as timed out
	var updated approval.Task
	err := db.NewSelect().Model(&updated).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query updated task")
	assert.True(t, updated.IsTimeout, "Task should be marked as timed out")

	// Verify timeout notify record was created
	var notifies []approval.TimeoutNotify
	err = db.NewSelect().Model(&notifies).Where(func(c orm.ConditionBuilder) {
		c.Equals("task_id", task.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query timeout notifies")
	assert.Len(t, notifies, 1, "Should have one timeout notify")
	assert.Equal(t, approval.TimeoutNotifyTimeout, notifies[0].NotifyType, "Notify type should be timeout")
}

// TestScanTimeouts_AutoPass verifies auto-pass action on timeout.
func TestScanTimeouts_AutoPass(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	node := insertNode(t, ctx, db, "v1", approval.TimeoutActionAutoPass, nil)
	pastDeadline := new(timex.DateTime(time.Now().Add(-1 * time.Hour)))
	task := insertTask(t, ctx, db, "inst1", node.ID, "user1", pastDeadline)

	scanner.ScanTimeouts(ctx)

	// Verify task is approved
	var updated approval.Task
	err := db.NewSelect().Model(&updated).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query updated task")
	assert.Equal(t, approval.TaskApproved, updated.Status, "Task should be auto-approved")
	assert.True(t, updated.FinishedAt != nil, "FinishedAt should be set")

	// Verify action log
	var logs []approval.ActionLog
	err = db.NewSelect().Model(&logs).Where(func(c orm.ConditionBuilder) {
		c.Equals("task_id", task.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query action logs")
	assert.Len(t, logs, 1, "Should have one action log")
	assert.Equal(t, approval.ActionApprove, logs[0].Action, "Action should be approve")
	assert.Equal(t, "system", logs[0].OperatorID, "Operator should be system")
}

// TestScanTimeouts_AutoReject verifies auto-reject action on timeout.
func TestScanTimeouts_AutoReject(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	node := insertNode(t, ctx, db, "v1", approval.TimeoutActionAutoReject, nil)
	pastDeadline := new(timex.DateTime(time.Now().Add(-1 * time.Hour)))
	task := insertTask(t, ctx, db, "inst1", node.ID, "user1", pastDeadline)

	scanner.ScanTimeouts(ctx)

	var updated approval.Task
	err := db.NewSelect().Model(&updated).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query updated task")
	assert.Equal(t, approval.TaskRejected, updated.Status, "Task should be auto-rejected")
}

// TestScanTimeouts_TransferAdmin verifies transfer-to-admin action on timeout.
func TestScanTimeouts_TransferAdmin(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	adminIDs := []string{"admin1", "admin2"}
	node := insertNode(t, ctx, db, "v1", approval.TimeoutActionTransferAdmin, adminIDs)
	pastDeadline := new(timex.DateTime(time.Now().Add(-1 * time.Hour)))
	task := insertTask(t, ctx, db, "inst1", node.ID, "user1", pastDeadline)

	scanner.ScanTimeouts(ctx)

	// Verify original task is transferred
	var original approval.Task
	err := db.NewSelect().Model(&original).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query original task")
	assert.Equal(t, approval.TaskTransferred, original.Status, "Original task should be transferred")

	// Verify admin tasks were created
	var adminTasks []approval.Task
	err = db.NewSelect().Model(&adminTasks).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", "inst1")
		c.NotEquals("id", task.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query admin tasks")
	assert.Len(t, adminTasks, 2, "Should create tasks for each admin")

	assignees := make([]string, len(adminTasks))
	for i, t := range adminTasks {
		assignees[i] = t.AssigneeID
	}
	assert.Contains(t, assignees, "admin1", "Admin1 should have a task")
	assert.Contains(t, assignees, "admin2", "Admin2 should have a task")
}

// TestScanTimeouts_SkipsAlreadyTimedOut verifies already-timed-out tasks are not processed again.
func TestScanTimeouts_SkipsAlreadyTimedOut(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	node := insertNode(t, ctx, db, "v1", approval.TimeoutActionAutoPass, nil)
	pastDeadline := new(timex.DateTime(time.Now().Add(-1 * time.Hour)))

	// Create a task already marked as timed out
	task := &approval.Task{
		InstanceID: "inst1",
		NodeID:     node.ID,
		AssigneeID: "user1",
		Status:     approval.TaskPending,
		Deadline:   pastDeadline,
		IsTimeout:  true,
	}
	_, err := db.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err, "Should insert task")

	scanner.ScanTimeouts(ctx)

	// Verify no action log was created (task was already timed out)
	var logs []approval.ActionLog
	err = db.NewSelect().Model(&logs).Scan(ctx)
	require.NoError(t, err, "Should query action logs")
	assert.Empty(t, logs, "No action logs should be created for already timed-out tasks")
}

// TestScanTimeouts_SkipsFutureDeadline verifies tasks with future deadlines are not processed.
func TestScanTimeouts_SkipsFutureDeadline(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	node := insertNode(t, ctx, db, "v1", approval.TimeoutActionAutoPass, nil)
	futureDeadline := new(timex.DateTime(time.Now().Add(1 * time.Hour)))
	insertTask(t, ctx, db, "inst1", node.ID, "user1", futureDeadline)

	scanner.ScanTimeouts(ctx)

	// Verify no timeout notifies
	var notifies []approval.TimeoutNotify
	err := db.NewSelect().Model(&notifies).Scan(ctx)
	require.NoError(t, err, "Should query timeout notifies")
	assert.Empty(t, notifies, "No timeout notifies should be created for future deadlines")
}

// TestSendPreWarning verifies pre-warning notification creation.
func TestSendPreWarning(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	task := &approval.Task{
		InstanceID: "inst1",
		NodeID:     "node1",
		AssigneeID: "user1",
		Status:     approval.TaskPending,
		Deadline:   new(timex.DateTime(time.Now().Add(2 * time.Hour))),
	}
	_, err := db.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err, "Should insert task")

	err = scanner.sendPreWarning(ctx, task, 2)
	require.NoError(t, err, "Should send pre-warning")

	// Verify pre-warning notify was created
	var notifies []approval.TimeoutNotify
	err = db.NewSelect().Model(&notifies).Where(func(c orm.ConditionBuilder) {
		c.Equals("task_id", task.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query timeout notifies")
	assert.Len(t, notifies, 1, "Should have one pre-warning notify")
	assert.Equal(t, approval.TimeoutNotifyPreWarning, notifies[0].NotifyType, "Notify type should be pre_warning")

	// Verify event was published to outbox
	var events []approval.EventOutbox
	err = db.NewSelect().Model(&events).Scan(ctx)
	require.NoError(t, err, "Should query events")
	assert.Len(t, events, 1, "Should have one event")
	assert.Equal(t, "approval.task.deadline_warning", events[0].EventType, "Event type should be deadline_warning")
}

// TestScanPreWarnings verifies the pre-warning scan with joined query.
func TestScanPreWarnings(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	// Create a flow version
	version := &approval.FlowVersion{
		FlowID:  "flow1",
		Version: 1,
		Status:  approval.VersionPublished,
	}
	_, err := db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err, "Should insert version")

	// Create a node with pre-warning hours
	node := &approval.FlowNode{
		FlowVersionID:           version.ID,
		NodeKey:                 "approval1",
		NodeKind:                approval.NodeApproval,
		Name:                    "Approval",
		TimeoutNotifyBeforeHours: 4,
	}
	_, err = db.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err, "Should insert node")

	// Create a task with deadline in 2 hours (within the 4-hour warning window)
	deadline := time.Now().Add(2 * time.Hour)
	task := insertTask(t, ctx, db, "inst1", node.ID, "user1", new(timex.DateTime(deadline)))

	scanner.ScanPreWarnings(ctx)

	// Verify pre-warning notify was created
	var notifies []approval.TimeoutNotify
	err = db.NewSelect().Model(&notifies).Where(func(c orm.ConditionBuilder) {
		c.Equals("task_id", task.ID)
		c.Equals("notify_type", string(approval.TimeoutNotifyPreWarning))
	}).Scan(ctx)
	require.NoError(t, err, "Should query timeout notifies")
	assert.Len(t, notifies, 1, "Should have one pre-warning notify")
}

// TestScanPreWarnings_OutsideWindow verifies tasks outside the warning window are skipped.
func TestScanPreWarnings_OutsideWindow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pub := publisher.NewEventPublisher()
	scanner := NewScanner(db, pub)

	version := &approval.FlowVersion{
		FlowID:  "flow1",
		Version: 1,
		Status:  approval.VersionPublished,
	}
	_, err := db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err, "Should insert version")

	node := &approval.FlowNode{
		FlowVersionID:           version.ID,
		NodeKey:                 "approval1",
		NodeKind:                approval.NodeApproval,
		Name:                    "Approval",
		TimeoutNotifyBeforeHours: 2,
	}
	_, err = db.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err, "Should insert node")

	// Create a task with deadline in 24 hours (outside the 2-hour warning window)
	deadline := time.Now().Add(24 * time.Hour)
	insertTask(t, ctx, db, "inst1", node.ID, "user1", new(timex.DateTime(deadline)))

	scanner.ScanPreWarnings(ctx)

	// Verify no pre-warning notifies
	var notifies []approval.TimeoutNotify
	err = db.NewSelect().Model(&notifies).Scan(ctx)
	require.NoError(t, err, "Should query timeout notifies")
	assert.Empty(t, notifies, "No pre-warning should be sent outside the warning window")
}
