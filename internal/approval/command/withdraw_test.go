package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/command"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &WithdrawTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// WithdrawTestSuite tests the WithdrawHandler.
type WithdrawTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.WithdrawHandler
	fixture *MinimalFixture
	nodeID  string
}

func (s *WithdrawTestSuite) SetupSuite() {
	s.handler = command.NewWithdrawHandler(s.db, service.NewTaskService(), dispatcher.NewEventPublisher())
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "withdraw")

	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "wd-node",
		Kind:          approval.NodeApproval,
		Name:          "Withdraw Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err)
	s.nodeID = node.ID
}

func (s *WithdrawTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.ActionLog)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *WithdrawTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *WithdrawTestSuite) insertInstance(applicantID string, status approval.InstanceStatus) *approval.Instance {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Withdraw Test",
		InstanceNo:    "WD-001",
		ApplicantID:   applicantID,
		Status:        status,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)
	return inst
}

func (s *WithdrawTestSuite) insertTask(instanceID string, status approval.TaskStatus) {
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: instanceID,
		NodeID:     s.nodeID,
		AssigneeID: "approver-1",
		SortOrder:  1,
		Status:     status,
	}
	_, err := s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err)
}

func (s *WithdrawTestSuite) TestWithdrawSuccess() {
	inst := s.insertInstance("applicant-1", approval.InstanceRunning)
	s.insertTask(inst.ID, approval.TaskPending)

	operator := approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"}
	_, err := s.handler.Handle(s.ctx, command.WithdrawCmd{
		InstanceID: inst.ID,
		Operator:   operator,
	})
	s.Require().NoError(err, "Should withdraw instance without error")

	// Verify instance status
	var updated approval.Instance
	updated.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.InstanceWithdrawn, updated.Status, "Should set status to withdrawn")

	// Verify tasks canceled
	var tasks []approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx))
	for _, t := range tasks {
		s.Assert().Equal(approval.TaskCanceled, t.Status, "All tasks should be canceled")
	}

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx))
	s.Assert().Len(logs, 1, "Should insert one action log")
	s.Assert().Equal(approval.ActionWithdraw, logs[0].Action, "Action should be withdraw")
}

func (s *WithdrawTestSuite) TestWithdrawNotApplicant() {
	inst := s.insertInstance("applicant-1", approval.InstanceRunning)

	operator := approval.OperatorInfo{ID: "other-user", Name: "Other"}
	_, err := s.handler.Handle(s.ctx, command.WithdrawCmd{
		InstanceID: inst.ID,
		Operator:   operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrNotApplicant, "Should return ErrNotApplicant")
}

func (s *WithdrawTestSuite) TestWithdrawNotAllowed() {
	inst := s.insertInstance("applicant-1", approval.InstanceApproved)

	operator := approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"}
	_, err := s.handler.Handle(s.ctx, command.WithdrawCmd{
		InstanceID: inst.ID,
		Operator:   operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrWithdrawNotAllowed, "Should not allow withdrawal of approved instance")
}

func (s *WithdrawTestSuite) TestWithdrawInstanceNotFound() {
	operator := approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"}
	_, err := s.handler.Handle(s.ctx, command.WithdrawCmd{
		InstanceID: "non-existent",
		Operator:   operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrInstanceNotFound, "Should return ErrInstanceNotFound")
}
