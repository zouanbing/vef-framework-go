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
		return &RemoveAssigneeTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// RemoveAssigneeTestSuite tests the RemoveAssigneeHandler.
type RemoveAssigneeTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.RemoveAssigneeHandler
	fixture *MinimalFixture
	nodeID  string
}

func (s *RemoveAssigneeTestSuite) SetupSuite() {
	eng := buildTestEngine()
	taskSvc, nodeSvc, _ := buildTestServices(eng)
	pub := dispatcher.NewEventPublisher()

	s.handler = command.NewRemoveAssigneeHandler(s.db, taskSvc, nodeSvc, eng, pub)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "remove")

	node := &approval.FlowNode{
		FlowVersionID:           s.fixture.VersionID,
		Key:                     "remove-node",
		Kind:                    approval.NodeApproval,
		Name:                    "Remove Assignee Node",
		PassRule:                approval.PassAll,
		IsRemoveAssigneeAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err)
	s.nodeID = node.ID
}

func (s *RemoveAssigneeTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.ActionLog)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *RemoveAssigneeTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *RemoveAssigneeTestSuite) setupData() (*approval.Instance, *approval.Task, *approval.Task) {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Remove Assignee Test",
		InstanceNo:    "RM-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)

	task1 := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "assignee-1",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task1).Exec(s.ctx)
	s.Require().NoError(err)

	task2 := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "assignee-2",
		SortOrder:  2,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task2).Exec(s.ctx)
	s.Require().NoError(err)

	return inst, task1, task2
}

func (s *RemoveAssigneeTestSuite) TestRemoveSuccess() {
	_, _, task2 := s.setupData()

	// assignee-1 removes assignee-2 (peer operation)
	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Assignee 1"}
	_, err := s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   task2.ID,
		Operator: operator,
	})
	s.Require().NoError(err, "Should remove assignee without error")

	// Verify task2 is removed
	var updated approval.Task
	updated.ID = task2.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.TaskRemoved, updated.Status, "Task should be removed")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", task2.InstanceID) }).
		Scan(s.ctx))
	found := false
	for _, log := range logs {
		if log.Action == approval.ActionRemoveAssignee {
			found = true
		}
	}
	s.Assert().True(found, "Should have a remove_assignee action log")
}

func (s *RemoveAssigneeTestSuite) TestRemoveNotAllowed() {
	// Create node with remove disabled
	node := &approval.FlowNode{
		FlowVersionID:           s.fixture.VersionID,
		Key:                     "no-remove-node",
		Kind:                    approval.NodeApproval,
		Name:                    "No Remove Node",
		IsRemoveAssigneeAllowed: false,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err)
	defer func() {
		_, _ = s.db.NewDelete().Model(node).WherePK().Exec(s.ctx)
	}()

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "No Remove Test",
		InstanceNo:    "RM-002",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "assignee-3",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err)

	operator := approval.OperatorInfo{ID: "assignee-3", Name: "Assignee"}
	_, err = s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrRemoveAssigneeNotAllowed)
}

func (s *RemoveAssigneeTestSuite) TestRemoveTaskNotFound() {
	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Assignee"}
	_, err := s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   "non-existent",
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound)
}
