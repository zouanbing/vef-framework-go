package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/null"
	"github.com/ilxqx/vef-framework-go/orm"
)

type InstanceServiceTestSuite struct {
	suite.Suite
	ctx       context.Context
	db        orm.DB
	eng       *engine.FlowEngine
	svc       *InstanceService
	flowSvc   *FlowService
	mockOrg   *MockOrganizationService
	mockUser  *MockUserService
	serialGen *MockSerialNoGenerator
	cleanup   func()
}

// TestInstanceServiceTestSuite tests instance service test suite scenarios.
func TestInstanceServiceTestSuite(t *testing.T) {
	suite.Run(t, new(InstanceServiceTestSuite))
}

func (s *InstanceServiceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.db, s.cleanup = setupTestDB(s.T())

	s.mockOrg = &MockOrganizationService{
		superiors:   map[string]struct{ id, name string }{"applicant1": {id: "superior1", name: "Superior"}},
		deptLeaders: map[string][]string{"dept1": {"leader1"}},
	}
	s.mockUser = &MockUserService{
		roleUsers: map[string][]string{"role_admin": {"admin1", "admin2"}},
	}
	s.serialGen = NewMockSerialNoGenerator()

	pub := publisher.NewEventPublisher()
	s.eng = setupEngine(s.mockOrg, s.mockUser, pub)
	s.svc = NewInstanceService(s.db, s.eng, s.serialGen, pub, s.mockUser)
	s.flowSvc = NewFlowService(s.db, pub)
}

func (s *InstanceServiceTestSuite) TearDownTest() {
	s.cleanup()
}

// ==================== P0: Core Flow ====================

func (s *InstanceServiceTestSuite) TestStartInstanceSuccess() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test Leave",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"reason": "vacation"},
	})
	s.Require().NoError(err, "Should not return error")
	s.Require().NotNil(instance, "Should not be nil")

	s.Equal(approval.InstanceRunning, instance.Status)
	s.Equal("simple_flow-0001", instance.SerialNo)
	s.Equal("applicant1", instance.ApplicantID)
	s.Equal(approvalNode.ID, instance.CurrentNodeID.String)

	// Verify tasks were created (sequential: first=pending, second=waiting)
	tasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")
	s.Equal("user1", tasks[0].AssigneeID)
	s.Equal(approval.TaskPending, tasks[0].Status)
	s.Equal("user2", tasks[1].AssigneeID)
	s.Equal(approval.TaskWaiting, tasks[1].Status)

	// Verify events
	events := queryEvents(s.T(), s.ctx, s.db)
	s.NotEmpty(events)
}

func (s *InstanceServiceTestSuite) TestSequentialApprovalHappyPath() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Sequential Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"amount": 1000},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	// User1 approves
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "approve",
		OperatorID: "user1",
		Opinion:    "OK",
	})
	s.Require().NoError(err, "Should not return error")

	// Verify first task approved, second activated
	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Equal(approval.TaskApproved, tasks[0].Status)
	s.Equal(approval.TaskPending, tasks[1].Status)

	// User2 approves
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[1].ID,
		Action:     "approve",
		OperatorID: "user2",
		Opinion:    "Approved",
	})
	s.Require().NoError(err, "Should not return error")

	// Verify instance is approved (end node reached)
	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceApproved, inst.Status)
}

func (s *InstanceServiceTestSuite) TestParallelApprovalAllPass() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Parallel Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// All 3 tasks should be pending
	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 3, "Length should match expected value")
	for _, task := range tasks {
		s.Equal(approval.TaskPending, task.Status)
	}

	// All 3 approve
	for _, task := range tasks {
		err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
			InstanceID: instance.ID,
			TaskID:     task.ID,
			Action:     "approve",
			OperatorID: task.AssigneeID,
		})
		s.Require().NoError(err, "Should not return error")
	}

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceApproved, inst.Status)
}

func (s *InstanceServiceTestSuite) TestRejectionOneRejectStrategy() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Rejection Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 3, "Length should match expected value")

	// User1 rejects -> under one_reject strategy, instance should be rejected
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "reject",
		OperatorID: tasks[0].AssigneeID,
		Opinion:    "No",
	})
	s.Require().NoError(err, "Should not return error")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceRejected, inst.Status)

	// Remaining tasks should be canceled
	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	for _, task := range tasks {
		if task.AssigneeID != tasks[0].AssigneeID {
			s.Equal(approval.TaskCanceled, task.Status)
		}
	}
}

func (s *InstanceServiceTestSuite) TestWithdraw() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Withdraw Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	err = s.svc.Withdraw(s.ctx, instance.ID, "applicant1", "Changed my mind")
	s.Require().NoError(err, "Should not return error")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceWithdrawn, inst.Status)
	s.True(inst.FinishedAt.Valid)

	// All tasks should be canceled
	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	for _, task := range tasks {
		s.Equal(approval.TaskCanceled, task.Status)
	}
}

// ==================== P1: Advanced Operations ====================

func (s *InstanceServiceTestSuite) TestTransfer() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Transfer Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	// Transfer first task from user1 to user3
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks[0].ID,
		Action:       "transfer",
		OperatorID:   "user1",
		TransferToID: "user3",
		Opinion:      "Please handle",
	})
	s.Require().NoError(err, "Should not return error")

	// Verify: original task transferred + new task for user3
	allTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(allTasks, 3, "Length should match expected value")

	transferredFound := false
	newTaskFound := false
	for _, task := range allTasks {
		if task.ID == tasks[0].ID {
			s.Equal(approval.TaskTransferred, task.Status)
			transferredFound = true
		}

		if task.AssigneeID == "user3" && task.Status == approval.TaskPending {
			newTaskFound = true
		}
	}
	s.True(transferredFound)
	s.True(newTaskFound)
}

func (s *InstanceServiceTestSuite) TestRollback() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Rollback Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"amount": 1000},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	// User1 approves
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "approve",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should not return error")

	// Reload tasks to get user2's pending task
	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	var user2Task string
	for _, t := range tasks {
		if t.AssigneeID == "user2" && t.Status == approval.TaskPending {
			user2Task = t.ID
		}
	}
	s.Require().NotEmpty(user2Task, "Should not be empty")

	// User2 rolls back to the same approval node
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       user2Task,
		Action:       "rollback",
		OperatorID:   "user2",
		TargetNodeID: approvalNode.ID,
	})
	s.Require().NoError(err, "Should not return error")

	// Instance should still be running with new tasks on the target node
	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceRunning, inst.Status)
}

func (s *InstanceServiceTestSuite) TestAddAssigneeBefore() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Add Before Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_before1"},
		AddType:    "before",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should not return error")

	// Verify: new task is pending, original task went to waiting
	allTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(allTasks, 3, "Length should match expected value")

	var newTask, originalTask bool
	for _, t := range allTasks {
		if t.AssigneeID == "user_before1" {
			s.Equal(approval.TaskPending, t.Status)
			newTask = true
		}

		if t.ID == tasks[0].ID {
			s.Equal(approval.TaskWaiting, t.Status)
			originalTask = true
		}
	}
	s.True(newTask)
	s.True(originalTask)
}

func (s *InstanceServiceTestSuite) TestAddAssigneeAfter() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Add After Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_after1"},
		AddType:    "after",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should not return error")

	allTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(allTasks, 3, "Length should match expected value")

	for _, t := range allTasks {
		if t.AssigneeID == "user_after1" {
			s.Equal(approval.TaskWaiting, t.Status)
		}
	}
}

func (s *InstanceServiceTestSuite) TestAddAssigneeParallel() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Add Parallel Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_parallel1"},
		AddType:    "parallel",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should not return error")

	allTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(allTasks, 3, "Length should match expected value")

	for _, t := range allTasks {
		if t.AssigneeID == "user_parallel1" {
			s.Equal(approval.TaskPending, t.Status)
		}
	}
}

func (s *InstanceServiceTestSuite) TestRemoveAssignee() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Remove Assignee Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	// Use a peer assignee (user2) as operator - they are on the same node
	err = s.svc.RemoveAssignee(s.ctx, tasks[0].ID, "user2")
	s.Require().NoError(err, "Should not return error")

	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	removedFound := false
	for _, t := range tasks {
		if t.AssigneeID == "user1" {
			s.Equal(approval.TaskRemoved, t.Status)
			removedFound = true
		}
	}
	s.True(removedFound)
}

func (s *InstanceServiceTestSuite) TestProcessTaskExecuteOnHandleNode() {
	_, _, _, handleNode, _ := buildHandleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "handle_flow",
		Title:       "Handle Execute Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, handleNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")
	targetTask := tasks[0]

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     targetTask.ID,
		Action:     "execute",
		OperatorID: targetTask.AssigneeID,
		Opinion:    "handled",
	})
	s.Require().NoError(err, "Should not return error")

	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, handleNode.ID)
	for _, task := range tasks {
		if task.ID == targetTask.ID {
			s.Equal(approval.TaskHandled, task.Status)
			continue
		}

		s.Equal(approval.TaskCanceled, task.Status)
	}

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceApproved, inst.Status)
}

func (s *InstanceServiceTestSuite) TestRemoveAssigneeLastActionableTaskBlocked() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Last Actionable Remove",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	// Make waiting task non-actionable, leaving only one actionable task.
	waitingTaskID := tasks[1].ID
	_, err = s.db.NewUpdate().Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Where(func(c orm.ConditionBuilder) {
			c.Equals("id", waitingTaskID)
		}).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Configure flow admin as operator.
	var flow approval.Flow
	err = s.db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("code", "simple_flow")
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	flow.AdminUserIDs = []string{"admin_operator"}
	_, err = s.db.NewUpdate().Model(&flow).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	err = s.svc.RemoveAssignee(s.ctx, tasks[0].ID, "admin_operator")
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrLastAssigneeRemoval)

	latestTasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.TaskPending, latestTasks[0].Status)
}

func (s *InstanceServiceTestSuite) TestRemoveAssigneeSequentialPromotesWaitingTask() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Sequential Remove Promotion",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	var pendingTask, waitingTask approval.Task
	for _, task := range tasks {
		switch task.Status {
		case approval.TaskPending:
			pendingTask = task
		case approval.TaskWaiting:
			waitingTask = task
		}
	}
	s.Require().NotEmpty(pendingTask.ID, "Should not be empty")
	s.Require().NotEmpty(waitingTask.ID, "Should not be empty")

	err = s.svc.RemoveAssignee(s.ctx, pendingTask.ID, waitingTask.AssigneeID)
	s.Require().NoError(err, "Should not return error")

	latestTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	for _, task := range latestTasks {
		if task.ID == pendingTask.ID {
			s.Equal(approval.TaskRemoved, task.Status)
			continue
		}

		if task.ID == waitingTask.ID {
			s.Equal(approval.TaskPending, task.Status)
		}
	}
}

