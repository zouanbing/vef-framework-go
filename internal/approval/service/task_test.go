package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/migration"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/orm"
)

// --- FinishTask ---

func TestFinishTask(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skip("Service tests only run on PostgreSQL")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind))
		fix := setupSvcFixture(t, env)
		svc := NewTaskService()

		t.Run("ValidTransition", func(t *testing.T) {
			task := insertTask(t, env, fix, approval.TaskPending)
			err := svc.FinishTask(env.Ctx, env.DB, task, approval.TaskApproved)
			require.NoError(t, err, "Should finish task")

			assert.Equal(t, approval.TaskApproved, task.Status, "Should update status in memory")
			assert.NotNil(t, task.FinishedAt, "Should set FinishedAt")

			// Verify DB
			var dbTask approval.Task
			dbTask.ID = task.ID
			require.NoError(t, env.DB.NewSelect().Model(&dbTask).WherePK().Scan(env.Ctx))
			assert.Equal(t, approval.TaskApproved, dbTask.Status, "DB should reflect new status")
			assert.NotNil(t, dbTask.FinishedAt, "DB should have FinishedAt")
		})

		t.Run("InvalidTransition", func(t *testing.T) {
			task := insertTask(t, env, fix, approval.TaskApproved)
			err := svc.FinishTask(env.Ctx, env.DB, task, approval.TaskPending)
			assert.ErrorIs(t, err, shared.ErrInvalidTaskTransition, "Should reject invalid transition")
		})
	})
}

// --- CancelRemainingTasks ---

func TestCancelRemainingTasks(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skip("Service tests only run on PostgreSQL")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind))
		fix := setupSvcFixture(t, env)
		svc := NewTaskService()

		t.Run("CancelsPendingAndWaiting", func(t *testing.T) {
			inst := fix.createInstance(t, env, approval.InstanceRunning)
			instID := inst.ID
			nodeID := fix.NodeIDs[0]
			insertTaskWithDetails(t, env, instID, nodeID, approval.TaskPending, 1)
			insertTaskWithDetails(t, env, instID, nodeID, approval.TaskWaiting, 2)
			insertTaskWithDetails(t, env, instID, nodeID, approval.TaskApproved, 3)

			err := svc.CancelRemainingTasks(env.Ctx, env.DB, instID, nodeID)
			require.NoError(t, err)

			var tasks []approval.Task
			require.NoError(t, env.DB.NewSelect().
				Model(&tasks).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("instance_id", instID).Equals("node_id", nodeID)
				}).
				OrderBy("sort_order").
				Scan(env.Ctx))

			assert.Equal(t, approval.TaskCanceled, tasks[0].Status, "Pending should be canceled")
			assert.Equal(t, approval.TaskCanceled, tasks[1].Status, "Waiting should be canceled")
			assert.Equal(t, approval.TaskApproved, tasks[2].Status, "Approved should remain unchanged")
		})
	})
}

// --- CancelInstanceTasks ---

func TestCancelInstanceTasks(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skip("Service tests only run on PostgreSQL")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind))
		fix := setupSvcFixture(t, env)
		svc := NewTaskService()

		t.Run("CancelsAllPendingWaitingForInstance", func(t *testing.T) {
			inst := fix.createInstance(t, env, approval.InstanceRunning)
			instID := inst.ID
			insertTaskWithDetails(t, env, instID, fix.NodeIDs[0], approval.TaskPending, 1)
			insertTaskWithDetails(t, env, instID, fix.NodeIDs[1], approval.TaskWaiting, 1)
			insertTaskWithDetails(t, env, instID, fix.NodeIDs[0], approval.TaskRejected, 2)

			err := svc.CancelInstanceTasks(env.Ctx, env.DB, instID)
			require.NoError(t, err)

			var tasks []approval.Task
			require.NoError(t, env.DB.NewSelect().
				Model(&tasks).
				Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instID) }).
				Scan(env.Ctx))

			canceledCount := 0
			for _, task := range tasks {
				if task.Status == approval.TaskCanceled {
					canceledCount++
				}
			}
			assert.Equal(t, 2, canceledCount, "Should cancel 2 tasks")
		})
	})
}

// --- ActivateNextSequentialTask ---

