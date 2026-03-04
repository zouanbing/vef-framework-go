package service_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &TaskServiceTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// TaskServiceTestSuite tests the TaskService.
type TaskServiceTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	svc     *service.TaskService
	fixture *SvcFixture
}

func (s *TaskServiceTestSuite) SetupSuite() {
	s.svc = service.NewTaskService()
	s.fixture = setupSvcFixture(s.T(), s.ctx, s.db)
}

func (s *TaskServiceTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.ActionLog)(nil),
		(*approval.Task)(nil),
		(*approval.Instance)(nil),
	)
}

func (s *TaskServiceTestSuite) TearDownSuite() {
	cleanAllServiceData(s.ctx, s.db)
}

// --- FinishTask ---

func (s *TaskServiceTestSuite) TestFinishTask() {
	s.Run("ValidTransition", func() {
		task := insertTask(s.T(), s.ctx, s.db, s.fixture, approval.TaskPending)
		err := s.svc.FinishTask(s.ctx, s.db, task, approval.TaskApproved)
		s.Require().NoError(err, "Should finish task")

		s.Assert().Equal(approval.TaskApproved, task.Status, "Should update status in memory")
		s.Assert().NotNil(task.FinishedAt, "Should set FinishedAt")

		// Verify DB
		var dbTask approval.Task
		dbTask.ID = task.ID
		s.Require().NoError(s.db.NewSelect().Model(&dbTask).WherePK().Scan(s.ctx))
		s.Assert().Equal(approval.TaskApproved, dbTask.Status, "DB should reflect new status")
		s.Assert().NotNil(dbTask.FinishedAt, "DB should have FinishedAt")
	})

	s.Run("InvalidTransition", func() {
		task := insertTask(s.T(), s.ctx, s.db, s.fixture, approval.TaskApproved)
		err := s.svc.FinishTask(s.ctx, s.db, task, approval.TaskPending)
		s.Assert().ErrorIs(err, shared.ErrInvalidTaskTransition, "Should reject invalid transition")
	})
}

// --- CancelRemainingTasks ---

func (s *TaskServiceTestSuite) TestCancelRemainingTasks() {
	s.Run("CancelsPendingAndWaiting", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 2)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskApproved, 3)

		err := s.svc.CancelRemainingTasks(s.ctx, s.db, inst.ID, nodeID)
		s.Require().NoError(err)

		var tasks []approval.Task
		s.Require().NoError(s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", inst.ID).Equals("node_id", nodeID)
			}).
			OrderBy("sort_order").
			Scan(s.ctx))

		s.Assert().Equal(approval.TaskCanceled, tasks[0].Status, "Pending should be canceled")
		s.Assert().Equal(approval.TaskCanceled, tasks[1].Status, "Waiting should be canceled")
		s.Assert().Equal(approval.TaskApproved, tasks[2].Status, "Approved should remain unchanged")
	})
}

// --- CancelInstanceTasks ---

func (s *TaskServiceTestSuite) TestCancelInstanceTasks() {
	s.Run("CancelsAllPendingWaitingForInstance", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskPending, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[1], approval.TaskWaiting, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskRejected, 2)

		err := s.svc.CancelInstanceTasks(s.ctx, s.db, inst.ID)
		s.Require().NoError(err)

		var tasks []approval.Task
		s.Require().NoError(s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
			Scan(s.ctx))

		canceledCount := 0
		for _, task := range tasks {
			if task.Status == approval.TaskCanceled {
				canceledCount++
			}
		}
		s.Assert().Equal(2, canceledCount, "Should cancel 2 tasks")
	})
}

// --- ActivateNextSequentialTask ---

func (s *TaskServiceTestSuite) TestActivateNextSequentialTask() {
	s.Run("ActivatesNextWaitingTask", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 2)

		instance := &approval.Instance{}
		instance.ID = inst.ID
		node := &approval.FlowNode{}
		node.ID = nodeID

		err := s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Require().NoError(err)

		var tasks []approval.Task
		s.Require().NoError(s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", inst.ID).Equals("node_id", nodeID)
			}).
			OrderBy("sort_order").
			Scan(s.ctx))

		s.Assert().Equal(approval.TaskPending, tasks[0].Status, "First waiting task should become pending")
		s.Assert().Equal(approval.TaskWaiting, tasks[1].Status, "Second waiting task should remain waiting")
	})

	s.Run("NoWaitingTasks", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)

		instance := &approval.Instance{}
		instance.ID = inst.ID
		node := &approval.FlowNode{}
		node.ID = s.fixture.NodeIDs[1]

		err := s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Assert().NoError(err, "Should not error when no waiting tasks exist")
	})
}

// --- PrepareOperation ---