func (s *InstanceServiceTestSuite) TestMultiStageApproval() {
	_, _, nodes := buildMultiStageFlow(s.T(), s.ctx, s.db)
	approval1 := nodes[1]
	approval2 := nodes[2]

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "multi_stage_flow",
		Title:       "Multi Stage Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Stage 1: user1 approves
	tasks1 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval1.ID)
	s.Require().Len(tasks1, 1, "Length should match expected value")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks1[0].ID,
		Action:     "approve",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should not return error")

	// Stage 2: user2 should have a pending task
	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Length should match expected value")
	s.Equal("user2", tasks2[0].AssigneeID)
	s.Equal(approval.TaskPending, tasks2[0].Status)

	// user2 approves -> instance complete
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks2[0].ID,
		Action:     "approve",
		OperatorID: "user2",
	})
	s.Require().NoError(err, "Should not return error")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceApproved, inst.Status)
}

// ==================== P2: Error Scenarios ====================

func (s *InstanceServiceTestSuite) TestStartInstanceFlowNotFound() {
	_, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "nonexistent",
		Title:       "Test",
		ApplicantID: "user1",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrFlowNotFound)
}

func (s *InstanceServiceTestSuite) TestStartInstanceFlowNotActive() {
	_, _, _, _, _ = buildSimpleFlow(s.T(), s.ctx, s.db)

	// Deactivate the flow
	_, err := s.db.NewUpdate().Model((*approval.Flow)(nil)).Set("is_active", false).Where(func(c orm.ConditionBuilder) {
		c.Equals("code", "simple_flow")
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	_, err = s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrFlowNotActive)
}

func (s *InstanceServiceTestSuite) TestStartInstanceNoPublishedVersion() {
	flow, version, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	_ = flow

	// Set version to draft
	_, err := s.db.NewUpdate().Model((*approval.FlowVersion)(nil)).Set("status", string(approval.VersionDraft)).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", version.ID)
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	_, err = s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrNoPublishedVersion)
}

func (s *InstanceServiceTestSuite) TestProcessTaskInstanceNotFound() {
	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: "nonexistent",
		TaskID:     "task1",
		Action:     "approve",
		OperatorID: "user1",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInstanceNotFound)
}

func (s *InstanceServiceTestSuite) TestProcessTaskInstanceCompleted() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Reject to complete the instance
	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "reject",
		OperatorID: tasks[0].AssigneeID,
	})
	s.Require().NoError(err, "Should not return error")

	// Try to process another task on the completed instance
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[1].ID,
		Action:     "approve",
		OperatorID: tasks[1].AssigneeID,
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInstanceCompleted)
}

func (s *InstanceServiceTestSuite) TestProcessTaskNotAssignee() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)

	// Try with wrong operator
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "approve",
		OperatorID: "wrong_user",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrNotAssignee)
}

func (s *InstanceServiceTestSuite) TestProcessTaskTransferNotAllowed() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)

	// Parallel flow node has IsTransferAllowed=false by default
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks[0].ID,
		Action:       "transfer",
		OperatorID:   tasks[0].AssigneeID,
		TransferToID: "other_user",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrTransferNotAllowed)
}

func (s *InstanceServiceTestSuite) TestProcessTaskRollbackNotAllowed() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)

	// Parallel flow node has IsRollbackAllowed=false by default
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks[0].ID,
		Action:       "rollback",
		OperatorID:   tasks[0].AssigneeID,
		TargetNodeID: "start",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrRollbackNotAllowed)
}

// ==================== P3: Event Verification ====================

func (s *InstanceServiceTestSuite) TestEventsStartInstance() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	_, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Event Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	events := queryEvents(s.T(), s.ctx, s.db)
	found := false
	for _, e := range events {
		if e.EventType == "approval.instance.created" {
			found = true

			break
		}
	}
	s.True(found, "InstanceCreatedEvent should be published")
}

func (s *InstanceServiceTestSuite) TestEventsStartInstanceCreatedBeforeCompletedOnAutoCompleteFlow() {
	buildAutoCompleteFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "auto_complete_flow",
		Title:       "Auto Complete Event Order",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	events := queryEvents(s.T(), s.ctx, s.db)
	createdIndex := -1
	completedIndex := -1
	for i, event := range events {
		switch event.EventType {
		case "approval.instance.created":
			if createdIndex == -1 {
				createdIndex = i
			}
		case "approval.instance.completed":
			if completedIndex == -1 {
				completedIndex = i
			}
		}
	}

	s.Require().NotEqual(-1, createdIndex, "approval.instance.created should be published")
	s.Require().NotEqual(-1, completedIndex, "approval.instance.completed should be published")
	s.Less(createdIndex, completedIndex, "instance created event must be before completed event")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceApproved, inst.Status)
}

func (s *InstanceServiceTestSuite) TestEventsWithdraw() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Withdraw Event Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	err = s.svc.Withdraw(s.ctx, instance.ID, "applicant1", "reason")
	s.Require().NoError(err, "Should not return error")

	events := queryEvents(s.T(), s.ctx, s.db)
	found := false
	for _, e := range events {
		if e.EventType == "approval.instance.withdrawn" {
			found = true

			break
		}
	}
	s.True(found, "InstanceWithdrawnEvent should be published")
}

// ==================== Regression: Bug Fixes ====================

func (s *InstanceServiceTestSuite) TestWithdrawNotApplicant() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Withdraw Not Applicant",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Another user tries to withdraw — should be rejected
	err = s.svc.Withdraw(s.ctx, instance.ID, "other_user", "I want to cancel it")
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrNotApplicant)

	// Instance should still be running
	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceRunning, inst.Status)
}

func (s *InstanceServiceTestSuite) TestTransferThenApprovePassAll() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Transfer PassAll Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	// Transfer first task from user1 to user3
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks[0].ID,
		Action:       "transfer",
		OperatorID:   "user1",
		TransferToID: "user3",
		Opinion:      "Delegate to user3",
	})
	s.Require().NoError(err, "Should not return error")

	// user3's new task should be pending
	allTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	var user3TaskID string
	for _, t := range allTasks {
		if t.AssigneeID == "user3" && t.Status == approval.TaskPending {
			user3TaskID = t.ID
		}
	}
	s.Require().NotEmpty(user3TaskID, "Should not be empty")

	// user3 approves (the transferred-to task)
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     user3TaskID,
		Action:     "approve",
		OperatorID: "user3",
	})
	s.Require().NoError(err, "Should not return error")

	// user2 approves (the second sequential task should now be pending)
	allTasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	var user2TaskID string
	for _, t := range allTasks {
		if t.AssigneeID == "user2" && t.Status == approval.TaskPending {
			user2TaskID = t.ID
		}
	}
	s.Require().NotEmpty(user2TaskID, "Should not be empty")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     user2TaskID,
		Action:     "approve",
		OperatorID: "user2",
	})
	s.Require().NoError(err, "Should not return error")

	// Instance should be approved — transferred tasks must NOT block PassAll
	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceApproved, inst.Status)
}

func (s *InstanceServiceTestSuite) TestAddCCManualNotAllowed() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	// Disable manual CC on the approval node
	_, err := s.db.NewUpdate().Model((*approval.FlowNode)(nil)).
		Set("is_manual_cc_allowed", false).
		Where(func(c orm.ConditionBuilder) {
			c.Equals("id", approvalNode.ID)
		}).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "CC Not Allowed Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Try to manually CC — should be rejected
	err = s.svc.AddCC(s.ctx, instance.ID, []string{"cc_user1"}, "applicant1")
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrManualCcNotAllowed)
}

func (s *InstanceServiceTestSuite) TestAddAssigneeInvalidType() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Invalid AddType Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Require().NotEmpty(tasks, "Should not be empty")

	// Use invalid AddType
	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_x"},
		AddType:    "invalid_type",
		OperatorID: "user1",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidAddAssigneeType)
}

func (s *InstanceServiceTestSuite) TestAddAssigneeTypeNotInNodeAllowedList() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	// Restrict node to only allow "after"
	_, err := s.db.NewUpdate().Model((*approval.FlowNode)(nil)).
		Set("add_assignee_types", `["after"]`).
		Where(func(c orm.ConditionBuilder) {
			c.Equals("id", approvalNode.ID)
		}).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Restricted AddType Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should not be empty")

	// Try "before" when only "after" is allowed
	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_x"},
		AddType:    "before",
		OperatorID: "user1",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidAddAssigneeType)
}

// ==================== Regression: Round 2 ====================

// TestStartInstanceNotAllowedInitiate verifies that when IsAllInitiateAllowed is false
// and the applicant is not in FlowInitiator, ErrNotAllowedInitiate is returned.
func (s *InstanceServiceTestSuite) TestStartInstanceNotAllowedInitiate() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	// Set flow to not allow all initiation
	flow.IsAllInitiateAllowed = false
	_, err := s.db.NewUpdate().Model(flow).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Add a flow initiator that does NOT include our applicant
	initiator := &approval.FlowInitiator{
		FlowID:        flow.ID,
		InitiatorKind: approval.InitiatorUser,
		InitiatorIDs:  []string{"Other_user"},
	}
	initiator.ID = id.Generate()
	_, err = s.db.NewInsert().Model(initiator).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Attempt to start instance as non-allowed user
	_, err = s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrNotAllowedInitiate)
}

// TestStartInstanceAllowedInitiateByUser verifies that a user listed in
// FlowInitiator can start the instance even when IsAllInitiateAllowed is false.
func (s *InstanceServiceTestSuite) TestStartInstanceAllowedInitiateByUser() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	flow.IsAllInitiateAllowed = false
	_, err := s.db.NewUpdate().Model(flow).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Add a flow initiator that includes our applicant
	initiator := &approval.FlowInitiator{
		FlowID:        flow.ID,
		InitiatorKind: approval.InitiatorUser,
		InitiatorIDs:  []string{"applicant1"},
	}
	initiator.ID = id.Generate()
	_, err = s.db.NewInsert().Model(initiator).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(approval.InstanceRunning, instance.Status)
}

// TestRemoveAssigneeNotAuthorized verifies that a user who is neither a peer
// assignee nor a flow admin cannot remove an assignee.
func (s *InstanceServiceTestSuite) TestRemoveAssigneeNotAuthorized() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Require().NotEmpty(tasks, "Should not be empty")

	// Try to remove assignee by a random user who is not a peer or admin
	err = s.svc.RemoveAssignee(s.ctx, tasks[0].ID, "random_user")
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrNotAssignee)
}

