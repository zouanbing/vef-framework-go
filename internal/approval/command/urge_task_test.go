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
	"github.com/ilxqx/vef-framework-go/result"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &UrgeTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// UrgeTaskTestSuite tests the UrgeTaskHandler.
type UrgeTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.UrgeTaskHandler
	fixture *MinimalFixture
	nodeID  string
	instID  string
}

func (s *UrgeTaskTestSuite) SetupSuite() {
	s.handler = command.NewUrgeTaskHandler(s.db, dispatcher.NewEventPublisher())
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "urge")

	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "urge-node",
		Kind:          approval.NodeApproval,
		Name:          "Urge Test Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err)
	s.nodeID = node.ID

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Urge Test",
		InstanceNo:    "URG-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)
	s.instID = inst.ID
}

func (s *UrgeTaskTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.UrgeRecord)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *UrgeTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *UrgeTaskTestSuite) insertTask(assigneeID string) *approval.Task {
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: s.instID,
		NodeID:     s.nodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err := s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err)
	return task
}

func (s *UrgeTaskTestSuite) TestUrgeSuccess() {
	task := s.insertTask("assignee-1")

	_, err := s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  task.ID,
		UrgerID: "manager-1",
		Message: "Please review ASAP",
	})
	s.Require().NoError(err, "Should urge task without error")

	// Verify urge record created
	var records []approval.UrgeRecord
	s.Require().NoError(s.db.NewSelect().Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("task_id", task.ID) }).
		Scan(s.ctx))
	s.Assert().Len(records, 1, "Should create one urge record")

	// Verify event published
	var events []approval.EventOutbox
	s.Require().NoError(s.db.NewSelect().Model(&events).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.task.urged") }).
		Scan(s.ctx))
	s.Assert().Len(events, 1, "Should publish one urge event")
}

func (s *UrgeTaskTestSuite) TestUrgeCooldown() {
	task := s.insertTask("assignee-2")

	// First urge
	_, err := s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  task.ID,
		UrgerID: "manager-1",
	})
	s.Require().NoError(err, "First urge should succeed")

	// Immediate second urge - should fail due to cooldown
	_, err = s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  task.ID,
		UrgerID: "manager-1",
	})
	s.Require().Error(err, "Second urge should fail")
	var re result.Error
	s.Require().ErrorAs(err, &re, "Should be a result.Error")
	s.Assert().Equal(shared.ErrCodeUrgeCooldown, re.Code, "Should return ErrCodeUrgeCooldown")
}

func (s *UrgeTaskTestSuite) TestUrgeTaskNotFound() {
	_, err := s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  "non-existent",
		UrgerID: "manager-1",
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return ErrTaskNotFound")
}
