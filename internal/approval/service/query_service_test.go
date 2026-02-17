package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/page"
)

type QueryServiceTestSuite struct {
	suite.Suite
	ctx     context.Context
	db      orm.DB
	svc     *QueryService
	instSvc *InstanceService
	flowSvc *FlowService
	cleanup func()
}

// TestQueryServiceTestSuite tests query service test suite scenarios.
func TestQueryServiceTestSuite(t *testing.T) {
	suite.Run(t, new(QueryServiceTestSuite))
}

func (s *QueryServiceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.db, s.cleanup = setupTestDB(s.T())
	s.svc = NewQueryService(s.db)

	pub := publisher.NewEventPublisher()
	mockOrg := &MockOrganizationService{}
	mockUser := &MockUserService{}
	eng := setupEngine(mockOrg, mockUser, pub)
	s.instSvc = NewInstanceService(s.db, eng, NewMockSerialNoGenerator(), pub, mockUser)
	s.flowSvc = NewFlowService(s.db, pub)
}

func (s *QueryServiceTestSuite) TearDownTest() {
	s.cleanup()
}

func (s *QueryServiceTestSuite) startFlowAndGetInstance(applicantID, title string) *approval.Instance {
	buildSimpleFlow(s.T(), s.ctx, s.db)

	instance, err := s.instSvc.StartInstance(s.ctx, StartInstanceCmd{
		FlowCode:    "simple_flow",
		Title:       title,
		ApplicantID: applicantID,
		FormData:    map[string]any{"reason": "test"},
	})
	s.Require().NoError(err, "Should not return error")

	return instance
}

func (s *QueryServiceTestSuite) TestFindInstancesNoFilter() {
	instance := s.startFlowAndGetInstance("applicant1", "Test Instance")

	results, count, err := s.svc.FindInstances(s.ctx, InstanceQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "FindInstances with no filter should succeed")
	s.Equal(1, count, "Should find 1 instance")
	s.Require().Len(results, 1, "Length should match expected value")
	s.Equal(instance.ID, results[0].ID)
}