func TestActivateNextSequentialTask(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skip("Service tests only run on PostgreSQL")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind))
		fix := setupSvcFixture(t, env)
		svc := NewTaskService()

		t.Run("ActivatesNextWaitingTask", func(t *testing.T) {
			inst := fix.createInstance(t, env, approval.InstanceRunning)
			instID := inst.ID
			nodeID := fix.NodeIDs[0]
			insertTaskWithDetails(t, env, instID, nodeID, approval.TaskWaiting, 1)
			insertTaskWithDetails(t, env, instID, nodeID, approval.TaskWaiting, 2)

			instance := &approval.Instance{}
			instance.ID = instID
			node := &approval.FlowNode{}
			node.ID = nodeID

			err := svc.ActivateNextSequentialTask(env.Ctx, env.DB, instance, node)
			require.NoError(t, err)

			var tasks []approval.Task
			require.NoError(t, env.DB.NewSelect().
				Model(&tasks).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("instance_id", instID).Equals("node_id", nodeID)
				}).
				OrderBy("sort_order").
				Scan(env.Ctx))

			assert.Equal(t, approval.TaskPending, tasks[0].Status, "First waiting task should become pending")
			assert.Equal(t, approval.TaskWaiting, tasks[1].Status, "Second waiting task should remain waiting")
		})

		t.Run("NoWaitingTasks", func(t *testing.T) {
			inst2 := fix.createInstance(t, env, approval.InstanceRunning)

			instance := &approval.Instance{}
			instance.ID = inst2.ID
			node := &approval.FlowNode{}
			node.ID = fix.NodeIDs[1]

			err := svc.ActivateNextSequentialTask(env.Ctx, env.DB, instance, node)
			assert.NoError(t, err, "Should not error when no waiting tasks exist")
		})
	})
}

// --- PrepareOperation ---

func TestPrepareOperation(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skip("Service tests only run on PostgreSQL")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind))
		fix := setupSvcFixture(t, env)
		svc := NewTaskService()
		ctx := env.Ctx

		t.Run("Success", func(t *testing.T) {
			nodeID, instanceID, taskID := setupPrepareOperationData(t, env, fix, ctx, approval.InstanceRunning, approval.TaskPending, "op-user-1")

			tc, err := svc.PrepareOperation(ctx, env.DB, taskID, "op-user-1", nil)
			require.NoError(t, err)
			assert.Equal(t, instanceID, tc.Instance.ID)
			assert.Equal(t, taskID, tc.Task.ID)
			assert.Equal(t, nodeID, tc.Node.ID)
		})

		t.Run("TaskNotFound", func(t *testing.T) {
			_, err := svc.PrepareOperation(ctx, env.DB, "non-existent", "op-user-1", nil)
			assert.ErrorIs(t, err, shared.ErrTaskNotFound)
		})

		t.Run("InstanceCompleted", func(t *testing.T) {
			_, _, taskID := setupPrepareOperationData(t, env, fix, ctx, approval.InstanceApproved, approval.TaskPending, "op-user-2")

			_, err := svc.PrepareOperation(ctx, env.DB, taskID, "op-user-2", nil)
			assert.ErrorIs(t, err, shared.ErrInstanceCompleted)
		})

		t.Run("NotAssignee", func(t *testing.T) {
			_, _, taskID := setupPrepareOperationData(t, env, fix, ctx, approval.InstanceRunning, approval.TaskPending, "op-user-3")

			_, err := svc.PrepareOperation(ctx, env.DB, taskID, "wrong-user", nil)
			assert.ErrorIs(t, err, shared.ErrNotAssignee)
		})

		t.Run("TaskNotPending", func(t *testing.T) {
			_, _, taskID := setupPrepareOperationData(t, env, fix, ctx, approval.InstanceRunning, approval.TaskApproved, "op-user-4")

			_, err := svc.PrepareOperation(ctx, env.DB, taskID, "op-user-4", nil)
			assert.ErrorIs(t, err, shared.ErrTaskNotPending)
		})
	})
}

// --- InsertActionLog ---