// TestDeployFlowAssigneesPersisted verifies that DeployFlow correctly
// inserts FlowNodeAssignee records into the database.
func (s *InstanceServiceTestSuite) TestDeployFlowAssigneesPersisted() {
	definition := `{
		"nodes": [
			{"id": "start1", "type": "start", "data": {"label": "Start"}},
			{"id": "approval1", "type": "approval", "data": {"label": "Approval", "passRule": "all", "assignees": [
				{"kind": "user", "ids": ["u1", "u2"], "sortOrder": 0},
				{"kind": "role", "ids": ["r1"], "sortOrder": 1}
			]}},
			{"id": "end1", "type": "end", "data": {"label": "End"}}
		],
		"edges": [
			{"source": "start1", "target": "approval1"},
			{"source": "approval1", "target": "end1"}
		]
	}`

	flow, err := s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "test_deploy",
		FlowName:   "Test Deploy Flow",
		CategoryID: id.Generate(),
		Definition: definition,
		OperatorID: "operator1",
	})
	s.Require().NoError(err, "Should not return error")

	// Find the version and approval node
	var version approval.FlowVersion
	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	var approvalNode approval.FlowNode
	err = s.db.NewSelect().Model(&approvalNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
		c.Equals("node_key", "approval1")
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Verify assignees were persisted
	var assignees []approval.FlowNodeAssignee
	err = s.db.NewSelect().Model(&assignees).Where(func(c orm.ConditionBuilder) {
		c.Equals("node_id", approvalNode.ID)
	}).OrderBy("sort_order").Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Len(assignees, 2)
	s.Equal(approval.AssigneeUser, assignees[0].AssigneeKind)
	s.Equal([]string{"u1", "u2"}, assignees[0].AssigneeIDs)
	s.Equal(approval.AssigneeRole, assignees[1].AssigneeKind)
	s.Equal([]string{"r1"}, assignees[1].AssigneeIDs)
}

// TestAddAssigneeNonAssigneeBlocked verifies that a user who is not the task
// assignee cannot add assignees (operator permission check).
func (s *InstanceServiceTestSuite) TestAddAssigneeNonAssigneeBlocked() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	tasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Require().NotEmpty(tasks, "Should not be empty")

	// Try to add assignee by a user who is NOT the task assignee
	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"new_user"},
		AddType:    "parallel",
		OperatorID: "not_the_assignee",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrNotAssignee)
}

func (s *InstanceServiceTestSuite) TestWithdrawNotRunning() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Withdraw first
	err = s.svc.Withdraw(s.ctx, instance.ID, "applicant1", "reason")
	s.Require().NoError(err, "Should not return error")

	// Try to withdraw again (withdrawn state doesn't allow withdraw)
	err = s.svc.Withdraw(s.ctx, instance.ID, "applicant1", "reason again")
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrWithdrawNotAllowed)
}

// ==================== Round 3 Regression Tests ====================

// TestDeployFlowConditionsStoredOnNodeBranches verifies that edge conditions
// are stored on the condition node's Branches field (not on edges).
func (s *InstanceServiceTestSuite) TestDeployFlowConditionsStoredOnNodeBranches() {
	definition := `{
		"nodes": [
			{"id": "start1", "type": "start", "data": {"label": "Start"}},
			{"id": "branch1", "type": "condition", "data": {"label": "Branch", "branches": [
				{"id": "b1", "label": "High Amount", "conditions": [{"type": "field", "subject": "amount", "operator": ">", "value": 1000}], "priority": 0},
				{"id": "b2", "label": "Default", "isDefault": true, "priority": 1}
			]}},
			{"id": "approval1", "type": "approval", "data": {"label": "High Amount", "passRule": "all", "assignees": [{"kind": "user", "ids": ["u1"], "sortOrder": 0}]}},
			{"id": "end1", "type": "end", "data": {"label": "End"}}
		],
		"edges": [
			{"source": "start1", "target": "branch1"},
			{"source": "branch1", "target": "approval1", "sourceHandle": "b1"},
			{"source": "branch1", "target": "end1", "sourceHandle": "b2"}
		]
	}`

	flow, err := s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "cond_flow",
		FlowName:   "Condition Flow",
		CategoryID: id.Generate(),
		Definition: definition,
		OperatorID: "operator1",
	})
	s.Require().NoError(err, "Should not return error")

	// Find the version
	var version approval.FlowVersion
	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Find the condition node
	var condNode approval.FlowNode
	err = s.db.NewSelect().Model(&condNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
		c.Equals("node_key", "branch1")
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Require().NotEmpty(condNode.Branches, "condition node should have branches")
	s.Len(condNode.Branches, 2, "should have two branches")

	// Verify the first branch has conditions
	s.Equal("b1", condNode.Branches[0].ID)
	s.Len(condNode.Branches[0].Conditions, 1, "first branch should have one condition")
	s.False(condNode.Branches[0].IsDefault, "first branch should not be default")

	// Verify the second branch is default
	s.Equal("b2", condNode.Branches[1].ID)
	s.True(condNode.Branches[1].IsDefault, "second branch should be default")
}

// TestRemoveAssigneeTriggersCompletion verifies that after removing an assignee,
// node completion is re-evaluated and the flow advances if conditions are met.
func (s *InstanceServiceTestSuite) TestRemoveAssigneeTriggersCompletion() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Remove Triggers Completion",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Sequential flow: user1 is pending (sortOrder=1), user2 is waiting (sortOrder=2)
	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	// Approve user1's task
	var user1Task approval.Task
	for _, t := range tasks {
		if t.AssigneeID == "user1" {
			user1Task = t
			break
		}
	}
	s.Require().NotEmpty(user1Task.ID, "Should not be empty")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     user1Task.ID,
		Action:     "approve",
		OperatorID: "user1",
		Opinion:    "OK",
	})
	s.Require().NoError(err, "Should not return error")

	// After user1 approves, user2 should now be pending (sequential advances)
	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	var user2Task approval.Task
	for _, t := range tasks {
		if t.AssigneeID == "user2" && t.Status == approval.TaskPending {
			user2Task = t
			break
		}
	}
	s.Require().NotEmpty(user2Task.ID, "user2 should have a pending task")

	// Now remove user2 (operator=user1 who is already an approved peer assignee)
	// After user1 approved, user1 is no longer pending, so we need a valid operator.
	// Use an admin-level check: set flow admin
	var flow approval.Flow
	err = s.db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("code", "simple_flow")
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	flow.AdminUserIDs = []string{"admin_operator"}
	_, err = s.db.NewUpdate().Model(&flow).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	err = s.svc.RemoveAssignee(s.ctx, user2Task.ID, "admin_operator")
	s.Require().NoError(err, "Should not return error")

	// After removing user2, PassAll should evaluate: 1 approved / 1 total → pass
	// Flow should advance to End and complete as approved
	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceApproved, inst.Status)
	s.True(inst.FinishedAt.Valid, "finished_at should be set")
}

// TestSubFlowParentRejectedStateConvergence verifies that when a sub-flow is rejected,
// the parent instance gets finished_at set and completion event is published.
func (s *InstanceServiceTestSuite) TestSubFlowParentRejectedStateConvergence() {
	// Build parent flow: Start -> SubFlow -> End
	parentFlow := &approval.Flow{
		TenantID:             "default",
		CategoryID:           id.Generate(),
		Code:                 "parent_flow",
		Name:                 "Parent Flow",
		IsActive:             true,
		IsAllInitiateAllowed: true,
		CurrentVersion:       1,
	}
	parentFlow.ID = id.Generate()
	parentFlow.CreatedBy = "system"
	parentFlow.UpdatedBy = "system"
	_, err := s.db.NewInsert().Model(parentFlow).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	parentVersion := &approval.FlowVersion{
		FlowID:  parentFlow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	parentVersion.ID = id.Generate()
	parentVersion.CreatedBy = "system"
	parentVersion.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(parentVersion).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	startNode := &approval.FlowNode{FlowVersionID: parentVersion.ID, NodeKey: "start", NodeKind: approval.NodeStart, Name: "Start"}
	startNode.ID = id.Generate()
	startNode.CreatedBy = "system"
	startNode.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(startNode).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Build child flow: Start -> Approval(user1) -> End
	childFlow := &approval.Flow{
		TenantID:             "default",
		CategoryID:           id.Generate(),
		Code:                 "child_flow",
		Name:                 "Child Flow",
		IsActive:             true,
		IsAllInitiateAllowed: true,
		CurrentVersion:       1,
	}
	childFlow.ID = id.Generate()
	childFlow.CreatedBy = "system"
	childFlow.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(childFlow).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	childVersion := &approval.FlowVersion{
		FlowID:  childFlow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	childVersion.ID = id.Generate()
	childVersion.CreatedBy = "system"
	childVersion.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(childVersion).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	childStart := &approval.FlowNode{FlowVersionID: childVersion.ID, NodeKey: "start", NodeKind: approval.NodeStart, Name: "Start"}
	childStart.ID = id.Generate()
	childStart.CreatedBy = "system"
	childStart.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(childStart).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	childApproval := &approval.FlowNode{
		FlowVersionID:          childVersion.ID,
		NodeKey:                "approval1",
		NodeKind:               approval.NodeApproval,
		Name:                   "Child Approval",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAll,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	childApproval.ID = id.Generate()
	childApproval.CreatedBy = "system"
	childApproval.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(childApproval).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	childEnd := &approval.FlowNode{FlowVersionID: childVersion.ID, NodeKey: "end", NodeKind: approval.NodeEnd, Name: "End"}
	childEnd.ID = id.Generate()
	childEnd.CreatedBy = "system"
	childEnd.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(childEnd).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	childAssignee := &approval.FlowNodeAssignee{
		NodeID:       childApproval.ID,
		AssigneeKind: approval.AssigneeUser,
		AssigneeIDs:  []string{"user1"},
		SortOrder:    0,
	}
	childAssignee.ID = id.Generate()
	_, err = s.db.NewInsert().Model(childAssignee).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	insertEdge(s.T(), s.ctx, s.db, childVersion.ID, childStart.ID, childApproval.ID)
	insertEdge(s.T(), s.ctx, s.db, childVersion.ID, childApproval.ID, childEnd.ID)

	// SubFlow node in parent
	subFlowNode := &approval.FlowNode{
		FlowVersionID: parentVersion.ID,
		NodeKey:       "subflow1",
		NodeKind:      approval.NodeSubFlow,
		Name:          "SubFlow",
		SubFlowConfig: map[string]any{"flowId": childFlow.ID},
	}
	subFlowNode.ID = id.Generate()
	subFlowNode.CreatedBy = "system"
	subFlowNode.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(subFlowNode).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	endNode := &approval.FlowNode{FlowVersionID: parentVersion.ID, NodeKey: "end", NodeKind: approval.NodeEnd, Name: "End"}
	endNode.ID = id.Generate()
	endNode.CreatedBy = "system"
	endNode.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(endNode).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	insertEdge(s.T(), s.ctx, s.db, parentVersion.ID, startNode.ID, subFlowNode.ID)
	insertEdge(s.T(), s.ctx, s.db, parentVersion.ID, subFlowNode.ID, endNode.ID)

	// Start parent flow
	parentInstance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parent_flow",
		Title:       "Parent Instance",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Find the child instance
	var childInstance approval.Instance
	err = s.db.NewSelect().Model(&childInstance).Where(func(c orm.ConditionBuilder) {
		c.Equals("parent_instance_id", parentInstance.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Equal(approval.InstanceRunning, childInstance.Status)

	// Find child's approval task
	childTasks := queryTasksByNode(s.T(), s.ctx, s.db, childInstance.ID, childApproval.ID)
	s.Require().Len(childTasks, 1, "Length should match expected value")

	// Reject the child task → child flow rejected → parent flow rejected
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: childInstance.ID,
		TaskID:     childTasks[0].ID,
		Action:     "reject",
		OperatorID: "user1",
		Opinion:    "Not approved",
	})
	s.Require().NoError(err, "Should not return error")

	// Verify child instance is rejected with finished_at
	childInst := queryInstance(s.T(), s.ctx, s.db, childInstance.ID)
	s.Equal(approval.InstanceRejected, childInst.Status)
	s.True(childInst.FinishedAt.Valid, "child finished_at should be set")

	// Verify parent instance is rejected with finished_at
	parentInst := queryInstance(s.T(), s.ctx, s.db, parentInstance.ID)
	s.Equal(approval.InstanceRejected, parentInst.Status)
	s.True(parentInst.FinishedAt.Valid, "parent finished_at should be set")

	// Verify sub-flow events were published
	events := queryEvents(s.T(), s.ctx, s.db)
	eventTypes := make(map[string]bool)
	for _, e := range events {
		eventTypes[e.EventType] = true
	}
	s.True(eventTypes["approval.subflow.started"], "SubFlowStartedEvent should be published")
	s.True(eventTypes["approval.subflow.completed"], "SubFlowCompletedEvent should be published")
}

// TestMainFlowApprovedPublishesCompletionEvent verifies that when a main flow
// (non-sub-flow) completes via approval, approval.instance.completed is published.
func (s *InstanceServiceTestSuite) TestMainFlowApprovedPublishesCompletionEvent() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Completion Event Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Sequential: approve user1, then user2
	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Length should match expected value")

	// Approve user1
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID, TaskID: tasks[0].ID,
		Action: "approve", OperatorID: "user1", Opinion: "OK",
	})
	s.Require().NoError(err, "Should not return error")

	// Approve user2
	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	var user2Task approval.Task
	for _, t := range tasks {
		if t.AssigneeID == "user2" && t.Status == approval.TaskPending {
			user2Task = t
			break
		}
	}
	s.Require().NotEmpty(user2Task.ID, "Should not be empty")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID, TaskID: user2Task.ID,
		Action: "approve", OperatorID: "user2", Opinion: "OK",
	})
	s.Require().NoError(err, "Should not return error")

	// Instance should be approved
	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceApproved, inst.Status)
	s.True(inst.FinishedAt.Valid)

	// Verify approval.instance.completed event was published
	events := queryEvents(s.T(), s.ctx, s.db)
	found := false
	for _, e := range events {
		if e.EventType == "approval.instance.completed" {
			found = true
			break
		}
	}
	s.True(found, "approval.instance.completed should be published for main flow approval")
}

