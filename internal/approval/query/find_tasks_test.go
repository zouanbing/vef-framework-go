package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/query"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/page"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindTasksTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindTasksTestSuite tests the FindTasksHandler.
type FindTasksTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindTasksHandler
	instIDs []string
}

func (s *FindTasksTestSuite) SetupSuite() {
	s.handler = query.NewFindTasksHandler(s.db)

	// Create 2 fixtures for 2 tenants
	fix1 := setupQueryFixture(s.T(), s.ctx, s.db, "qt-t1", 2)
	fix2 := setupQueryFixture(s.T(), s.ctx, s.db, "qt-t2", 1)

	// Create instances
	inst1 := &approval.Instance{TenantID: "t1", FlowID: fix1.FlowID, FlowVersionID: fix1.VersionID, Title: "T1", InstanceNo: "QT-001", ApplicantID: "a1", Status: approval.InstanceRunning}
	inst2 := &approval.Instance{TenantID: "t1", FlowID: fix1.FlowID, FlowVersionID: fix1.VersionID, Title: "T2", InstanceNo: "QT-002", ApplicantID: "a1", Status: approval.InstanceRunning}
	inst3 := &approval.Instance{TenantID: "t2", FlowID: fix2.FlowID, FlowVersionID: fix2.VersionID, Title: "T3", InstanceNo: "QT-003", ApplicantID: "a2", Status: approval.InstanceRunning}
	for _, inst := range []*approval.Instance{inst1, inst2, inst3} {
		_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
		s.Require().NoError(err)
	}
	s.instIDs = []string{inst1.ID, inst2.ID, inst3.ID}

	tasks := []approval.Task{
		{TenantID: "t1", InstanceID: inst1.ID, NodeID: fix1.NodeIDs[0], AssigneeID: "user-1", SortOrder: 1, Status: approval.TaskPending},
		{TenantID: "t1", InstanceID: inst1.ID, NodeID: fix1.NodeIDs[0], AssigneeID: "user-2", SortOrder: 2, Status: approval.TaskApproved},
		{TenantID: "t1", InstanceID: inst2.ID, NodeID: fix1.NodeIDs[1], AssigneeID: "user-1", SortOrder: 1, Status: approval.TaskPending},
		{TenantID: "t2", InstanceID: inst3.ID, NodeID: fix2.NodeIDs[0], AssigneeID: "user-3", SortOrder: 1, Status: approval.TaskRejected},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test task")
	}
}

func (s *FindTasksTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindTasksTestSuite) TestFindAll() {
	result, err := s.handler.Handle(s.ctx, query.FindTasksQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err)
	s.Assert().Equal(int64(4), result.Total, "Should find all 4 tasks")
	s.Assert().Len(result.Items, 4)
}

func (s *FindTasksTestSuite) TestFilterByTenant() {
	result, err := s.handler.Handle(s.ctx, query.FindTasksQuery{
		TenantID: "t1",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err)
	s.Assert().Equal(int64(3), result.Total, "Should find 3 tasks for tenant t1")
}

func (s *FindTasksTestSuite) TestFilterByAssignee() {
	result, err := s.handler.Handle(s.ctx, query.FindTasksQuery{
		AssigneeID: "user-1",
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err)
	s.Assert().Equal(int64(2), result.Total, "Should find 2 tasks for user-1")
}

func (s *FindTasksTestSuite) TestFilterByInstance() {
	result, err := s.handler.Handle(s.ctx, query.FindTasksQuery{
		InstanceID: s.instIDs[0],
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err)
	s.Assert().Equal(int64(2), result.Total, "Should find 2 tasks for inst-1")
}

func (s *FindTasksTestSuite) TestFilterByStatus() {
	result, err := s.handler.Handle(s.ctx, query.FindTasksQuery{
		Status:   string(approval.TaskPending),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err)
	s.Assert().Equal(int64(2), result.Total, "Should find 2 pending tasks")
}

func (s *FindTasksTestSuite) TestCombinedFilters() {
	result, err := s.handler.Handle(s.ctx, query.FindTasksQuery{
		TenantID:   "t1",
		AssigneeID: "user-1",
		Status:     string(approval.TaskPending),
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err)
	s.Assert().Equal(int64(2), result.Total, "Should find 2 matching tasks")
}

func (s *FindTasksTestSuite) TestPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindTasksQuery{
		Pageable: page.Pageable{Page: 1, Size: 2},
	})
	s.Require().NoError(err)
	s.Assert().Equal(int64(4), result.Total)
	s.Assert().Len(result.Items, 2, "Page 1 should return 2 records")
}

func (s *FindTasksTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindTasksQuery{
		TenantID: "non-existent",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), result.Total)
	s.Assert().Empty(result.Items)
}