func TestInsertActionLog(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skip("Service tests only run on PostgreSQL")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind))
		fix := setupSvcFixture(t, env)
		svc := NewTaskService()

		t.Run("WithAllFields", func(t *testing.T) {
			inst := fix.createInstance(t, env, approval.InstanceRunning)
			task := insertTaskWithDetails(t, env, inst.ID, fix.NodeIDs[0], approval.TaskPending, 1)
			operator := approval.OperatorInfo{ID: "log-user-1", Name: "Logger"}

			err := svc.InsertActionLog(env.Ctx, env.DB, inst.ID, task, operator, approval.ActionApprove, "looks good", "transfer-to-1", "rollback-node-1")
			require.NoError(t, err)

			var log approval.ActionLog
			require.NoError(t, env.DB.NewSelect().
				Model(&log).
				Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
				Scan(env.Ctx))

			assert.Equal(t, approval.ActionApprove, log.Action)
			assert.Equal(t, "log-user-1", log.OperatorID)
			assert.NotNil(t, log.Opinion)
			assert.Equal(t, "looks good", *log.Opinion)
			assert.NotNil(t, log.TransferToID)
			assert.Equal(t, "transfer-to-1", *log.TransferToID)
			assert.NotNil(t, log.RollbackToNodeID)
			assert.Equal(t, "rollback-node-1", *log.RollbackToNodeID)
		})

		t.Run("WithEmptyOptionalFields", func(t *testing.T) {
			inst2 := fix.createInstance(t, env, approval.InstanceRunning)
			task := insertTaskWithDetails(t, env, inst2.ID, fix.NodeIDs[1], approval.TaskPending, 1)
			operator := approval.OperatorInfo{ID: "log-user-2", Name: "Logger2"}

			err := svc.InsertActionLog(env.Ctx, env.DB, inst2.ID, task, operator, approval.ActionSubmit, "", "", "")
			require.NoError(t, err)

			var log approval.ActionLog
			require.NoError(t, env.DB.NewSelect().
				Model(&log).
				Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst2.ID) }).
				Scan(env.Ctx))

			assert.Nil(t, log.Opinion, "Should not set opinion when empty")
			assert.Nil(t, log.TransferToID, "Should not set transfer_to_id when empty")
			assert.Nil(t, log.RollbackToNodeID, "Should not set rollback_to_node_id when empty")
		})
	})
}

// --- IsAuthorizedForNodeOperation ---

func TestIsAuthorizedForNodeOperation(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skip("Service tests only run on PostgreSQL")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind))
		fix := setupSvcFixture(t, env)
		svc := NewTaskService()

		t.Run("PeerAssignee", func(t *testing.T) {
			inst := fix.createInstance(t, env, approval.InstanceRunning)
			instID := inst.ID
			nodeID := fix.NodeIDs[0]
			insertTaskWithDetails(t, env, instID, nodeID, approval.TaskPending, 1)
			peerTask := insertTaskWithDetailsAndAssignee(t, env, instID, nodeID, approval.TaskPending, 2, "peer-user")

			result := svc.IsAuthorizedForNodeOperation(env.Ctx, env.DB, *peerTask, "peer-user")
			assert.True(t, result, "Peer assignee should be authorized")
		})

		t.Run("FlowAdmin", func(t *testing.T) {
			// Update fixture flow with admin users
			_, err := env.DB.NewUpdate().
				Model((*approval.Flow)(nil)).
				Set("admin_user_ids", []string{"admin-user"}).
				Where(func(cb orm.ConditionBuilder) { cb.Equals("id", fix.FlowID) }).
				Exec(env.Ctx)
			require.NoError(t, err)

			inst := fix.createInstance(t, env, approval.InstanceRunning)

			task := approval.Task{
				InstanceID: inst.ID,
				NodeID:     fix.NodeIDs[1],
				AssigneeID: "other-user",
				Status:     approval.TaskPending,
			}

			result := svc.IsAuthorizedForNodeOperation(env.Ctx, env.DB, task, "admin-user")
			assert.True(t, result, "Flow admin should be authorized")
		})

		t.Run("Unauthorized", func(t *testing.T) {
			task := approval.Task{
				InstanceID: "non-existent",
				NodeID:     "non-existent-node",
				AssigneeID: "other",
				Status:     approval.TaskPending,
			}

			result := svc.IsAuthorizedForNodeOperation(env.Ctx, env.DB, task, "random-user")
			assert.False(t, result, "Random user should not be authorized")
		})
	})
}

// --- CanRemoveAssigneeTask ---

func TestCanRemoveAssigneeTask(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skip("Service tests only run on PostgreSQL")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind))
		fix := setupSvcFixture(t, env)
		svc := NewTaskService()
		eng := engine.NewFlowEngine(nil, nil, nil)

		t.Run("HasOtherActionableTasks", func(t *testing.T) {
			inst := fix.createInstance(t, env, approval.InstanceRunning)
			instID := inst.ID
			nodeID := fix.NodeIDs[0]
			task1 := insertTaskWithDetails(t, env, instID, nodeID, approval.TaskPending, 1)
			insertTaskWithDetails(t, env, instID, nodeID, approval.TaskPending, 2)

			node := &approval.FlowNode{PassRule: approval.PassAll}
			node.ID = nodeID
			canRemove, err := svc.CanRemoveAssigneeTask(env.Ctx, env.DB, eng, node, *task1)
			require.NoError(t, err)
			assert.True(t, canRemove, "Should allow removal when other actionable tasks exist")
		})
	})
}

// --- Test helpers ---

