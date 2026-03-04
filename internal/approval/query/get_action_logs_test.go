package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/query"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetActionLogsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// GetActionLogsTestSuite tests the GetActionLogsHandler.
type GetActionLogsTestSuite struct {
	suite.Suite

	ctx        context.Context
	db         orm.DB
	handler    *query.GetActionLogsHandler
	instanceID string
}

func (s *GetActionLogsTestSuite) SetupSuite() {
	s.handler = query.NewGetActionLogsHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "qal", 0)

	inst1 := &approval.Instance{TenantID: "default", FlowID: fix.FlowID, FlowVersionID: fix.VersionID, Title: "Log Test 1", InstanceNo: "AL-001", ApplicantID: "a1", Status: approval.InstanceRunning}
	inst2 := &approval.Instance{TenantID: "default", FlowID: fix.FlowID, FlowVersionID: fix.VersionID, Title: "Log Test 2", InstanceNo: "AL-002", ApplicantID: "a2", Status: approval.InstanceRunning}
	for _, inst := range []*approval.Instance{inst1, inst2} {
		_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
		s.Require().NoError(err)
	}
	s.instanceID = inst1.ID

	logs := []approval.ActionLog{
		{InstanceID: inst1.ID, Action: approval.ActionSubmit, OperatorID: "user-1", OperatorName: "Alice"},
		{InstanceID: inst1.ID, Action: approval.ActionApprove, OperatorID: "user-2", OperatorName: "Bob"},
		{InstanceID: inst1.ID, Action: approval.ActionReject, OperatorID: "user-3", OperatorName: "Charlie"},
		{InstanceID: inst2.ID, Action: approval.ActionSubmit, OperatorID: "user-4", OperatorName: "Dave"},
	}
	for i := range logs {
		_, err := s.db.NewInsert().Model(&logs[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test action log")
	}
}

func (s *GetActionLogsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetActionLogsTestSuite) TestGetLogsByInstance() {
	logs, err := s.handler.Handle(s.ctx, query.GetActionLogsQuery{
		InstanceID: s.instanceID,
	})
	s.Require().NoError(err, "Should query action logs without error")
	s.Assert().Len(logs, 3, "Should find 3 logs for the instance")
}

func (s *GetActionLogsTestSuite) TestOrderByCreatedAt() {
	logs, err := s.handler.Handle(s.ctx, query.GetActionLogsQuery{
		InstanceID: s.instanceID,
	})
	s.Require().NoError(err)
	s.Require().Len(logs, 3)

	// Verify ordering: logs should be ordered by created_at ascending
	s.Assert().Equal(approval.ActionSubmit, logs[0].Action, "First log should be submit")
	s.Assert().Equal(approval.ActionApprove, logs[1].Action, "Second log should be approve")
	s.Assert().Equal(approval.ActionReject, logs[2].Action, "Third log should be reject")
}

func (s *GetActionLogsTestSuite) TestNoLogs() {
	logs, err := s.handler.Handle(s.ctx, query.GetActionLogsQuery{
		InstanceID: "non-existent-instance",
	})
	s.Require().NoError(err, "Should not error for no results")
	s.Assert().Empty(logs, "Should return empty slice")
}
