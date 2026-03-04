package engine_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ApprovalProcessorTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// ApprovalProcessorTestSuite tests ApprovalProcessor with a real database.
type ApprovalProcessorTestSuite struct {
	suite.Suite

	ctx       context.Context
	db        orm.DB
	processor *engine.ApprovalProcessor
	registry  *strategy.StrategyRegistry

	flowID        string
	flowVersionID string
	nodeID        string
}

func (s *ApprovalProcessorTestSuite) SetupSuite() {
	s.registry = strategy.NewStrategyRegistry(nil, []strategy.AssigneeResolver{strategy.NewUserAssigneeResolver()}, nil)
	s.processor = engine.NewApprovalProcessor(nil)

	// Build FK chain: FlowCategory → Flow → FlowVersion → FlowNode
	category := &approval.FlowCategory{TenantID: "default", Code: "approval-test", Name: "Approval Test"}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:              "default",
		CategoryID:            category.ID,
		Code:                  "approval-test-flow",
		Name:                  "Approval Test Flow",
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "test",
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow")
	s.flowID = flow.ID

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionDraft}
	_, err = s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow version")
	s.flowVersionID = version.ID

	node := &approval.FlowNode{
		FlowVersionID:           version.ID,
		Key:                     "approval-node-1",
		Kind:                    approval.NodeApproval,
		Name:                    "Approval Node",
		DuplicateAssigneeAction: approval.DuplicateAssigneeAutoPass,
	}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow node")
	s.nodeID = node.ID
}

