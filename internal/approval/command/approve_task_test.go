package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
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
	cleanRuntimeData(s.ctx, s.db)
}

func (s *ApproveTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *ApproveTaskTestSuite) newRunningInstance(assigneeID string) (*approval.Instance, *approval.Task) {
	return setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, assigneeID)
}

func (s *ApproveTaskTestSuite) TestApproveSuccess() {
	inst, task := s.newRunningInstance("approver-1")

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
	_, task := s.newRunningInstance("approver-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrNotAssignee)
}

func (s *ApproveTaskTestSuite) TestApproveAlreadyCompleted() {
	_, task := s.newRunningInstance("approver-1")

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