// ==================== P1-2: Rollback Target Validation ====================

func (s *InstanceServiceTestSuite) TestRollbackPreviousTypeValidTarget() {
	// Build multi-stage: Start -> Approval1(user1) -> Approval2(user2) -> End
	flow, version, nodes := buildMultiStageFlow(s.T(), s.ctx, s.db)
	approval1 := nodes[1]
	approval2 := nodes[2]

	// Set Approval2 to RollbackPrevious
	approval2.IsRollbackAllowed = true
	approval2.RollbackType = approval.RollbackPrevious
	_, err := s.db.NewUpdate().Model(approval2).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	_ = flow
	_ = version

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode: "multi_stage_flow", Title: "Rollback Previous Test",
		ApplicantID: "applicant1", FormData: map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Approve at approval1 (user1)
	tasks1 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval1.ID)
	s.Require().Len(tasks1, 1, "Length should match expected value")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID, TaskID: tasks1[0].ID,
		Action: "approve", OperatorID: "user1",
	})
	s.Require().NoError(err, "Should not return error")

	// Now at approval2 (user2). Rollback to approval1 (previous node) should succeed.
	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Length should match expected value")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID, TaskID: tasks2[0].ID,
		Action: "rollback", OperatorID: "user2", TargetNodeID: approval1.ID,
	})
	s.Require().NoError(err, "Should not return error")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceRunning, inst.Status)
}

func (s *InstanceServiceTestSuite) TestRollbackPreviousTypeInvalidTarget() {
	_, _, nodes := buildMultiStageFlow(s.T(), s.ctx, s.db)
	startNode := nodes[0]
	approval2 := nodes[2]

	// Set Approval2 to RollbackPrevious
	approval2.IsRollbackAllowed = true
	approval2.RollbackType = approval.RollbackPrevious
	_, err := s.db.NewUpdate().Model(approval2).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode: "multi_stage_flow", Title: "Rollback Invalid Test",
		ApplicantID: "applicant1", FormData: map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Approve at approval1
	approval1 := nodes[1]
	tasks1 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval1.ID)
	s.Require().Len(tasks1, 1, "Length should match expected value")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID, TaskID: tasks1[0].ID,
		Action: "approve", OperatorID: "user1",
	})
	s.Require().NoError(err, "Should not return error")

	// At approval2, try to rollback to startNode (not the previous node, which is approval1)
	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Length should match expected value")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID, TaskID: tasks2[0].ID,
		Action: "rollback", OperatorID: "user2", TargetNodeID: startNode.ID,
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidRollbackTarget)
}

// ==================== P1-1: Sub-flow Cycle Detection ====================

func (s *InstanceServiceTestSuite) TestSubFlowCycleDetection() {
	// Create a flow with a sub-flow node that references itself
	flowA := &approval.Flow{
		TenantID: "default", CategoryID: id.Generate(),
		Code: "flow_cycle_a", Name: "Cycle A",
		IsActive: true, IsAllInitiateAllowed: true, CurrentVersion: 1,
	}
	flowA.ID = id.Generate()
	flowA.CreatedBy = "system"
	flowA.UpdatedBy = "system"
	_, err := s.db.NewInsert().Model(flowA).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	versionA := &approval.FlowVersion{
		FlowID: flowA.ID, Version: 1, Status: approval.VersionPublished,
	}
	versionA.ID = id.Generate()
	versionA.CreatedBy = "system"
	versionA.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(versionA).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	startNode := createNode(s.T(), s.ctx, s.db, versionA.ID, "start", approval.NodeStart, "Start", approval.ApprovalSequential, approval.PassAll)

	// Sub-flow node that references flowA itself (self-cycle)
	sfNode := &approval.FlowNode{
		FlowVersionID: versionA.ID,
		NodeKey:       "subflow1",
		NodeKind:      approval.NodeSubFlow,
		Name:          "Self SubFlow",
		SubFlowConfig: map[string]any{"flowId": flowA.ID},
	}
	sfNode.ID = id.Generate()
	sfNode.CreatedBy = "system"
	sfNode.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(sfNode).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	endNode := createNode(s.T(), s.ctx, s.db, versionA.ID, "end", approval.NodeEnd, "End", approval.ApprovalSequential, approval.PassAll)

	insertEdge(s.T(), s.ctx, s.db, versionA.ID, startNode.ID, sfNode.ID)
	insertEdge(s.T(), s.ctx, s.db, versionA.ID, sfNode.ID, endNode.ID)

	// Start an instance - should fail with cycle detection
	_, err = s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode: "flow_cycle_a", Title: "Cycle Test",
		ApplicantID: "applicant1", FormData: map[string]any{},
	})
	s.Require().Error(err, "Should return error")
	s.Contains(err.Error(), "circular sub-flow")
}

// ==================== P1-5: Sub-flow Instance Audit Fields ====================