// SvcFixture holds IDs of records created to satisfy FK constraints.
type SvcFixture struct {
	CategoryID string
	FlowID     string
	VersionID  string
	NodeIDs    []string
	instNo     int
}

func setupSvcFixture(t *testing.T, env *testx.DBEnv) *SvcFixture {
	t.Helper()
	cat := &approval.FlowCategory{TenantID: "default", Code: "svc-test-cat", Name: "Svc Test Cat"}
	_, err := env.DB.NewInsert().Model(cat).Exec(env.Ctx)
	require.NoError(t, err)

	flow := &approval.Flow{
		TenantID: "default", CategoryID: cat.ID, Code: "svc-test-flow", Name: "Svc Test Flow",
		BindingMode: approval.BindingStandalone, IsAllInitiationAllowed: true, IsActive: true,
	}
	_, err = env.DB.NewInsert().Model(flow).Exec(env.Ctx)
	require.NoError(t, err)

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionPublished}
	_, err = env.DB.NewInsert().Model(version).Exec(env.Ctx)
	require.NoError(t, err)

	// Create several nodes for tests that need different nodes
	var nodeIDs []string
	for i := range 6 {
		node := &approval.FlowNode{
			FlowVersionID: version.ID, Key: "svc-node-" + string(rune('a'+i)),
			Kind: approval.NodeApproval, Name: "Svc Node",
		}
		_, err = env.DB.NewInsert().Model(node).Exec(env.Ctx)
		require.NoError(t, err)
		nodeIDs = append(nodeIDs, node.ID)
	}

	return &SvcFixture{CategoryID: cat.ID, FlowID: flow.ID, VersionID: version.ID, NodeIDs: nodeIDs}
}

func (f *SvcFixture) createInstance(t *testing.T, env *testx.DBEnv, status approval.InstanceStatus) *approval.Instance {
	t.Helper()
	f.instNo++
	inst := &approval.Instance{
		TenantID: "default", FlowID: f.FlowID, FlowVersionID: f.VersionID,
		Title: "Svc Test", InstanceNo: "SVC-" + string(rune('0'+f.instNo)),
		ApplicantID: "applicant", Status: status,
	}
	_, err := env.DB.NewInsert().Model(inst).Exec(env.Ctx)
	require.NoError(t, err)
	return inst
}

func insertTask(t *testing.T, env *testx.DBEnv, fix *SvcFixture, status approval.TaskStatus) *approval.Task {
	t.Helper()
	inst := fix.createInstance(t, env, approval.InstanceRunning)
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     fix.NodeIDs[0],
		AssigneeID: "user-svc-test",
		SortOrder:  1,
		Status:     status,
	}
	_, err := env.DB.NewInsert().Model(task).Exec(env.Ctx)
	require.NoError(t, err)
	return task
}

func insertTaskWithDetails(t *testing.T, env *testx.DBEnv, instanceID, nodeID string, status approval.TaskStatus, sortOrder int) *approval.Task {
	t.Helper()
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: instanceID,
		NodeID:     nodeID,
		AssigneeID: "user-default",
		SortOrder:  sortOrder,
		Status:     status,
	}
	_, err := env.DB.NewInsert().Model(task).Exec(env.Ctx)
	require.NoError(t, err)
	return task
}

func insertTaskWithDetailsAndAssignee(t *testing.T, env *testx.DBEnv, instanceID, nodeID string, status approval.TaskStatus, sortOrder int, assigneeID string) *approval.Task {
	t.Helper()
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: instanceID,
		NodeID:     nodeID,
		AssigneeID: assigneeID,
		SortOrder:  sortOrder,
		Status:     status,
	}
	_, err := env.DB.NewInsert().Model(task).Exec(env.Ctx)
	require.NoError(t, err)
	return task
}

func setupPrepareOperationData(t *testing.T, env *testx.DBEnv, fix *SvcFixture, ctx context.Context, instanceStatus approval.InstanceStatus, taskStatus approval.TaskStatus, assigneeID string) (nodeID, instanceID, taskID string) {
	t.Helper()

	node := &approval.FlowNode{
		FlowVersionID: fix.VersionID,
		Key:           "prep-node-" + assigneeID,
		Kind:          approval.NodeApproval,
		Name:          "Prep Node",
	}
	_, err := env.DB.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err)

	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "Prep Instance",
		InstanceNo:    "PREP-" + assigneeID,
		ApplicantID:   "applicant",
		Status:        instanceStatus,
	}
	_, err = env.DB.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: instance.ID,
		NodeID:     node.ID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     taskStatus,
	}
	_, err = env.DB.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err)

	return node.ID, instance.ID, task.ID
}
