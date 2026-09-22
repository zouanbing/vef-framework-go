package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindMyCompletedTasksTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindMyCompletedTasksTestSuite tests the FindMyCompletedTasksHandler.
type FindMyCompletedTasksTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindMyCompletedTasksHandler
	// purchaseFlowID is the flow of the filter fixture's second instance.
	purchaseFlowID string
}

func (s *FindMyCompletedTasksTestSuite) SetupSuite() {
	s.handler = query.NewFindMyCompletedTasksHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mct-flow", 1)

	inst := &approval.Instance{
		TenantID:      "t1",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "Completed Instance",
		InstanceNo:    "MCT-001",
		ApplicantID:   "user-x",
		Status:        approval.InstanceApproved,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")

	_, err = s.db.NewUpdate().Model(new(approval.Flow)).
		Set("labels", map[string]string{"app": "smp"}).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", fix.FlowID)
		}).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set flow labels")

	now := timex.Now()

	tasks := []approval.Task{
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 1, Status: approval.TaskApproved, FinishedAt: &now},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 2, Status: approval.TaskRejected, FinishedAt: &now},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 3, Status: approval.TaskHandled, FinishedAt: &now},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 4, Status: approval.TaskRolledBack, FinishedAt: &now},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 5, Status: approval.TaskPending},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-b", SortOrder: 6, Status: approval.TaskApproved, FinishedAt: &now},
	}
	for i := range tasks {
		tasks[i].VisitID = ensureActiveVisit(s.T(), s.ctx, s.db, tasks[i].TenantID, tasks[i].InstanceID, tasks[i].NodeID).ID
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	s.seedFilterFixture(fix)
}

// seedFilterFixture gives user-f one completed task on each of two instances
// that differ in every filtered attribute, so each filter keeps exactly one.
// A still-pending task on the second instance must never surface. A separate
// assignee keeps the user-a assertions exact.
func (s *FindMyCompletedTasksTestSuite) seedFilterFixture(travelFix *QueryFixture) {
	purchaseFix := setupQueryFixture(s.T(), s.ctx, s.db, "mct-flow-b", 1)
	s.purchaseFlowID = purchaseFix.FlowID

	travel := &approval.Instance{
		TenantID:      "t1",
		FlowID:        travelFix.FlowID,
		FlowVersionID: travelFix.VersionID,
		Title:         "Travel reimbursement",
		InstanceNo:    "MCT-101",
		ApplicantID:   "user-p",
		ApplicantName: "Alice Wang",
		Status:        approval.InstanceApproved,
	}
	purchase := &approval.Instance{
		TenantID:      "t1",
		FlowID:        purchaseFix.FlowID,
		FlowVersionID: purchaseFix.VersionID,
		Title:         "Purchase request",
		InstanceNo:    "MCT-102",
		ApplicantID:   "user-q",
		ApplicantName: "Bob Li",
		Status:        approval.InstanceRunning,
	}

	for _, inst := range []*approval.Instance{travel, purchase} {
		_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
		s.Require().NoError(err, "Should insert filter fixture instance")
	}

	travelFinishedAt, purchaseFinishedAt := septemberAt(1, 10), septemberAt(10, 10)

	insertTask(s.T(), s.ctx, s.db, &approval.Task{
		TenantID: "t1", InstanceID: travel.ID, NodeID: travelFix.NodeIDs[0], AssigneeID: "user-f",
		SortOrder: 1, Status: approval.TaskApproved, FinishedAt: &travelFinishedAt,
	})
	insertTask(s.T(), s.ctx, s.db, &approval.Task{
		TenantID: "t1", InstanceID: purchase.ID, NodeID: purchaseFix.NodeIDs[0], AssigneeID: "user-f",
		SortOrder: 1, Status: approval.TaskTransferred, FinishedAt: &purchaseFinishedAt,
	})
	insertTask(s.T(), s.ctx, s.db, &approval.Task{
		TenantID: "t1", InstanceID: purchase.ID, NodeID: purchaseFix.NodeIDs[0], AssigneeID: "user-f",
		SortOrder: 2, Status: approval.TaskPending,
	})
}

func (s *FindMyCompletedTasksTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindMyCompletedTasksTestSuite) TestFindCompletedForUser() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Should include approved, rejected, handled, and rolled-back tasks for user-a")
}

func (s *FindMyCompletedTasksTestSuite) TestExcludesPendingTasks() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")

	for _, item := range result.Items {
		s.Assert().NotEqual("pending", item.Status, "Should not include pending tasks")
	}
}