func (s *InstanceServiceTestSuite) TestSubFlowInstanceHasAuditFields() {
	// Create parent flow A with sub-flow node pointing to flow B
	flowB := &approval.Flow{
		TenantID: "default", CategoryID: id.Generate(),
		Code: "flow_child_b", Name: "Child B",
		IsActive: true, IsAllInitiateAllowed: true, CurrentVersion: 1,
	}
	flowB.ID = id.Generate()
	flowB.CreatedBy = "system"
	flowB.UpdatedBy = "system"
	_, err := s.db.NewInsert().Model(flowB).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	versionB := &approval.FlowVersion{
		FlowID: flowB.ID, Version: 1, Status: approval.VersionPublished,
	}
	versionB.ID = id.Generate()
	versionB.CreatedBy = "system"
	versionB.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(versionB).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	bStart := createNode(s.T(), s.ctx, s.db, versionB.ID, "start", approval.NodeStart, "Start", approval.ApprovalSequential, approval.PassAll)
	bEnd := createNode(s.T(), s.ctx, s.db, versionB.ID, "end", approval.NodeEnd, "End", approval.ApprovalSequential, approval.PassAll)
	insertEdge(s.T(), s.ctx, s.db, versionB.ID, bStart.ID, bEnd.ID)

	// Create parent flow A
	flowA := &approval.Flow{
		TenantID: "default", CategoryID: id.Generate(),
		Code: "flow_parent_a", Name: "Parent A",
		IsActive: true, IsAllInitiateAllowed: true, CurrentVersion: 1,
	}
	flowA.ID = id.Generate()
	flowA.CreatedBy = "system"
	flowA.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(flowA).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	versionA := &approval.FlowVersion{
		FlowID: flowA.ID, Version: 1, Status: approval.VersionPublished,
	}
	versionA.ID = id.Generate()
	versionA.CreatedBy = "system"
	versionA.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(versionA).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	aStart := createNode(s.T(), s.ctx, s.db, versionA.ID, "start", approval.NodeStart, "Start", approval.ApprovalSequential, approval.PassAll)
	sfNode := &approval.FlowNode{
		FlowVersionID: versionA.ID,
		NodeKey:       "subflow1",
		NodeKind:      approval.NodeSubFlow,
		Name:          "SubFlow",
		SubFlowConfig: map[string]any{"flowId": flowB.ID},
	}
	sfNode.ID = id.Generate()
	sfNode.CreatedBy = "system"
	sfNode.UpdatedBy = "system"
	_, err = s.db.NewInsert().Model(sfNode).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	aEnd := createNode(s.T(), s.ctx, s.db, versionA.ID, "end", approval.NodeEnd, "End", approval.ApprovalSequential, approval.PassAll)
	insertEdge(s.T(), s.ctx, s.db, versionA.ID, aStart.ID, sfNode.ID)
	insertEdge(s.T(), s.ctx, s.db, versionA.ID, sfNode.ID, aEnd.ID)

	// Start parent flow
	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode: "flow_parent_a", Title: "Audit Fields Test",
		ApplicantID: "applicant1", FormData: map[string]any{},
	})
	s.Require().NoError(err, "Should not return error")

	// Find the child instance
	var childInstance approval.Instance
	err = s.db.NewSelect().Model(&childInstance).Where(func(c orm.ConditionBuilder) {
		c.Equals("parent_instance_id", instance.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Verify key fields are set
	s.NotEmpty(childInstance.ID, "child instance ID should be set")
	s.NotEmpty(childInstance.CreatedBy, "child instance CreatedBy should not be empty")
	s.Equal(flowB.ID, childInstance.FlowID)
	s.True(childInstance.ParentInstanceID.Valid)
	s.Equal(instance.ID, childInstance.ParentInstanceID.String)
}

// TestFilterEditableFormData tests filter editable form data scenarios.
func TestFilterEditableFormData(t *testing.T) {
	t.Run("EmptyPermissionsReturnsAllFields", func(t *testing.T) {
		data := map[string]any{"name": "Alice", "age": 30}
		result := filterEditableFormData(data, nil)
		assert.Equal(t, data, result, "Nil permissions should return all fields unchanged")
	})

	t.Run("EditableFieldsAllowed", func(t *testing.T) {
		data := map[string]any{"name": "Alice", "age": 30, "dept": "IT"}
		perms := map[string]any{
			"name": "editable",
			"age":  "visible",
			"dept": "required",
		}
		result := filterEditableFormData(data, perms)
		assert.Equal(t, "Alice", result["name"], "Editable field should be included")
		assert.Equal(t, "IT", result["dept"], "Required field should be included")
		assert.NotContains(t, result, "age", "Visible fields should be filtered out")
	})

	t.Run("HiddenFieldsFiltered", func(t *testing.T) {
		data := map[string]any{"name": "Alice", "secret": "hidden_data"}
		perms := map[string]any{
			"name":   "editable",
			"secret": "hidden",
		}
		result := filterEditableFormData(data, perms)
		assert.Contains(t, result, "name", "Editable field should be present")
		assert.NotContains(t, result, "secret", "Hidden fields should be filtered out")
	})

	t.Run("FieldWithoutPermissionDenied", func(t *testing.T) {
		data := map[string]any{"name": "Alice", "extra": "value"}
		perms := map[string]any{"name": "editable"}
		result := filterEditableFormData(data, perms)
		assert.Contains(t, result, "name", "Editable field should be present")
		assert.NotContains(t, result, "extra", "Fields without permission config should be denied by default")
	})

	t.Run("NonStringPermissionExcluded", func(t *testing.T) {
		data := map[string]any{"name": "Alice"}
		perms := map[string]any{"name": 123}
		result := filterEditableFormData(data, perms)
		assert.NotContains(t, result, "name", "Non-string permission should result in exclusion")
	})
}

// TestValidateOpinion tests validate opinion scenarios.
func TestValidateOpinion(t *testing.T) {
	t.Run("RequiredAndEmpty", func(t *testing.T) {
		err := validateOpinion(&approval.FlowNode{IsOpinionRequired: true}, "")
		assert.ErrorIs(t, err, ErrOpinionRequired, "Empty opinion should fail when required")
	})

	t.Run("RequiredAndProvided", func(t *testing.T) {
		err := validateOpinion(&approval.FlowNode{IsOpinionRequired: true}, "I approve")
		assert.NoError(t, err, "Provided opinion should pass when required")
	})

	t.Run("NotRequired", func(t *testing.T) {
		err := validateOpinion(&approval.FlowNode{IsOpinionRequired: false}, "")
		assert.NoError(t, err, "Empty opinion should pass when not required")
	})
}

type InstanceServiceEdgeCaseTestSuite struct {
	suite.Suite
	ctx       context.Context
	db        orm.DB
	svc       *InstanceService
	flowSvc   *FlowService
	mockOrg   *MockOrganizationService
	mockUser  *MockUserService
	serialGen *MockSerialNoGenerator
	cleanup   func()
}

// TestInstanceServiceEdgeCaseTestSuite tests instance service edge case test suite scenarios.
func TestInstanceServiceEdgeCaseTestSuite(t *testing.T) {
	suite.Run(t, new(InstanceServiceEdgeCaseTestSuite))
}

func (s *InstanceServiceEdgeCaseTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.db, s.cleanup = setupTestDB(s.T())

	s.mockOrg = &MockOrganizationService{
		superiors:   map[string]struct{ id, name string }{"applicant1": {id: "superior1", name: "Superior"}},
		deptLeaders: map[string][]string{"dept1": {"leader1"}},
	}
	s.mockUser = &MockUserService{
		roleUsers: map[string][]string{"role_admin": {"admin1", "admin2"}},
	}
	s.serialGen = NewMockSerialNoGenerator()

	pub := publisher.NewEventPublisher()
	eng := setupEngine(s.mockOrg, s.mockUser, pub)
	s.svc = NewInstanceService(s.db, eng, s.serialGen, pub, s.mockUser)
	s.flowSvc = NewFlowService(s.db, pub)
}

func (s *InstanceServiceEdgeCaseTestSuite) TearDownTest() {
	s.cleanup()
}

// insertInitiator creates a FlowInitiator record for permission tests.
func (s *InstanceServiceEdgeCaseTestSuite) insertInitiator(flowID string, kind approval.InitiatorKind, ids []string) {
	s.T().Helper()

	initiator := &approval.FlowInitiator{
		FlowID:        flowID,
		InitiatorKind: kind,
		InitiatorIDs:  ids,
	}
	initiator.ID = id.Generate()

	_, err := s.db.NewInsert().Model(initiator).Exec(s.ctx)
	s.Require().NoError(err, "Should insert initiator")
}

// disableAllInitiate sets IsAllInitiateAllowed=false on the given flow.
func (s *InstanceServiceEdgeCaseTestSuite) disableAllInitiate(flow *approval.Flow) {
	s.T().Helper()

	flow.IsAllInitiateAllowed = false
	_, err := s.db.NewUpdate().Model(flow).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update flow")
}

// startSimpleInstance starts an instance on simple_flow with defaults.
func (s *InstanceServiceEdgeCaseTestSuite) startSimpleInstance(title string) *approval.Instance {
	s.T().Helper()

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       title,
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start instance")

	return instance
}

// deployAndPublishMinimalFlow deploys a start->end flow and publishes it.
func (s *InstanceServiceEdgeCaseTestSuite) deployAndPublishMinimalFlow(flowCode, flowName string) (*approval.Flow, approval.FlowVersion) {
	s.T().Helper()

	def := minimalFlowDefinition()
	data, _ := json.Marshal(def)

	flow, err := s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   flowCode,
		FlowName:   flowName,
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	s.Require().NoError(err, "Should deploy flow")

	var version approval.FlowVersion
	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).OrderByDesc("version").Limit(1).Scan(s.ctx)
	s.Require().NoError(err, "Should find flow version")

	return flow, version
}

// advanceMultiStageToSecondApproval builds a multi-stage flow, starts an instance,
// and approves the first stage, returning the instance and second-stage tasks.
func (s *InstanceServiceEdgeCaseTestSuite) advanceMultiStageToSecondApproval(title string) (*approval.Instance, []*approval.FlowNode, []approval.Task) {
	s.T().Helper()

	_, _, nodes := buildMultiStageFlow(s.T(), s.ctx, s.db)
	approval1 := nodes[1]
	approval2 := nodes[2]

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "multi_stage_flow",
		Title:       title,
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start multi-stage instance")

	tasks1 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval1.ID)
	s.Require().Len(tasks1, 1, "First approval should have one task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks1[0].ID,
		Action:     "approve",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should approve first stage")

	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Second approval should have one task")

	return instance, nodes, tasks2
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskUnsupportedAction() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance := s.startSimpleInstance("Unsupported Action")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "invalid_action",
		OperatorID: "user1",
	})
	s.Require().Error(err, "Invalid action should fail")
	s.Contains(err.Error(), "unsupported action", "Error should mention unsupported action")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskOpinionRequired() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	approvalNode.IsOpinionRequired = true
	_, err := s.db.NewUpdate().Model(approvalNode).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update node")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Opinion Required",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start instance")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	// Both approve and reject should fail without opinion
	for _, action := range []string{"approve", "reject"} {
		err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
			InstanceID: instance.ID,
			TaskID:     tasks[0].ID,
			Action:     action,
			OperatorID: "user1",
			Opinion:    "",
		})
		s.ErrorIs(err, ErrOpinionRequired, "Action %q without opinion should fail", action)
	}
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskFormDataMerge() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	approvalNode.FieldPermissions = map[string]any{
		"amount": "editable",
		"status": "visible",
	}
	_, err := s.db.NewUpdate().Model(approvalNode).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update node permissions")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Form Data Merge",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"amount": 100, "status": "draft"},
	})
	s.Require().NoError(err, "Should start instance")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "approve",
		OperatorID: "user1",
		FormData:   map[string]any{"amount": 200, "status": "approved"},
	})
	s.Require().NoError(err, "Should approve with form data")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(float64(200), inst.FormData["amount"], "Editable field should be updated")
	s.Equal("draft", inst.FormData["status"], "Visible-only field should remain unchanged")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskTransferMissingTarget() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance := s.startSimpleInstance("Transfer No Target")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks[0].ID,
		Action:       "transfer",
		OperatorID:   "user1",
		TransferToID: "",
	})
	s.Require().Error(err, "Transfer without target should fail")
	s.Contains(err.Error(), "transfer target user ID required", "Error should mention missing target")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRollbackMissingTargetNodeID() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	instance := s.startSimpleInstance("Rollback No Target")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks[0].ID,
		Action:       "rollback",
		OperatorID:   "user1",
		TargetNodeID: "",
	})
	s.Require().Error(err, "Rollback without target should fail")
	s.Contains(err.Error(), "target node ID required for rollback", "Error should mention missing target node")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRollbackTargetNoneType() {
	_, _, nodes := buildMultiStageFlow(s.T(), s.ctx, s.db)
	approval1 := nodes[1]
	approval2 := nodes[2]

	approval2.IsRollbackAllowed = true
	approval2.RollbackType = approval.RollbackNone
	_, err := s.db.NewUpdate().Model(approval2).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update rollback type")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "multi_stage_flow",
		Title:       "Rollback None",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start instance")

	tasks1 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval1.ID)
	s.Require().Len(tasks1, 1, "First approval should have one task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks1[0].ID,
		Action:     "approve",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should approve first stage")

	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Second approval should have one task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks2[0].ID,
		Action:       "rollback",
		OperatorID:   "user2",
		TargetNodeID: approval1.ID,
	})
	s.ErrorIs(err, ErrRollbackNotAllowed, "RollbackNone should prevent rollback")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRollbackTargetStartType() {
	instance, nodes, _ := s.advanceMultiStageToSecondApproval("Rollback Start")
	startNode := nodes[0]
	approval2 := nodes[2]

	approval2.IsRollbackAllowed = true
	approval2.RollbackType = approval.RollbackStart
	_, err := s.db.NewUpdate().Model(approval2).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update rollback type")

	// Re-query tasks after node update
	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Should have second-stage task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks2[0].ID,
		Action:       "rollback",
		OperatorID:   "user2",
		TargetNodeID: startNode.ID,
	})
	s.Require().NoError(err, "Should rollback to start")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceRunning, inst.Status, "Instance should still be running after rollback")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRollbackTargetStartTypeInvalidTarget() {
	instance, nodes, _ := s.advanceMultiStageToSecondApproval("Rollback Start Invalid")
	approval1 := nodes[1]
	approval2 := nodes[2]

	approval2.IsRollbackAllowed = true
	approval2.RollbackType = approval.RollbackStart
	_, err := s.db.NewUpdate().Model(approval2).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update rollback type")

	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Should have second-stage task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks2[0].ID,
		Action:       "rollback",
		OperatorID:   "user2",
		TargetNodeID: approval1.ID,
	})
	s.ErrorIs(err, ErrInvalidRollbackTarget, "Non-start target should be rejected for RollbackStart")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestCheckInitiationPermissionByDept() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	s.disableAllInitiate(flow)
	s.insertInitiator(flow.ID, approval.InitiatorDept, []string{"dept_engineering"})

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:        "simple_flow",
		Title:           "Dept Allowed",
		ApplicantID:     "applicant1",
		ApplicantDeptID: "dept_engineering",
		FormData:        map[string]any{},
	})
	s.Require().NoError(err, "Dept initiator should allow start")
	s.NotNil(instance, "Instance should be created")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestCheckInitiationPermissionByRole() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	s.disableAllInitiate(flow)
	s.insertInitiator(flow.ID, approval.InitiatorRole, []string{"role_admin"})

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Role Allowed",
		ApplicantID: "admin1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Role initiator should allow start")
	s.NotNil(instance, "Instance should be created")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestCheckInitiationPermissionNoInitiators() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	s.disableAllInitiate(flow)

	_, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "No Initiators",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.ErrorIs(err, ErrNotAllowedInitiate, "Should deny when no initiators configured")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddCCSuccess() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance := s.startSimpleInstance("CC Success")

	if instance.CurrentNodeID.Valid {
		_, err := s.db.NewUpdate().Model((*approval.FlowNode)(nil)).
			Set("is_manual_cc_allowed", true).
			Where(func(c orm.ConditionBuilder) {
				c.Equals("id", instance.CurrentNodeID.String)
			}).Exec(s.ctx)
		s.Require().NoError(err, "Should enable manual CC")
	}

	err := s.svc.AddCC(s.ctx, instance.ID, []string{"cc_user1", "cc_user2"}, "applicant1")
	s.Require().NoError(err, "Should add CC users")

	var records []approval.CCRecord
	err = s.db.NewSelect().Model(&records).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instance.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query CC records")
	s.Len(records, 2, "Should have two CC records")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddCCInstanceNotFound() {
	err := s.svc.AddCC(s.ctx, "nonexistent", []string{"cc_user1"}, "applicant1")
	s.ErrorIs(err, ErrInstanceNotFound, "Should fail for nonexistent instance")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddCCCompletedInstance() {
	buildAutoCompleteFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "auto_complete_flow",
		Title:       "Completed CC",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start auto-complete instance")

	err = s.svc.AddCC(s.ctx, instance.ID, []string{"cc_user1"}, "applicant1")
	s.Require().Error(err, "AddCC should fail on completed instance")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskTaskNotFound() {
	buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Task Not Found")

	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     "nonexistent_task",
		Action:     "approve",
		OperatorID: "user1",
	})
	s.ErrorIs(err, ErrTaskNotFound, "Should fail for nonexistent task")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskTaskNotPending() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Task Not Pending")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[1].ID,
		Action:     "approve",
		OperatorID: "user2",
	})
	s.ErrorIs(err, ErrTaskNotPending, "Waiting task should not be processable")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestWithdrawInstanceNotFound() {
	err := s.svc.Withdraw(s.ctx, "nonexistent", "user1", "reason")
	s.ErrorIs(err, ErrInstanceNotFound, "Should fail for nonexistent instance")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeInstanceNotFound() {
	err := s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: "nonexistent",
		TaskID:     "task1",
		UserIDs:    []string{"user_x"},
		AddType:    "before",
		OperatorID: "user1",
	})
	s.ErrorIs(err, ErrInstanceNotFound, "Should fail for nonexistent instance")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeInstanceCompleted() {
	buildAutoCompleteFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "auto_complete_flow",
		Title:       "Completed AddAssignee",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start auto-complete instance")

	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     "any_task",
		UserIDs:    []string{"user_x"},
		AddType:    "before",
		OperatorID: "user1",
	})
	s.ErrorIs(err, ErrInstanceCompleted, "Should fail on completed instance")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeNodeNotAllowed() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Node Not Allowed",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start parallel instance")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_x"},
		AddType:    "before",
		OperatorID: tasks[0].AssigneeID,
	})
	s.ErrorIs(err, ErrAddAssigneeNotAllowed, "Should fail when node disallows add assignee")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRemoveAssigneeTaskNotFound() {
	err := s.svc.RemoveAssignee(s.ctx, "nonexistent_task", "user1")
	s.ErrorIs(err, ErrTaskNotFound, "Should fail for nonexistent task")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRemoveAssigneeNodeNotAllowed() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Remove Not Allowed",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start parallel instance")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err = s.svc.RemoveAssignee(s.ctx, tasks[0].ID, tasks[1].AssigneeID)
	s.ErrorIs(err, ErrRemoveAssigneeNotAllowed, "Should fail when node disallows remove assignee")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestStartInstanceWithBusinessRecordID() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:         "simple_flow",
		Title:            "Business Record",
		ApplicantID:      "applicant1",
		BusinessRecordID: "biz_record_001",
		FormData:         map[string]any{},
	})
	s.Require().NoError(err, "Should start instance with business record ID")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.True(inst.BusinessRecordID.Valid, "BusinessRecordID should be set")
	s.Equal("biz_record_001", inst.BusinessRecordID.String, "BusinessRecordID should match")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestStartInstanceWithApplicantDeptID() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:        "simple_flow",
		Title:           "Dept ID",
		ApplicantID:     "applicant1",
		ApplicantDeptID: "dept_001",
		FormData:        map[string]any{},
	})
	s.Require().NoError(err, "Should start instance with dept ID")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.True(inst.ApplicantDeptID.Valid, "ApplicantDeptID should be set")
	s.Equal("dept_001", inst.ApplicantDeptID.String, "ApplicantDeptID should match")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRollbackDataKeepStrategy() {
	_, _, nodes := buildMultiStageFlow(s.T(), s.ctx, s.db)
	approval1 := nodes[1]
	approval2 := nodes[2]

	approval2.IsRollbackAllowed = true
	approval2.RollbackType = approval.RollbackAny
	approval2.RollbackDataStrategy = approval.RollbackDataKeep
	_, err := s.db.NewUpdate().Model(approval2).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update rollback strategy")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "multi_stage_flow",
		Title:       "Rollback Data Keep",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"amount": 1000},
	})
	s.Require().NoError(err, "Should start instance")

	tasks1 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval1.ID)
	s.Require().Len(tasks1, 1, "First approval should have one task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks1[0].ID,
		Action:     "approve",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should approve first stage")

	snapshot := &approval.FormSnapshot{
		InstanceID: instance.ID,
		NodeID:     approval1.ID,
		FormData:   map[string]any{"amount": 500},
	}
	snapshot.ID = id.Generate()
	snapshot.CreatedBy = "system"
	_, err = s.db.NewInsert().Model(snapshot).Exec(s.ctx)
	s.Require().NoError(err, "Should insert form snapshot")

	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Second approval should have one task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks2[0].ID,
		Action:       "rollback",
		OperatorID:   "user2",
		TargetNodeID: approval1.ID,
	})
	s.Require().NoError(err, "Should rollback with data keep")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceRunning, inst.Status, "Instance should still be running")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskFormDataWithNilInstanceFormData() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	approvalNode.FieldPermissions = map[string]any{"amount": "editable"}
	_, err := s.db.NewUpdate().Model(approvalNode).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update node permissions")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Nil FormData Merge",
		ApplicantID: "applicant1",
		FormData:    nil,
	})
	s.Require().NoError(err, "Should start instance with nil form data")

	// Force instance form_data to NULL
	_, err = s.db.NewUpdate().Model((*approval.Instance)(nil)).
		Set("form_data", null.String{}).
		Where(func(c orm.ConditionBuilder) {
			c.Equals("id", instance.ID)
		}).Exec(s.ctx)
	s.Require().NoError(err, "Should nullify form data")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "approve",
		OperatorID: "user1",
		FormData:   map[string]any{"amount": 999},
	})
	s.Require().NoError(err, "Should merge form data into nil instance form data")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestPublishVersionVersionNotFound() {
	err := s.flowSvc.PublishVersion(s.ctx, "nonexistent_version_id", "admin")
	s.ErrorIs(err, ErrFlowNotFound, "Should fail for nonexistent version")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestGetInstanceDetailTaskQueryError() {
	buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Detail Task Error")

	_, err := s.db.NewRaw("DROP TABLE apv_task").Exec(s.ctx)
	s.Require().NoError(err, "Should drop task table")

	_, err = NewQueryService(s.db).GetInstanceDetail(s.ctx, instance.ID)
	s.Require().Error(err, "Should fail with dropped task table")
	s.Contains(err.Error(), "query tasks", "Error should mention task query failure")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestGetInstanceDetailActionLogQueryError() {
	buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Detail ActionLog Error")

	_, err := s.db.NewRaw("DROP TABLE apv_action_log").Exec(s.ctx)
	s.Require().NoError(err, "Should drop action log table")

	_, err = NewQueryService(s.db).GetInstanceDetail(s.ctx, instance.ID)
	s.Require().Error(err, "Should fail with dropped action log table")
	s.Contains(err.Error(), "query action logs", "Error should mention action log query failure")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestGetInstanceDetailFlowNodesQueryError() {
	buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Detail FlowNodes Error")

	_, err := s.db.NewRaw("DROP TABLE apv_flow_node").Exec(s.ctx)
	s.Require().NoError(err, "Should drop flow node table")

	_, err = NewQueryService(s.db).GetInstanceDetail(s.ctx, instance.ID)
	s.Require().Error(err, "Should fail with dropped flow node table")
	s.Contains(err.Error(), "query flow nodes", "Error should mention flow node query failure")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestGetActionLogsQueryError() {
	_, err := s.db.NewRaw("DROP TABLE apv_action_log").Exec(s.ctx)
	s.Require().NoError(err, "Should drop action log table")

	_, err = NewQueryService(s.db).GetActionLogs(s.ctx, "any_instance_id")
	s.Require().Error(err, "Should fail with dropped action log table")
	s.Contains(err.Error(), "query action logs", "Error should mention action log query failure")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRollbackDataKeepNoSnapshot() {
	_, _, nodes := buildMultiStageFlow(s.T(), s.ctx, s.db)
	approval1 := nodes[1]
	approval2 := nodes[2]

	approval2.IsRollbackAllowed = true
	approval2.RollbackType = approval.RollbackAny
	approval2.RollbackDataStrategy = approval.RollbackDataKeep
	_, err := s.db.NewUpdate().Model(approval2).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update rollback strategy")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "multi_stage_flow",
		Title:       "Rollback No Snapshot",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"amount": 1000},
	})
	s.Require().NoError(err, "Should start instance")

	tasks1 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval1.ID)
	s.Require().Len(tasks1, 1, "First approval should have one task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks1[0].ID,
		Action:     "approve",
		OperatorID: "user1",
	})
	s.Require().NoError(err, "Should approve first stage")

	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Second approval should have one task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks2[0].ID,
		Action:       "rollback",
		OperatorID:   "user2",
		TargetNodeID: approval1.ID,
	})
	s.Require().NoError(err, "Rollback without snapshot should still succeed")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceRunning, inst.Status, "Instance should still be running")
	s.Equal(float64(1000), inst.FormData["amount"], "Form data should be preserved when no snapshot exists")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRollbackTargetAnyTypeInvalidTarget() {
	instance, nodes, _ := s.advanceMultiStageToSecondApproval("Rollback Any Invalid")
	approval2 := nodes[2]

	approval2.IsRollbackAllowed = true
	approval2.RollbackType = approval.RollbackAny
	_, err := s.db.NewUpdate().Model(approval2).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update rollback type")

	tasks2 := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approval2.ID)
	s.Require().Len(tasks2, 1, "Should have second-stage task")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks2[0].ID,
		Action:       "rollback",
		OperatorID:   "user2",
		TargetNodeID: "completely_nonexistent_node",
	})
	s.ErrorIs(err, ErrInvalidRollbackTarget, "Non-existent target should be rejected")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestCheckInitiationPermissionRoleNotMatched() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	s.disableAllInitiate(flow)
	s.insertInitiator(flow.ID, approval.InitiatorRole, []string{"role_admin"})

	// "applicant1" is not in role_admin users (admin1, admin2)
	_, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Role Not Matched",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.ErrorIs(err, ErrNotAllowedInitiate, "Non-matching role should be denied")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestCheckInitiationPermissionNilUserService() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	s.disableAllInitiate(flow)
	s.insertInitiator(flow.ID, approval.InitiatorRole, []string{"role_admin"})

	pub := publisher.NewEventPublisher()
	eng := setupEngine(s.mockOrg, s.mockUser, pub)
	svcNoUser := NewInstanceService(s.db, eng, s.serialGen, pub, nil)

	_, err := svcNoUser.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Nil UserService",
		ApplicantID: "admin1",
		FormData:    map[string]any{},
	})
	s.ErrorIs(err, ErrNotAllowedInitiate, "Nil user service should deny role-based initiation")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskRollbackActionLogFields() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Rollback ActionLog")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks[0].ID,
		Action:       "rollback",
		OperatorID:   "user1",
		TargetNodeID: approvalNode.ID,
		Opinion:      "need revision",
	})
	s.Require().NoError(err, "Should process rollback")

	var logs []approval.ActionLog
	err = s.db.NewSelect().Model(&logs).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instance.ID)
		c.Equals("action", string(approval.ActionRollback))
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query rollback logs")
	s.Require().Len(logs, 1, "Should have one rollback log")
	s.True(logs[0].RollbackToNodeID.Valid, "RollbackToNodeID should be set")
	s.Equal(approvalNode.ID, logs[0].RollbackToNodeID.String, "RollbackToNodeID should match target")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskTransferActionLogFields() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Transfer ActionLog")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID:   instance.ID,
		TaskID:       tasks[0].ID,
		Action:       "transfer",
		OperatorID:   "user1",
		TransferToID: "user3",
		Opinion:      "please handle",
	})
	s.Require().NoError(err, "Should process transfer")

	var logs []approval.ActionLog
	err = s.db.NewSelect().Model(&logs).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instance.ID)
		c.Equals("action", string(approval.ActionTransfer))
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query transfer logs")
	s.Require().Len(logs, 1, "Should have one transfer log")
	s.True(logs[0].TransferToID.Valid, "TransferToID should be set")
	s.Equal("user3", logs[0].TransferToID.String, "TransferToID should match target user")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestWithdrawWithEmptyReason() {
	buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Withdraw Empty Reason")

	err := s.svc.Withdraw(s.ctx, instance.ID, "applicant1", "")
	s.Require().NoError(err, "Withdraw with empty reason should succeed")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceWithdrawn, inst.Status, "Instance should be withdrawn")

	var logs []approval.ActionLog
	err = s.db.NewSelect().Model(&logs).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instance.ID)
		c.Equals("action", string(approval.ActionWithdraw))
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query withdraw logs")
	s.Require().Len(logs, 1, "Should have one withdraw log")
	s.False(logs[0].Opinion.Valid, "Opinion should not be valid when reason is empty")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRemoveAssigneeFlowAdmin() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	var flow approval.Flow
	err := s.db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("code", "simple_flow")
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query flow")

	flow.AdminUserIDs = []string{"flow_admin"}
	_, err = s.db.NewUpdate().Model(&flow).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update flow admin")

	instance := s.startSimpleInstance("Flow Admin Remove")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Should have two tasks")

	err = s.svc.RemoveAssignee(s.ctx, tasks[0].ID, "flow_admin")
	s.Require().NoError(err, "Flow admin should be able to remove assignee")

	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	removedFound := false
	for _, t := range tasks {
		if t.AssigneeID == "user1" {
			s.Equal(approval.TaskRemoved, t.Status, "user1 task should have removed status")
			removedFound = true
		}
	}
	s.True(removedFound, "user1 task should be found as removed")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRemoveAssigneeNonSequentialNoPromotion() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	approvalNode.IsRemoveAssigneeAllowed = true
	_, err := s.db.NewUpdate().Model(approvalNode).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should enable remove assignee")

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Parallel Remove",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start parallel instance")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 3, "Should have three parallel tasks")

	err = s.svc.RemoveAssignee(s.ctx, tasks[0].ID, tasks[1].AssigneeID)
	s.Require().NoError(err, "Peer should be able to remove assignee")

	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	removedCount := 0
	for _, t := range tasks {
		if t.Status == approval.TaskRemoved {
			removedCount++
		}
	}
	s.Equal(1, removedCount, "Should have exactly one removed task")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeTaskNotFound() {
	buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("AddAssignee TaskNotFound")

	err := s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     "nonexistent_task_id",
		UserIDs:    []string{"user_x"},
		AddType:    "before",
		OperatorID: "user1",
	})
	s.ErrorIs(err, ErrTaskNotFound, "Should fail for nonexistent task")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestHandleApproveHandleNodeStatus() {
	_, _, _, handleNode, _ := buildHandleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "handle_flow",
		Title:       "Handle Status",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start handle flow instance")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, handleNode.ID)
	s.Require().NotEmpty(tasks, "Should have handle tasks")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "approve",
		OperatorID: tasks[0].AssigneeID,
	})
	s.Require().NoError(err, "Should approve handle task")

	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, handleNode.ID)
	for _, t := range tasks {
		if t.ID == tasks[0].ID {
			s.Equal(approval.TaskHandled, t.Status, "Handle node tasks should get TaskHandled status")
		}
	}
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRejectTriggersPassRuleRejected() {
	_, _, _, approvalNode, _ := buildParallelFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "parallel_flow",
		Title:       "Reject PassRuleRejected",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start parallel instance")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "reject",
		OperatorID: tasks[0].AssigneeID,
		Opinion:    "not approved",
	})
	s.Require().NoError(err, "Should process rejection")

	inst := queryInstance(s.T(), s.ctx, s.db, instance.ID)
	s.Equal(approval.InstanceRejected, inst.Status, "Instance should be rejected via PassAnyReject")
	s.True(inst.FinishedAt.Valid, "FinishedAt should be set after rejection")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestStartInstanceFlowNotActive() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	flow.IsActive = false
	_, err := s.db.NewUpdate().Model(flow).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should deactivate flow")

	_, err = s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Flow Not Active",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.ErrorIs(err, ErrFlowNotActive, "Inactive flow should reject start")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestStartInstanceNoPublishedVersion() {
	_, version, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	version.Status = approval.VersionDraft
	_, err := s.db.NewUpdate().Model(version).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should set version to draft")

	_, err = s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "No Published Version",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.ErrorIs(err, ErrNoPublishedVersion, "Should fail without published version")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestPublishVersionAlreadyPublished() {
	flow, version := s.deployAndPublishMinimalFlow("publish_test", "Publish Test")
	_ = flow

	err := s.flowSvc.PublishVersion(s.ctx, version.ID, "admin")
	s.Require().NoError(err, "Should publish version")

	err = s.flowSvc.PublishVersion(s.ctx, version.ID, "admin")
	s.ErrorIs(err, ErrVersionNotDraft, "Re-publishing should fail")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestDeployFlowUpdateExistingFlow() {
	data, _ := json.Marshal(minimalFlowDefinition())

	flow1, err := s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "update_test",
		FlowName:   "Original Flow",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	s.Require().NoError(err, "Should deploy original flow")
	s.Equal("Original Flow", flow1.Name, "Flow name should match")

	flow2, err := s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "update_test",
		FlowName:   "Updated Flow",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	s.Require().NoError(err, "Should update existing flow")
	s.Equal("Updated Flow", flow2.Name, "Flow name should be updated")
	s.Equal(flow1.ID, flow2.ID, "Same flow should be updated, not recreated")

	var versions []approval.FlowVersion
	err = s.db.NewSelect().Model(&versions).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow2.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query versions")
	s.Len(versions, 2, "Should have two versions after re-deploy")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestDeployFlowEdgeWithSourceHandle() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "start", Target: "end", SourceHandle: "branch_1"},
		},
	}
	data, _ := json.Marshal(def)

	flow, err := s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "handle_test",
		FlowName:   "SourceHandle Test",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	s.Require().NoError(err, "Should deploy flow with source handle")

	var version approval.FlowVersion
	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should find version")

	var edges []approval.FlowEdge
	err = s.db.NewSelect().Model(&edges).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query edges")
	s.Require().Len(edges, 1, "Should have one edge")
	s.True(edges[0].SourceHandle.Valid, "SourceHandle should be set")
	s.Equal("branch_1", edges[0].SourceHandle.String, "SourceHandle should be branch_1")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRemoveAssigneeLastAssignee() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	approvalNode.PassRule = approval.PassAll
	_, err := s.db.NewUpdate().Model(approvalNode).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update pass rule")

	instance := s.startSimpleInstance("Last Assignee")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Should have two sequential tasks")

	// user2 tries to remove user1 (the only pending task)
	err = s.svc.RemoveAssignee(s.ctx, tasks[0].ID, tasks[1].AssigneeID)
	if err != nil {
		s.ErrorIs(err, ErrLastAssigneeRemoval, "Should either succeed or fail with last assignee error")
	}
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRemoveAssigneeSequentialPromotes() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Sequential Promote")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().Len(tasks, 2, "Should have two sequential tasks")
	s.Equal(approval.TaskPending, tasks[0].Status, "First task should be pending")
	s.Equal(approval.TaskWaiting, tasks[1].Status, "Second task should be waiting")

	err := s.svc.RemoveAssignee(s.ctx, tasks[0].ID, tasks[1].AssigneeID)
	s.Require().NoError(err, "Should remove pending assignee")

	tasks = queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	for _, t := range tasks {
		if t.AssigneeID == "user2" && t.Status != approval.TaskRemoved {
			s.Equal(approval.TaskPending, t.Status, "user2 should be promoted to pending")
		}
	}
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeBefore() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("AddAssignee Before")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_before"},
		AddType:    "before",
		OperatorID: tasks[0].AssigneeID,
	})
	s.Require().NoError(err, "Should add assignee before")

	allTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	for _, t := range allTasks {
		if t.ID == tasks[0].ID {
			s.Equal(approval.TaskWaiting, t.Status, "Original task should become waiting")
		}
		if t.AssigneeID == "user_before" {
			s.Equal(approval.TaskPending, t.Status, "Before-added task should be pending")
		}
	}
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeAfter() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("AddAssignee After")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_after"},
		AddType:    "after",
		OperatorID: tasks[0].AssigneeID,
	})
	s.Require().NoError(err, "Should add assignee after")

	allTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	for _, t := range allTasks {
		if t.AssigneeID == "user_after" {
			s.Equal(approval.TaskWaiting, t.Status, "After-added task should be waiting")
		}
	}
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeParallel() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("AddAssignee Parallel")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_parallel"},
		AddType:    "parallel",
		OperatorID: tasks[0].AssigneeID,
	})
	s.Require().NoError(err, "Should add assignee parallel")

	allTasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	for _, t := range allTasks {
		if t.AssigneeID == "user_parallel" {
			s.Equal(approval.TaskPending, t.Status, "Parallel-added task should be pending")
		}
	}
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeInvalidAddType() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("AddAssignee Invalid Type")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_x"},
		AddType:    "invalid_type",
		OperatorID: tasks[0].AssigneeID,
	})
	s.ErrorIs(err, ErrInvalidAddAssigneeType, "Invalid add type should be rejected")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeRestrictedAddTypes() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	approvalNode.AddAssigneeTypes = []string{"after"}
	_, err := s.db.NewUpdate().Model(approvalNode).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should restrict add types")

	instance := s.startSimpleInstance("Restricted AddTypes")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err = s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_x"},
		AddType:    "before",
		OperatorID: tasks[0].AssigneeID,
	})
	s.ErrorIs(err, ErrInvalidAddAssigneeType, "Disallowed add type should be rejected")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddAssigneeNotAssignee() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Not Assignee")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.AddAssignee(s.ctx, AddAssigneeCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UserIDs:    []string{"user_x"},
		AddType:    "before",
		OperatorID: "wrong_user",
	})
	s.ErrorIs(err, ErrNotAssignee, "Non-assignee should be rejected")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestWithdrawNotApplicant() {
	buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Not Applicant Withdraw")

	err := s.svc.Withdraw(s.ctx, instance.ID, "wrong_user", "reason")
	s.ErrorIs(err, ErrNotApplicant, "Non-applicant should not be able to withdraw")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestWithdrawAlreadyCompleted() {
	buildAutoCompleteFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "auto_complete_flow",
		Title:       "Already Completed",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start auto-complete instance")

	err = s.svc.Withdraw(s.ctx, instance.ID, "applicant1", "reason")
	s.ErrorIs(err, ErrWithdrawNotAllowed, "Completed instance should not allow withdraw")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestAddCCManualCCNotAllowed() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)

	approvalNode.IsManualCCAllowed = false
	_, err := s.db.NewUpdate().Model(approvalNode).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should disable manual CC")

	instance := s.startSimpleInstance("CC Not Allowed")

	err = s.svc.AddCC(s.ctx, instance.ID, []string{"cc_user1"}, "applicant1")
	s.ErrorIs(err, ErrManualCcNotAllowed, "Manual CC should be rejected when disabled")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestDeployFlowWithNodeBranchConditions() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "cond", Type: "condition", Data: map[string]any{
				"label": "Condition",
				"branches": []any{
					map[string]any{"id": "b1", "label": "High", "conditions": []any{
						map[string]any{"type": "field", "subject": "amount", "operator": ">", "value": 100},
					}, "priority": 0},
					map[string]any{"id": "b2", "label": "Default", "isDefault": true, "priority": 1},
				},
			}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "start", Target: "cond"},
			{Source: "cond", Target: "end", SourceHandle: "b1"},
			{Source: "cond", Target: "end", SourceHandle: "b2"},
		},
	}
	data, _ := json.Marshal(def)

	flow, err := s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "cond_test",
		FlowName:   "Conditions Test",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	s.Require().NoError(err, "Should deploy flow with conditions")

	var version approval.FlowVersion
	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should find version")

	// Verify condition node has branches
	var condNode approval.FlowNode
	err = s.db.NewSelect().Model(&condNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
		c.Equals("node_key", "cond")
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should find condition node")
	s.Require().Len(condNode.Branches, 2, "Condition node should have two branches")
	s.Len(condNode.Branches[0].Conditions, 1, "First branch should have one condition")

	// Verify edges were created
	var edges []approval.FlowEdge
	err = s.db.NewSelect().Model(&edges).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query edges")
	s.Require().Len(edges, 3, "Should have three edges")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestRemoveAssigneeNotAuthorized() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Not Authorized Remove")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.RemoveAssignee(s.ctx, tasks[0].ID, "random_user")
	s.ErrorIs(err, ErrNotAssignee, "Non-assignee should not be able to remove")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestCheckInitiationPermissionGetUsersByRoleError() {
	flow, _, _, _, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	s.disableAllInitiate(flow)
	s.insertInitiator(flow.ID, approval.InitiatorRole, []string{"nonexistent_role"})

	_, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Role Error",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.ErrorIs(err, ErrNotAllowedInitiate, "Empty role users should deny initiation")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskInstanceCompleted() {
	buildAutoCompleteFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "auto_complete_flow",
		Title:       "Completed Instance",
		ApplicantID: "applicant1",
		FormData:    map[string]any{},
	})
	s.Require().NoError(err, "Should start auto-complete instance")

	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     "any_task",
		Action:     "approve",
		OperatorID: "user1",
	})
	s.ErrorIs(err, ErrInstanceCompleted, "Completed instance should reject task processing")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestProcessTaskNotAssignee() {
	_, _, _, approvalNode, _ := buildSimpleFlow(s.T(), s.ctx, s.db)
	instance := s.startSimpleInstance("Not Assignee ProcessTask")

	tasks := queryTasksByNode(s.T(), s.ctx, s.db, instance.ID, approvalNode.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	err := s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "approve",
		OperatorID: "wrong_user",
	})
	s.ErrorIs(err, ErrNotAssignee, "Non-assignee should not be able to process task")
}

