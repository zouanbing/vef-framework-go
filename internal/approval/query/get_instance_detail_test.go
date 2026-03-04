package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetInstanceDetailTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// GetInstanceDetailTestSuite tests the GetInstanceDetailHandler.
type GetInstanceDetailTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.GetInstanceDetailHandler

	instanceID    string
	flowVersionID string
}

func (s *GetInstanceDetailTestSuite) SetupSuite() {
	s.handler = query.NewGetInstanceDetailHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "qid", 0)
	s.flowVersionID = fix.VersionID

	// Create flow nodes
	nodes := []approval.FlowNode{
		{FlowVersionID: fix.VersionID, Key: "start-1", Kind: approval.NodeStart, Name: "Start"},
		{FlowVersionID: fix.VersionID, Key: "approval-1", Kind: approval.NodeApproval, Name: "Approval"},
		{FlowVersionID: fix.VersionID, Key: "end-1", Kind: approval.NodeEnd, Name: "End"},
	}
	for i := range nodes {
		_, err := s.db.NewInsert().Model(&nodes[i]).Exec(s.ctx)
		s.Require().NoError(err)
	}

	// Create instance
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "Detail Test Instance",
		InstanceNo:    "DT-001",
		ApplicantID:   "user-1",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err)
	s.instanceID = instance.ID

	// Create tasks
	tasks := []approval.Task{
		{TenantID: "default", InstanceID: instance.ID, NodeID: nodes[1].ID, AssigneeID: "user-2", SortOrder: 1, Status: approval.TaskPending},
		{TenantID: "default", InstanceID: instance.ID, NodeID: nodes[1].ID, AssigneeID: "user-3", SortOrder: 2, Status: approval.TaskPending},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err)
	}

	// Create action logs
	logs := []approval.ActionLog{
		{InstanceID: instance.ID, Action: approval.ActionSubmit, OperatorID: "user-1", OperatorName: "Applicant"},
	}
	for i := range logs {
		_, err := s.db.NewInsert().Model(&logs[i]).Exec(s.ctx)
		s.Require().NoError(err)
	}
}

func (s *GetInstanceDetailTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetInstanceDetailTestSuite) TestGetDetailSuccess() {
	detail, err := s.handler.Handle(s.ctx, query.GetInstanceDetailQuery{
		InstanceID: s.instanceID,
	})
	s.Require().NoError(err, "Should get instance detail without error")
	s.Require().NotNil(detail)

	s.Assert().Equal(s.instanceID, detail.Instance.ID, "Should return correct instance")
	s.Assert().Equal("Detail Test Instance", detail.Instance.Title, "Should return correct title")
	s.Assert().Len(detail.Tasks, 2, "Should return 2 tasks")
	s.Assert().Len(detail.ActionLogs, 1, "Should return 1 action log")
	s.Assert().Len(detail.FlowNodes, 3, "Should return 3 flow nodes")
}

func (s *GetInstanceDetailTestSuite) TestTasksSortedBySortOrder() {
	detail, err := s.handler.Handle(s.ctx, query.GetInstanceDetailQuery{
		InstanceID: s.instanceID,
	})
	s.Require().NoError(err)
	s.Require().Len(detail.Tasks, 2)

	s.Assert().Equal(1, detail.Tasks[0].SortOrder, "First task should have sort_order 1")
	s.Assert().Equal(2, detail.Tasks[1].SortOrder, "Second task should have sort_order 2")
}

func (s *GetInstanceDetailTestSuite) TestInstanceNotFound() {
	_, err := s.handler.Handle(s.ctx, query.GetInstanceDetailQuery{
		InstanceID: "non-existent-instance",
	})
	s.Require().Error(err, "Should error for non-existent instance")
}
