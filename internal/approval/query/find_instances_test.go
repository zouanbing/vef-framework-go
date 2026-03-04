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
		return &FindInstancesTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindInstancesTestSuite tests the FindInstancesHandler.
type FindInstancesTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindInstancesHandler
	fix1    *QueryFixture
	fix2    *QueryFixture
	fix3    *QueryFixture
}

func (s *FindInstancesTestSuite) SetupSuite() {
	s.handler = query.NewFindInstancesHandler(s.db)

	s.fix1 = setupQueryFixture(s.T(), s.ctx, s.db, "qi-flow1", 0)
	s.fix2 = setupQueryFixture(s.T(), s.ctx, s.db, "qi-flow2", 0)
	s.fix3 = setupQueryFixture(s.T(), s.ctx, s.db, "qi-flow3", 0)

	instances := []approval.Instance{
		{TenantID: "t1", FlowID: s.fix1.FlowID, FlowVersionID: s.fix1.VersionID, Title: "Leave Request A", InstanceNo: "NO-001", ApplicantID: "user-1", Status: approval.InstanceRunning},
		{TenantID: "t1", FlowID: s.fix1.FlowID, FlowVersionID: s.fix1.VersionID, Title: "Leave Request B", InstanceNo: "NO-002", ApplicantID: "user-2", Status: approval.InstanceApproved},
		{TenantID: "t1", FlowID: s.fix2.FlowID, FlowVersionID: s.fix2.VersionID, Title: "Expense Report C", InstanceNo: "NO-003", ApplicantID: "user-1", Status: approval.InstanceRunning},
		{TenantID: "t2", FlowID: s.fix3.FlowID, FlowVersionID: s.fix3.VersionID, Title: "Purchase Order D", InstanceNo: "NO-004", ApplicantID: "user-3", Status: approval.InstanceRejected},
	}
	for i := range instances {
		_, err := s.db.NewInsert().Model(&instances[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test instance")
	}
}

func (s *FindInstancesTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindInstancesTestSuite) TestFindAll() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(4), result.Total, "Should find all 4 instances")
	s.Assert().Len(result.Items, 4, "Should return 4 records")
}

func (s *FindInstancesTestSuite) TestFilterByTenant() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		TenantID: "t1",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 instances for tenant t1")
}

func (s *FindInstancesTestSuite) TestFilterByApplicant() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		ApplicantID: "user-1",
		Pageable:    page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 instances for user-1")
}

func (s *FindInstancesTestSuite) TestFilterByStatus() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		Status:   string(approval.InstanceRunning),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 running instances")
}

func (s *FindInstancesTestSuite) TestFilterByFlowID() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		FlowID:   s.fix1.FlowID,
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 instances for flow-1")
}

func (s *FindInstancesTestSuite) TestFilterByKeyword() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		Keyword:  "Expense",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 instance matching 'Expense'")
	s.Assert().Equal("Expense Report C", result.Items[0].Title, "Should match the correct instance")
}

func (s *FindInstancesTestSuite) TestPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		Pageable: page.Pageable{Page: 1, Size: 2},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(4), result.Total, "Total should be 4")
	s.Assert().Len(result.Items, 2, "Page 1 should return 2 records")

	result2, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		Pageable: page.Pageable{Page: 2, Size: 2},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Len(result2.Items, 2, "Page 2 should return 2 records")
}

func (s *FindInstancesTestSuite) TestCombinedFilters() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		TenantID:    "t1",
		ApplicantID: "user-1",
		Status:      string(approval.InstanceRunning),
		Pageable:    page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 running instances for user-1 in t1")
}

func (s *FindInstancesTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindInstancesQuery{
		TenantID: "non-existent-tenant",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query instances without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 instances")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