func (s *InstanceServiceEdgeCaseTestSuite) TestPublishVersionArchiveOldPublished() {
	data, _ := json.Marshal(minimalFlowDefinition())

	flow, err := s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "archive_test",
		FlowName:   "Archive Test",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	s.Require().NoError(err, "Should deploy v1")

	var v1 approval.FlowVersion
	err = s.db.NewSelect().Model(&v1).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
		c.Equals("version", 1)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should find v1")

	err = s.flowSvc.PublishVersion(s.ctx, v1.ID, "admin")
	s.Require().NoError(err, "Should publish v1")

	_, err = s.flowSvc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "archive_test",
		FlowName:   "Archive Test v2",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	s.Require().NoError(err, "Should deploy v2")

	var v2 approval.FlowVersion
	err = s.db.NewSelect().Model(&v2).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
		c.Equals("version", 2)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should find v2")

	err = s.flowSvc.PublishVersion(s.ctx, v2.ID, "admin")
	s.Require().NoError(err, "Should publish v2")

	err = s.db.NewSelect().Model(&v1).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should re-read v1")
	err = s.db.NewSelect().Model(&v2).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should re-read v2")

	s.Equal(approval.VersionArchived, v1.Status, "v1 should be archived")
	s.Equal(approval.VersionPublished, v2.Status, "v2 should be published")
}