func (s *TaskServiceTestSuite) TestPrepareOperation() {
	s.Run("Success", func() {
		nodeID, instanceID, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskPending, "op-user-1")

		tc, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, "op-user-1", nil)
		s.Require().NoError(err)
		s.Assert().Equal(instanceID, tc.Instance.ID)
		s.Assert().Equal(taskID, tc.Task.ID)
		s.Assert().Equal(nodeID, tc.Node.ID)
	})

	s.Run("TaskNotFound", func() {
		_, err := s.svc.PrepareOperation(s.ctx, s.db, "non-existent", "op-user-1", nil)
		s.Assert().ErrorIs(err, shared.ErrTaskNotFound)
	})

	s.Run("InstanceCompleted", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceApproved, approval.TaskPending, "op-user-2")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, "op-user-2", nil)
		s.Assert().ErrorIs(err, shared.ErrInstanceCompleted)
	})

	s.Run("NotAssignee", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskPending, "op-user-3")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, "wrong-user", nil)
		s.Assert().ErrorIs(err, shared.ErrNotAssignee)
	})

	s.Run("TaskNotPending", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskApproved, "op-user-4")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, "op-user-4", nil)
		s.Assert().ErrorIs(err, shared.ErrTaskNotPending)
	})
}

// --- InsertActionLog ---

func (s *TaskServiceTestSuite) TestInsertActionLog() {
	s.Run("WithAllFields", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		task := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskPending, 1)
		operator := approval.OperatorInfo{ID: "log-user-1", Name: "Logger"}

		err := s.svc.InsertActionLog(s.ctx, s.db, inst.ID, task, operator, approval.ActionApprove, "looks good", "transfer-to-1", "rollback-node-1")
		s.Require().NoError(err)

		var log approval.ActionLog
		s.Require().NoError(s.db.NewSelect().
			Model(&log).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
			Scan(s.ctx))

		s.Assert().Equal(approval.ActionApprove, log.Action)
		s.Assert().Equal("log-user-1", log.OperatorID)
		s.Assert().NotNil(log.Opinion)
		s.Assert().Equal("looks good", *log.Opinion)
		s.Assert().NotNil(log.TransferToID)
		s.Assert().Equal("transfer-to-1", *log.TransferToID)
		s.Assert().NotNil(log.RollbackToNodeID)
		s.Assert().Equal("rollback-node-1", *log.RollbackToNodeID)
	})

	s.Run("WithEmptyOptionalFields", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		task := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[1], approval.TaskPending, 1)
		operator := approval.OperatorInfo{ID: "log-user-2", Name: "Logger2"}

		err := s.svc.InsertActionLog(s.ctx, s.db, inst.ID, task, operator, approval.ActionSubmit, "", "", "")
		s.Require().NoError(err)

		var log approval.ActionLog
		s.Require().NoError(s.db.NewSelect().
			Model(&log).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
			Scan(s.ctx))

		s.Assert().Nil(log.Opinion, "Should not set opinion when empty")
		s.Assert().Nil(log.TransferToID, "Should not set transfer_to_id when empty")
		s.Assert().Nil(log.RollbackToNodeID, "Should not set rollback_to_node_id when empty")
	})
}

// --- IsAuthorizedForNodeOperation ---

func (s *TaskServiceTestSuite) TestIsAuthorizedForNodeOperation() {
	s.Run("PeerAssignee", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 1)
		peerTask := insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 2, "peer-user")

		result := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, *peerTask, "peer-user")
		s.Assert().True(result, "Peer assignee should be authorized")
	})

	s.Run("FlowAdmin", func() {
		// Update fixture flow with admin users
		_, err := s.db.NewUpdate().
			Model((*approval.Flow)(nil)).
			Set("admin_user_ids", []string{"admin-user"}).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("id", s.fixture.FlowID) }).
			Exec(s.ctx)
		s.Require().NoError(err)

		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)

		task := approval.Task{
			InstanceID: inst.ID,
			NodeID:     s.fixture.NodeIDs[1],
			AssigneeID: "other-user",
			Status:     approval.TaskPending,
		}

		result := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, task, "admin-user")
		s.Assert().True(result, "Flow admin should be authorized")
	})

	s.Run("Unauthorized", func() {
		task := approval.Task{
			InstanceID: "non-existent",
			NodeID:     "non-existent-node",
			AssigneeID: "other",
			Status:     approval.TaskPending,
		}

		result := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, task, "random-user")
		s.Assert().False(result, "Random user should not be authorized")
	})
}

// --- CanRemoveAssigneeTask ---

func (s *TaskServiceTestSuite) TestCanRemoveAssigneeTask() {
	s.Run("HasOtherActionableTasks", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		task1 := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 2)

		node := &approval.FlowNode{PassRule: approval.PassAll}
		node.ID = nodeID
		canRemove, err := s.svc.CanRemoveAssigneeTask(s.ctx, s.db, engine.NewFlowEngine(nil, nil, nil), node, *task1)
		s.Require().NoError(err)
		s.Assert().True(canRemove, "Should allow removal when other actionable tasks exist")
	})
}
