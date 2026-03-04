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
		return &RejectTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// RejectTaskTestSuite tests the RejectTaskHandler.
type RejectTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.RejectTaskHandler
	fixture *FlowFixture
}

func (s *RejectTaskTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine()
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	pub := dispatcher.NewEventPublisher()

	s.handler = command.NewRejectTaskHandler(s.db, taskSvc, nodeSvc, validSvc, pub)
}

func (s *RejectTaskTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.ActionLog)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *RejectTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *RejectTaskTestSuite) setupRunningInstance(assigneeID string) (*approval.Instance, *approval.Task) {
	var approvalNodeID string
	for key, id := range s.fixture.NodeIDs {
		if key != "start-1" && key != "end-1" {
			approvalNodeID = id
			break
		}
	}
	s.Require().NotEmpty(approvalNodeID)

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Reject Test",
		InstanceNo:    "REJ-001",
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

func (s *RejectTaskTestSuite) TestRejectSuccess() {
	inst, task := s.setupRunningInstance("rejector-1")

	operator := approval.OperatorInfo{ID: "rejector-1", Name: "Rejector"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "Not acceptable",
	})
	s.Require().NoError(err, "Should reject task without error")

	// Verify task status
	var updated approval.Task
	updated.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.TaskRejected, updated.Status, "Task should be rejected")

	// Verify instance status (with PassAll rule, one rejection rejects instance)
	var updatedInst approval.Instance
	updatedInst.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&updatedInst).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.InstanceRejected, updatedInst.Status, "Instance should be rejected")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx))
	found := false
	for _, log := range logs {
		if log.Action == approval.ActionReject {
			found = true
			s.Assert().Equal("rejector-1", log.OperatorID)
		}
	}
	s.Assert().True(found, "Should have a reject action log")
}

func (s *RejectTaskTestSuite) TestRejectTaskNotFound() {
	operator := approval.OperatorInfo{ID: "rejector-1", Name: "Rejector"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   "non-existent",
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound)
}

func (s *RejectTaskTestSuite) TestRejectNotAssignee() {
	_, task := s.setupRunningInstance("rejector-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrNotAssignee)
}