func (s *QueryServiceTestSuite) TestFindInstancesFilterByApplicantID() {
	s.startFlowAndGetInstance("applicant1", "Instance A")

	results, count, err := s.svc.FindInstances(s.ctx, InstanceQuery{
		ApplicantID: "applicant1",
		Pageable:    page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(1, count)
	s.Require().Len(results, 1, "Length should match expected value")
	s.Equal("applicant1", results[0].ApplicantID)

	results, count, err = s.svc.FindInstances(s.ctx, InstanceQuery{
		ApplicantID: "nonexistent",
		Pageable:    page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(0, count)
	s.Empty(results)
}

func (s *QueryServiceTestSuite) TestFindInstancesFilterByStatus() {
	s.startFlowAndGetInstance("applicant1", "Running Instance")

	results, count, err := s.svc.FindInstances(s.ctx, InstanceQuery{
		Status:   string(approval.InstanceRunning),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(1, count)
	s.Equal(approval.InstanceRunning, results[0].Status)

	results, count, err = s.svc.FindInstances(s.ctx, InstanceQuery{
		Status:   string(approval.InstanceRejected),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(0, count)
	s.Empty(results)
}

func (s *QueryServiceTestSuite) TestFindInstancesFilterByFlowID() {
	instance := s.startFlowAndGetInstance("applicant1", "Flow Filter Test")

	results, count, err := s.svc.FindInstances(s.ctx, InstanceQuery{
		FlowID:   instance.FlowID,
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(1, count)
	s.Equal(instance.FlowID, results[0].FlowID)

	_, count, err = s.svc.FindInstances(s.ctx, InstanceQuery{
		FlowID:   "nonexistent_flow_id",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(0, count)
}

func (s *QueryServiceTestSuite) TestFindInstancesFilterByKeyword() {
	s.startFlowAndGetInstance("applicant1", "Leave Application")

	results, count, err := s.svc.FindInstances(s.ctx, InstanceQuery{
		Keyword:  "Leave",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(1, count)
	s.Contains(results[0].Title, "Leave")

	_, count, err = s.svc.FindInstances(s.ctx, InstanceQuery{
		Keyword:  "zzz_nonexistent",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(0, count)
}

func (s *QueryServiceTestSuite) TestFindInstancesPagination() {
	s.startFlowAndGetInstance("applicant1", "Page Test")

	results, count, err := s.svc.FindInstances(s.ctx, InstanceQuery{
		Pageable: page.Pageable{Page: 1, Size: 1},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(1, count)
	s.Len(results, 1)

	results, _, err = s.svc.FindInstances(s.ctx, InstanceQuery{
		Pageable: page.Pageable{Page: 2, Size: 1},
	})
	s.Require().NoError(err, "Should not return error")
	s.Empty(results)
}

func (s *QueryServiceTestSuite) TestFindInstancesEmptyResult() {
	results, count, err := s.svc.FindInstances(s.ctx, InstanceQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(0, count)
	s.Empty(results)
}

func (s *QueryServiceTestSuite) TestFindTasksNoFilter() {
	s.startFlowAndGetInstance("applicant1", "Task Query Test")

	results, count, err := s.svc.FindTasks(s.ctx, TaskQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "FindTasks with no filter should succeed")
	s.True(count > 0, "Should find at least one task")
	s.NotEmpty(results)
}

func (s *QueryServiceTestSuite) TestFindTasksFilterByAssigneeID() {
	s.startFlowAndGetInstance("applicant1", "Assignee Filter")

	results, count, err := s.svc.FindTasks(s.ctx, TaskQuery{
		AssigneeID: "user1",
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.True(count > 0)
	for _, task := range results {
		s.Equal("user1", task.AssigneeID)
	}

	_, count, err = s.svc.FindTasks(s.ctx, TaskQuery{
		AssigneeID: "nonexistent",
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(0, count)
}

func (s *QueryServiceTestSuite) TestFindTasksFilterByInstanceID() {
	instance := s.startFlowAndGetInstance("applicant1", "Instance Filter")

	results, count, err := s.svc.FindTasks(s.ctx, TaskQuery{
		InstanceID: instance.ID,
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.True(count > 0)
	for _, task := range results {
		s.Equal(instance.ID, task.InstanceID)
	}
}

func (s *QueryServiceTestSuite) TestFindTasksFilterByStatus() {
	s.startFlowAndGetInstance("applicant1", "Status Filter")

	results, count, err := s.svc.FindTasks(s.ctx, TaskQuery{
		Status:   string(approval.TaskPending),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.True(count > 0)
	for _, task := range results {
		s.Equal(approval.TaskPending, task.Status)
	}
}

func (s *QueryServiceTestSuite) TestFindTasksEmptyResult() {
	results, count, err := s.svc.FindTasks(s.ctx, TaskQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should not return error")
	s.Equal(0, count)
	s.Empty(results)
}

func (s *QueryServiceTestSuite) TestGetInstanceDetailSuccess() {
	instance := s.startFlowAndGetInstance("applicant1", "Detail Test")

	detail, err := s.svc.GetInstanceDetail(s.ctx, instance.ID)
	s.Require().NoError(err, "GetInstanceDetail should succeed")
	s.Require().NotNil(detail, "Should not be nil")

	s.Equal(instance.ID, detail.Instance.ID)
	s.Equal("Detail Test", detail.Instance.Title)

	s.NotEmpty(detail.Tasks, "Should have tasks")
	s.NotEmpty(detail.ActionLogs, "Should have action logs")
	s.NotEmpty(detail.FlowNodes, "Should have flow nodes")
}

func (s *QueryServiceTestSuite) TestGetInstanceDetailNonexistentInstance() {
	_, err := s.svc.GetInstanceDetail(s.ctx, "nonexistent_id")
	s.Require().Error(err, "Should return error for nonexistent instance")
	s.Contains(err.Error(), "query instance")
}

func (s *QueryServiceTestSuite) TestGetActionLogsSuccess() {
	instance := s.startFlowAndGetInstance("applicant1", "Action Log Test")

	logs, err := s.svc.GetActionLogs(s.ctx, instance.ID)
	s.Require().NoError(err, "GetActionLogs should succeed")
	s.NotEmpty(logs, "Should have at least the submit action log")
	s.Equal(approval.ActionSubmit, logs[0].Action)
}

func (s *QueryServiceTestSuite) TestGetActionLogsEmptyResult() {
	logs, err := s.svc.GetActionLogs(s.ctx, id.Generate())
	s.Require().NoError(err, "Should not return error")
	s.Empty(logs)
}

// TestQueryServiceQueryErrors tests query service query errors scenarios.
func TestQueryServiceQueryErrors(t *testing.T) {
	tests := []struct {
		name        string
		dropTable   string
		queryFunc   func(svc *QueryService, ctx context.Context) error
		errContains string
	}{
		{
			name:      "FindInstances",
			dropTable: "apv_instance",
			queryFunc: func(svc *QueryService, ctx context.Context) error {
				_, _, err := svc.FindInstances(ctx, InstanceQuery{})
				return err
			},
			errContains: "query instances",
		},
		{
			name:      "FindTasks",
			dropTable: "apv_task",
			queryFunc: func(svc *QueryService, ctx context.Context) error {
				_, _, err := svc.FindTasks(ctx, TaskQuery{})
				return err
			},
			errContains: "query tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db, cleanup := setupTestDB(t)
			defer cleanup()

			_, err := db.NewRaw("DROP TABLE " + tt.dropTable).Exec(ctx)
			require.NoError(t, err, "Should drop table")

			err = tt.queryFunc(NewQueryService(db), ctx)
			require.Error(t, err, "Should fail with dropped table")
			assert.Contains(t, err.Error(), tt.errContains, "Error should contain expected message")
		})
	}
}