// ==================== Urge Task ====================

// TestUrgeTaskSuccess verifies successful urge task operation.
func (s *InstanceServiceTestSuite) TestUrgeTaskSuccess() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Urge Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"reason": "test"},
	})
	s.Require().NoError(err, "Should start instance")

	tasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	pendingTask := tasks[0]

	err = s.svc.UrgeTask(s.ctx, UrgeTaskCmd{
		InstanceID: instance.ID,
		TaskID:     pendingTask.ID,
		UrgerID:    "applicant1",
		Message:    "请尽快处理",
	})
	s.Require().NoError(err, "Should urge task")

	// Verify urge record was created
	var records []approval.UrgeRecord
	err = s.db.NewSelect().Model(&records).Where(func(c orm.ConditionBuilder) {
		c.Equals("task_id", pendingTask.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should query urge records")
	s.Len(records, 1, "Should have one urge record")
	s.Equal("applicant1", records[0].UrgerID, "Urger should be applicant1")
	s.Equal(pendingTask.AssigneeID, records[0].TargetUserID, "Target should be assignee")
	s.Equal("请尽快处理", records[0].Message, "Message should match")

	// Verify event was published
	events := queryEvents(s.T(), s.ctx, s.db)
	found := false
	for _, evt := range events {
		if evt.EventType == "approval.task.urged" {
			found = true
			break
		}
	}
	s.True(found, "Should have urge event")
}

// TestUrgeTaskNotFound verifies urge with invalid task ID.
func (s *InstanceServiceTestSuite) TestUrgeTaskNotFound() {
	err := s.svc.UrgeTask(s.ctx, UrgeTaskCmd{
		InstanceID: "nonexistent",
		TaskID:     "nonexistent",
		UrgerID:    "user1",
	})
	s.Error(err, "Should return error for nonexistent task")
}

// TestUrgeTaskNotPending verifies urge fails for completed tasks.
func (s *InstanceServiceTestSuite) TestUrgeTaskNotPending() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Urge Not Pending Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"reason": "test"},
	})
	s.Require().NoError(err, "Should start instance")

	tasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	// Approve the first task so it becomes non-pending
	err = s.svc.ProcessTask(s.ctx, ProcessTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		Action:     "approve",
		OperatorID: tasks[0].AssigneeID,
		Opinion:    "ok",
	})
	s.Require().NoError(err, "Should approve task")

	// Try to urge the already-approved task
	err = s.svc.UrgeTask(s.ctx, UrgeTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UrgerID:    "applicant1",
	})
	s.Error(err, "Should return error for non-pending task")
}

// TestUrgeTaskCooldown verifies cooldown enforcement.
func (s *InstanceServiceTestSuite) TestUrgeTaskCooldown() {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.svc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       "Urge Cooldown Test",
		ApplicantID: "applicant1",
		FormData:    map[string]any{"reason": "test"},
	})
	s.Require().NoError(err, "Should start instance")

	tasks := queryTasks(s.T(), s.ctx, s.db, instance.ID)
	s.Require().NotEmpty(tasks, "Should have tasks")

	cmd := UrgeTaskCmd{
		InstanceID: instance.ID,
		TaskID:     tasks[0].ID,
		UrgerID:    "applicant1",
		Message:    "please hurry",
	}

	// First urge should succeed
	err = s.svc.UrgeTask(s.ctx, cmd)
	s.Require().NoError(err, "First urge should succeed")

	// Second urge should fail (cooldown)
	err = s.svc.UrgeTask(s.ctx, cmd)
	s.Error(err, "Second urge should fail due to cooldown")
	assert.Contains(s.T(), err.Error(), "催办操作过于频繁", "Error should mention cooldown")
}
