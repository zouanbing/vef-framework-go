package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/command"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ApproveTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// ApproveTaskTestSuite tests the ApproveTaskHandler.
type ApproveTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.ApproveTaskHandler
	fixture *FlowFixture
}

func (s *ApproveTaskTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine()
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	pub := dispatcher.NewEventPublisher()

	s.handler = command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, pub)
}

func (s *ApproveTaskTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.ActionLog)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *ApproveTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *ApproveTaskTestSuite) setupRunningInstance(assigneeID string) (*approval.Instance, *approval.Task) {
	// Find the approval node key from the fixture
	var approvalNodeID string
	for key, id := range s.fixture.NodeIDs {
		if key != "start-1" && key != "end-1" {
			approvalNodeID = id
			break
		}
	}
	s.Require().NotEmpty(approvalNodeID, "Should find approval node ID")

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Approve Test",
		InstanceNo:    "APV-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &approvalNodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     approvalNodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err)

	return inst, task
}

func (s *ApproveTaskTestSuite) TestApproveSuccess() {
	inst, task := s.setupRunningInstance("approver-1")

	operator := approval.OperatorInfo{ID: "approver-1", Name: "Approver"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "Approved",
	})
	s.Require().NoError(err, "Should approve task without error")

	// Verify task status
	var updated approval.Task
	updated.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.TaskApproved, updated.Status, "Task should be approved")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx))
	s.Assert().GreaterOrEqual(len(logs), 1, "Should have at least 1 action log")

	found := false
	for _, log := range logs {
		if log.Action == approval.ActionApprove {
			found = true
			s.Assert().Equal("approver-1", log.OperatorID)
		}
	}
	s.Assert().True(found, "Should have an approve action log")
}

func (s *ApproveTaskTestSuite) TestApproveTaskNotFound() {
	operator := approval.OperatorInfo{ID: "approver-1", Name: "Approver"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   "non-existent",
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound)
}

func (s *ApproveTaskTestSuite) TestApproveNotAssignee() {
	_, task := s.setupRunningInstance("approver-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrNotAssignee)
}

func (s *ApproveTaskTestSuite) TestApproveAlreadyCompleted() {
	_, task := s.setupRunningInstance("approver-1")

	// Set task to already approved
	_, err := s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskApproved).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err)

	operator := approval.OperatorInfo{ID: "approver-1", Name: "Approver"}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending)
}