// cleanTransientData removes all transient test data (respect FK order).
func (s *ApprovalProcessorTestSuite) cleanTransientData() {
	_, err := s.db.NewDelete().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean tasks")

	_, err = s.db.NewDelete().
		Model((*approval.FormSnapshot)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean form snapshots")

	_, err = s.db.NewDelete().
		Model((*approval.FlowNodeAssignee)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean assignee configs")

	_, err = s.db.NewDelete().
		Model((*approval.Instance)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean instances")
}

func (s *ApprovalProcessorTestSuite) TearDownTest() {
	s.cleanTransientData()
}

// newInstance creates and inserts a test instance, returning it with its generated ID.
func (s *ApprovalProcessorTestSuite) newInstance(applicantID string) *approval.Instance {
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.flowID,
		FlowVersionID: s.flowVersionID,
		Title:         "Approval Test Instance",
		InstanceNo:    "APV-001",
		ApplicantID:   applicantID,
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test instance")

	return instance
}

// newNode builds a FlowNode value with the suite's nodeID and overridable fields.
func (s *ApprovalProcessorTestSuite) newNode(opts ...func(*approval.FlowNode)) *approval.FlowNode {
	node := &approval.FlowNode{
		DuplicateAssigneeAction: approval.DuplicateAssigneeAutoPass,
	}
	node.ID = s.nodeID

	for _, opt := range opts {
		opt(node)
	}

	return node
}

func (s *ApprovalProcessorTestSuite) insertAssigneeConfig(userIDs []string) {
	cfg := &approval.FlowNodeAssignee{
		NodeID:    s.nodeID,
		Kind:      approval.AssigneeUser,
		IDs:       userIDs,
		SortOrder: 1,
	}
	_, err := s.db.NewInsert().Model(cfg).Exec(s.ctx)
	s.Require().NoError(err, "Should insert assignee config")
}

func (s *ApprovalProcessorTestSuite) queryTasks(instanceID string) []approval.Task {
	var tasks []approval.Task
	err := s.db.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("sort_order").
		Scan(s.ctx)
	s.Require().NoError(err, "Should query tasks")

	return tasks
}

func (s *ApprovalProcessorTestSuite) queryFormSnapshots(instanceID string) []approval.FormSnapshot {
	var snapshots []approval.FormSnapshot
	err := s.db.NewSelect().
		Model(&snapshots).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query form snapshots")

	return snapshots
}

func (s *ApprovalProcessorTestSuite) newProcessContext(instance *approval.Instance, node *approval.FlowNode) *engine.ProcessContext {
	return &engine.ProcessContext{
		DB:          s.db,
		Instance:    instance,
		Node:        node,
		FormData:    approval.NewFormData(instance.FormData),
		ApplicantID: instance.ApplicantID,
		Registry:    s.registry,
	}
}

// --- Tests ---

func (s *ApprovalProcessorTestSuite) TestNodeKind() {
	s.Assert().Equal(approval.NodeApproval, s.processor.NodeKind(), "Should return NodeApproval kind")
}

func (s *ApprovalProcessorTestSuite) TestProcessWithAssignees() {
	instance := s.newInstance("applicant-1")
	s.insertAssigneeConfig([]string{"user-1", "user-2"})

	pc := s.newProcessContext(instance, s.newNode())

	result, err := s.processor.Process(s.ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for approval tasks")

	// Verify tasks created
	tasks := s.queryTasks(instance.ID)
	s.Require().Len(tasks, 2, "Should create 2 tasks")

	assigneeIDs := make([]string, len(tasks))
	for i, task := range tasks {
		assigneeIDs[i] = task.AssigneeID
		s.Assert().Equal(instance.ID, task.InstanceID, "Task should reference instance")
		s.Assert().Equal(s.nodeID, task.NodeID, "Task should reference node")
		s.Assert().Equal(approval.TaskPending, task.Status, "Task should be pending")
	}
	s.Assert().ElementsMatch([]string{"user-1", "user-2"}, assigneeIDs, "Should create tasks for all assignees")
}

func (s *ApprovalProcessorTestSuite) TestProcessSequentialApproval() {
	instance := s.newInstance("applicant-1")
	s.insertAssigneeConfig([]string{"user-1", "user-2", "user-3"})

	pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
		n.ApprovalMethod = approval.ApprovalSequential
	}))

	result, err := s.processor.Process(s.ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for sequential approval")

	// Verify task ordering
	tasks := s.queryTasks(instance.ID)
	s.Require().Len(tasks, 3, "Should create 3 tasks")

	s.Assert().Equal(approval.TaskPending, tasks[0].Status, "First task should be pending")
	s.Assert().Equal(1, tasks[0].SortOrder, "First task should have sort order 1")

	s.Assert().Equal(approval.TaskWaiting, tasks[1].Status, "Second task should be waiting")
	s.Assert().Equal(2, tasks[1].SortOrder, "Second task should have sort order 2")

	s.Assert().Equal(approval.TaskWaiting, tasks[2].Status, "Third task should be waiting")
	s.Assert().Equal(3, tasks[2].SortOrder, "Third task should have sort order 3")
}

func (s *ApprovalProcessorTestSuite) TestProcessEmptyAssignee() {
	s.Run("AutoPass", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")
		// No assignee config inserted

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeAutoPass
		}))

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should auto-pass when no assignees and EmptyAssigneeAutoPass")

		tasks := s.queryTasks(instance.ID)
		s.Assert().Empty(tasks, "Should not create any tasks")
	})

	s.Run("TransferApplicant", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferApplicant
		}))

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when transferred to applicant")

		tasks := s.queryTasks(instance.ID)
		s.Require().Len(tasks, 1, "Should create one task for applicant")
		s.Assert().Equal("applicant-1", tasks[0].AssigneeID, "Task should be assigned to applicant")
	})

	s.Run("TransferSpecified", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferSpecified
			n.FallbackUserIDs = []string{"fallback-user-1"}
		}))

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when transferred to specified user")

		tasks := s.queryTasks(instance.ID)
		s.Require().Len(tasks, 1, "Should create one task for fallback user")
		s.Assert().Equal("fallback-user-1", tasks[0].AssigneeID, "Task should be assigned to fallback user")
	})

	s.Run("TransferAdmin", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferAdmin
			n.AdminUserIDs = []string{"admin-1", "admin-2"}
		}))

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when transferred to admin")

		tasks := s.queryTasks(instance.ID)
		s.Require().Len(tasks, 2, "Should create tasks for all admins")
		assigneeIDs := []string{tasks[0].AssigneeID, tasks[1].AssigneeID}
		s.Assert().ElementsMatch([]string{"admin-1", "admin-2"}, assigneeIDs, "Tasks should be assigned to admins")
	})

	s.Run("TransferSuperiorNilService", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferSuperior
		}))

		_, err := s.processor.Process(s.ctx, pc)
		s.Require().ErrorIs(err, engine.ErrAssigneeServiceNotConfigured, "Should return ErrAssigneeServiceNotConfigured when assignee service is nil")
	})

	s.Run("DefaultAction", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = "unknown_action"
		}))

		_, err := s.processor.Process(s.ctx, pc)
		s.Require().ErrorIs(err, engine.ErrNoAssignee, "Should return ErrNoAssignee for unknown empty handler action")
	})
}

