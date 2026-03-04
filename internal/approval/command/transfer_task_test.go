package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/command"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &TransferTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// TransferTaskTestSuite tests the TransferTaskHandler.
type TransferTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.TransferTaskHandler
	fixture *MinimalFixture
	nodeID  string
}

func (s *TransferTaskTestSuite) SetupSuite() {
	taskSvc := service.NewTaskService()
	pub := dispatcher.NewEventPublisher()
	s.handler = command.NewTransferTaskHandler(s.db, taskSvc, pub)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "transfer")

	node := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "transfer-node",
		Kind:              approval.NodeApproval,
		Name:              "Transfer Node",
		IsTransferAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err)
	s.nodeID = node.ID
}

func (s *TransferTaskTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.ActionLog)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *TransferTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *TransferTaskTestSuite) setupData(assigneeID string) (*approval.Instance, *approval.Task) {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Transfer Test",
		InstanceNo:    "TR-001",
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

func (s *TransferTaskTestSuite) TestTransferSuccess() {
	_, task := s.setupData("assignee-1")

	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Original Assignee"}
	_, err := s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TransferToID: "new-assignee-1",
		Opinion:      "Need expert review",
	})
	s.Require().NoError(err, "Should transfer task without error")

	// Verify original task transferred
	var original approval.Task
	original.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&original).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.TaskTransferred, original.Status, "Original task should be transferred")

	// Verify new task created for transferee
	var newTasks []approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&newTasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", task.InstanceID).
				Equals("assignee_id", "new-assignee-1")
		}).
		Scan(s.ctx))
	s.Assert().Len(newTasks, 1, "Should create one new task for transferee")
	s.Assert().Equal(approval.TaskPending, newTasks[0].Status, "New task should be pending")
}

func (s *TransferTaskTestSuite) TestTransferNotAllowed() {
	// Create node with transfer disabled
	node := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "no-transfer-node",
		Kind:              approval.NodeApproval,
		Name:              "No Transfer Node",
		IsTransferAllowed: false,
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
		Title:         "No Transfer Test",
		InstanceNo:    "TR-002",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "assignee-2",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err)

	operator := approval.OperatorInfo{ID: "assignee-2", Name: "Assignee"}
	_, err = s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TransferToID: "new-assignee",
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrTransferNotAllowed)
}

func (s *TransferTaskTestSuite) TestTransferTaskNotFound() {
	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Assignee"}
	_, err := s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       "non-existent",
		Operator:     operator,
		TransferToID: "new-assignee",
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound)
}

func (s *TransferTaskTestSuite) TestTransferNotAssignee() {
	_, task := s.setupData("assignee-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TransferToID: "new-assignee",
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrNotAssignee)
}