func (s *FindMyCompletedTasksTestSuite) TestProjectsInstanceStatus() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().Len(result.Items, 4, "All four completed tasks should be projected")

	for _, item := range result.Items {
		s.Assert().Equal(approval.InstanceApproved, item.InstanceStatus,
			"Each row should carry the instance's current status")
	}
}

func (s *FindMyCompletedTasksTestSuite) TestProjectsFlowLabels() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().NotEmpty(result.Items, "Completed tasks should be projected")

	for _, item := range result.Items {
		s.Assert().Equal(map[string]string{"app": "smp"}, item.Labels,
			"Each row should carry the flow's labels")
	}
}

func (s *FindMyCompletedTasksTestSuite) TestFilters() {
	const travel, purchase = "Travel reimbursement", "Purchase request"

	// titles runs the query for user-f and returns the matched instance titles,
	// checking that the count agrees with the rows the join returned.
	titles := func(q query.FindMyCompletedTasksQuery) []string {
		q.UserID, q.Page, q.Size = "user-f", 1, 10

		result, err := s.handler.Handle(s.ctx, q)
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(len(result.Items)), result.Total, "Total should count exactly the filtered rows")

		matched := make([]string, len(result.Items))
		for i, item := range result.Items {
			matched[i] = item.InstanceTitle
		}

		return matched
	}

	s.Run("NoFilter", func() {
		s.ElementsMatch([]string{travel, purchase}, titles(query.FindMyCompletedTasksQuery{}),
			"Joining the instance should keep every completed task and still drop the pending one")
	})

	s.Run("Keyword", func() {
		s.Equal([]string{purchase}, titles(query.FindMyCompletedTasksQuery{Keyword: new("request")}),
			"Keyword should match the instance title by substring")
	})

	s.Run("ApplicantID", func() {
		s.Equal([]string{travel}, titles(query.FindMyCompletedTasksQuery{ApplicantID: new("user-p")}),
			"ApplicantID should match the applicant exactly")
	})

	s.Run("ApplicantName", func() {
		s.Equal([]string{purchase}, titles(query.FindMyCompletedTasksQuery{ApplicantName: new("Bob")}),
			"ApplicantName should match the applicant name by substring")
	})

	s.Run("FlowID", func() {
		s.Equal([]string{purchase}, titles(query.FindMyCompletedTasksQuery{FlowID: new(s.purchaseFlowID)}),
			"FlowID should match the instance's flow")
	})

	s.Run("Status", func() {
		s.Equal([]string{travel}, titles(query.FindMyCompletedTasksQuery{Status: new(approval.TaskApproved)}),
			"Status should match how the caller finished the task")
	})

	s.Run("NonCompletedStatusMatchesNothing", func() {
		s.Empty(titles(query.FindMyCompletedTasksQuery{Status: new(approval.TaskPending)}),
			"Status should narrow within the completed statuses, never widen to pending tasks")
	})

	s.Run("InstanceStatus", func() {
		s.Equal([]string{purchase}, titles(query.FindMyCompletedTasksQuery{InstanceStatus: new(approval.InstanceRunning)}),
			"InstanceStatus should match the instance's current status")
	})

	s.Run("FinishedAtRange", func() {
		s.Equal([]string{purchase}, titles(query.FindMyCompletedTasksQuery{FinishedAtFrom: new(septemberAt(5, 0))}),
			"FinishedAtFrom should drop tasks finished earlier")
		s.Equal([]string{travel}, titles(query.FindMyCompletedTasksQuery{FinishedAtTo: new(septemberAt(5, 0))}),
			"FinishedAtTo should drop tasks finished later")
	})

	s.Run("FinishedAtBoundsAreInclusive", func() {
		finishedAt := septemberAt(1, 10)

		s.Equal([]string{travel}, titles(query.FindMyCompletedTasksQuery{FinishedAtFrom: &finishedAt, FinishedAtTo: &finishedAt}),
			"A task finished exactly on both bounds should be kept")
	})

	s.Run("FiltersCombineWithAnd", func() {
		s.Empty(titles(query.FindMyCompletedTasksQuery{Keyword: new("request"), Status: new(approval.TaskApproved)}),
			"Filters matching different tasks should together match none")
	})
}

func (s *FindMyCompletedTasksTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "non-existent-user",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 completed tasks")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