func (s *ApprovalProcessorTestSuite) TestProcessSameApplicant() {
	s.Run("AutoPass", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("user-1")
		s.insertAssigneeConfig([]string{"user-1"})

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = approval.SameApplicantAutoPass
		}))

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should auto-pass when same applicant")

		tasks := s.queryTasks(instance.ID)
		s.Assert().Empty(tasks, "Should not create tasks when auto-passing")
	})

	s.Run("SelfApprove", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("user-1")
		s.insertAssigneeConfig([]string{"user-1"})

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = approval.SameApplicantSelfApprove
		}))

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for self-approval")

		tasks := s.queryTasks(instance.ID)
		s.Require().Len(tasks, 1, "Should create one task for self-approval")
		s.Assert().Equal("user-1", tasks[0].AssigneeID, "Task should be assigned to applicant")
	})

	s.Run("NotSameApplicant", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")
		s.insertAssigneeConfig([]string{"user-1", "user-2"})

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = approval.SameApplicantAutoPass
		}))

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when assignees differ from applicant")

		tasks := s.queryTasks(instance.ID)
		s.Assert().Len(tasks, 2, "Should create tasks normally when assignees differ")
	})

	s.Run("TransferSuperiorNilService", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("user-1")
		s.insertAssigneeConfig([]string{"user-1"})

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = approval.SameApplicantTransferSuperior
		}))

		_, err := s.processor.Process(s.ctx, pc)
		s.Require().ErrorIs(err, engine.ErrAssigneeServiceNotConfigured, "Should return ErrAssigneeServiceNotConfigured when assignee service is nil")
	})

	s.Run("DefaultAction", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("user-1")
		s.insertAssigneeConfig([]string{"user-1"})

		pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = "unknown_action"
		}))

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error for unknown same-applicant action")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should default to creating tasks")

		tasks := s.queryTasks(instance.ID)
		s.Require().Len(tasks, 1, "Should create task for applicant in default branch")
		s.Assert().Equal("user-1", tasks[0].AssigneeID, "Task should be assigned to applicant")
	})
}

func (s *ApprovalProcessorTestSuite) TestProcessFormSnapshot() {
	instance := s.newInstance("applicant-1")
	instance.FormData = map[string]any{"amount": float64(1000)}
	_, err := s.db.NewUpdate().Model(instance).Select("form_data").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should update instance form data")

	s.insertAssigneeConfig([]string{"user-1"})

	pc := s.newProcessContext(instance, s.newNode())

	_, err = s.processor.Process(s.ctx, pc)
	s.Require().NoError(err, "Should process without error")

	// Verify form snapshot created
	snapshots := s.queryFormSnapshots(instance.ID)
	s.Require().Len(snapshots, 1, "Should create one form snapshot")
	s.Assert().Equal(instance.ID, snapshots[0].InstanceID, "Snapshot should reference instance")
	s.Assert().Equal(s.nodeID, snapshots[0].NodeID, "Snapshot should reference node")
}

func (s *ApprovalProcessorTestSuite) TestProcessDeduplication() {
	instance := s.newInstance("applicant-1")
	s.insertAssigneeConfig([]string{"user-1", "user-1", "user-2"})

	pc := s.newProcessContext(instance, s.newNode(func(n *approval.FlowNode) {
		n.DuplicateAssigneeAction = approval.DuplicateAssigneeAutoPass
	}))

	result, err := s.processor.Process(s.ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for approval")

	tasks := s.queryTasks(instance.ID)
	s.Require().Len(tasks, 2, "Should create 2 tasks after deduplication")

	assigneeIDs := make([]string, len(tasks))
	for i, task := range tasks {
		assigneeIDs[i] = task.AssigneeID
	}
	s.Assert().ElementsMatch([]string{"user-1", "user-2"}, assigneeIDs, "Should deduplicate assignees")
}

func (s *ApprovalProcessorTestSuite) TestProcessMultipleAssigneeConfigs() {
	instance := s.newInstance("applicant-1")

	// Insert two separate assignee configs with different sort orders
	cfg1 := &approval.FlowNodeAssignee{
		NodeID:    s.nodeID,
		Kind:      approval.AssigneeUser,
		IDs:       []string{"user-1"},
		SortOrder: 1,
	}
	_, err := s.db.NewInsert().Model(cfg1).Exec(s.ctx)
	s.Require().NoError(err, "Should insert first assignee config")

	cfg2 := &approval.FlowNodeAssignee{
		NodeID:    s.nodeID,
		Kind:      approval.AssigneeUser,
		IDs:       []string{"user-2", "user-3"},
		SortOrder: 2,
	}
	_, err = s.db.NewInsert().Model(cfg2).Exec(s.ctx)
	s.Require().NoError(err, "Should insert second assignee config")

	pc := s.newProcessContext(instance, s.newNode())

	result, err := s.processor.Process(s.ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for approval")

	tasks := s.queryTasks(instance.ID)
	s.Require().Len(tasks, 3, "Should create tasks from all assignee configs")

	assigneeIDs := make([]string, len(tasks))
	for i, task := range tasks {
		assigneeIDs[i] = task.AssigneeID
	}
	s.Assert().ElementsMatch([]string{"user-1", "user-2", "user-3"}, assigneeIDs, "Should resolve assignees from all configs")
}

func (s *ApprovalProcessorTestSuite) TestDBError() {
	instance := s.newInstance("applicant-1")
	s.insertAssigneeConfig([]string{"user-1"})

	canceledCtx, cancel := context.WithCancel(s.ctx)
	cancel()

	pc := &engine.ProcessContext{
		DB:          s.db,
		Instance:    instance,
		Node:        s.newNode(),
		FormData:    approval.NewFormData(nil),
		ApplicantID: instance.ApplicantID,
		Registry:    s.registry,
	}

	_, err := s.processor.Process(canceledCtx, pc)
	s.Require().Error(err, "Should return error when context is canceled")
}
