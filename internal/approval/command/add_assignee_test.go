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
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &AddAssigneeTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// AddAssigneeTestSuite tests the AddAssigneeHandler.
type AddAssigneeTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.AddAssigneeHandler
	fixture *MinimalFixture
	nodeID  string
}

func (s *AddAssigneeTestSuite) SetupSuite() {
	s.handler = command.NewAddAssigneeHandler(s.db, dispatcher.NewEventPublisher())
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "add-assignee")

	node := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "add-assignee-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Add Assignee Node",
		IsAddAssigneeAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err)
	s.nodeID = node.ID
}

func (s *AddAssigneeTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.ActionLog)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *AddAssigneeTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *AddAssigneeTestSuite) setupData(assigneeID string) (*approval.Instance, *approval.Task) {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Add Assignee Test",
		InstanceNo:    "AA-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err)

	return inst, task
}

func (s *AddAssigneeTestSuite) TestAddAssigneeSuccess() {
	_, task := s.setupData("operator-1")

	operator := approval.OperatorInfo{ID: "operator-1", Name: "Operator"}
	_, err := s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1", "new-user-2"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().NoError(err, "Should add assignees without error")

	// Verify new tasks created
	var tasks []approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", task.InstanceID) }).
		OrderBy("sort_order").
		Scan(s.ctx))
	s.Assert().GreaterOrEqual(len(tasks), 3, "Should have at least 3 tasks (1 original + 2 new)")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeNotAllowed() {
	// Create node with add-assignee disabled
	node := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "no-add-node",
		Kind:                 approval.NodeApproval,
		Name:                 "No Add Node",
		IsAddAssigneeAllowed: false,
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
		Title:         "No Add Test",
		InstanceNo:    "AA-002",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "operator-2",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err)

	operator := approval.OperatorInfo{ID: "operator-2", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrAddAssigneeNotAllowed, "Should return ErrAddAssigneeNotAllowed")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeNotAssignee() {
	_, task := s.setupData("operator-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should return ErrNotAssignee")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeTaskNotFound() {
	operator := approval.OperatorInfo{ID: "operator-1", Name: "Operator"}
	_, err := s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   "non-existent",
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return ErrTaskNotFound")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeInstanceCompleted() {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Completed Instance",
		InstanceNo:    "AA-003",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceApproved,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "operator-3",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err)

	operator := approval.OperatorInfo{ID: "operator-3", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrInstanceCompleted, "Should return ErrInstanceCompleted")
}
