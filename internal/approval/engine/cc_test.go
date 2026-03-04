package engine_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &CCProcessorTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// CCProcessorTestSuite tests CCProcessor with a real database.
type CCProcessorTestSuite struct {
	suite.Suite

	ctx       context.Context
	db        orm.DB
	processor *engine.CCProcessor

	flowID        string
	flowVersionID string
	nodeID        string
}

func (s *CCProcessorTestSuite) SetupSuite() {
	s.processor = engine.NewCCProcessor()

	// Build FK chain: FlowCategory → Flow → FlowVersion → FlowNode
	category := &approval.FlowCategory{TenantID: "default", Code: "cc-test", Name: "CC Test"}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:              "default",
		CategoryID:            category.ID,
		Code:                  "cc-test-flow",
		Name:                  "CC Test Flow",
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
		FlowVersionID: version.ID,
		Key:           "cc-node-1",
		Kind:          approval.NodeCC,
		Name:          "CC Node",
	}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow node")
	s.nodeID = node.ID
}

// cleanTransientData removes all transient test data (respect FK order).
// Called from TearDownTest and deferred in each s.Run() subtest for data isolation.
func (s *CCProcessorTestSuite) cleanTransientData() {
	_, err := s.db.NewDelete().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean cc records")

	_, err = s.db.NewDelete().
		Model((*approval.FlowNodeCC)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean cc configs")

	_, err = s.db.NewDelete().
		Model((*approval.Instance)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean instances")
}

func (s *CCProcessorTestSuite) TearDownTest() {
	s.cleanTransientData()
}

// newInstance creates and inserts a test instance, returning it with its generated ID.
func (s *CCProcessorTestSuite) newInstance() *approval.Instance {
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.flowID,
		FlowVersionID: s.flowVersionID,
		Title:         "CC Test Instance",
		InstanceNo:    "CC-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test instance")

	return instance
}

// newNode builds a FlowNode value with the suite's nodeID and the given IsReadConfirmRequired flag.
func (s *CCProcessorTestSuite) newNode(readConfirm bool) *approval.FlowNode {
	node := &approval.FlowNode{IsReadConfirmRequired: readConfirm}
	node.ID = s.nodeID

	return node
}

func (s *CCProcessorTestSuite) insertCCConfig(ids []string) {
	cfg := &approval.FlowNodeCC{
		NodeID: s.nodeID,
		Kind:   approval.CCUser,
		IDs:    ids,
		Timing: approval.CCTimingAlways,
	}
	_, err := s.db.NewInsert().Model(cfg).Exec(s.ctx)
	s.Require().NoError(err, "Should insert cc config")
}

// --- Tests ---

func (s *CCProcessorTestSuite) TestNodeKind() {
	s.Assert().Equal(approval.NodeCC, s.processor.NodeKind(), "Should return NodeCC kind")
}

func (s *CCProcessorTestSuite) TestNoCCConfigs() {
	s.Run("Continue", func() {
		defer s.cleanTransientData()

		instance := s.newInstance()
		pc := &engine.ProcessContext{
			DB:       s.db,
			Instance: instance,
			Node:     s.newNode(false),
		}

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should not error when no CC configs exist")
		s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should continue when no CC users")
		s.Assert().Empty(result.Events, "Should have no events when no CC users")
	})

	s.Run("WaitWhenReadConfirmRequired", func() {
		defer s.cleanTransientData()

		instance := s.newInstance()
		pc := &engine.ProcessContext{
			DB:       s.db,
			Instance: instance,
			Node:     s.newNode(true),
		}

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should not error when no CC configs exist")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when read confirm required even with no CC users")
		s.Assert().Empty(result.Events, "Should have no events when no CC users")
	})
}

func (s *CCProcessorTestSuite) TestSingleCCConfig() {
	s.Run("Continue", func() {
		defer s.cleanTransientData()

		instance := s.newInstance()
		s.insertCCConfig([]string{"cc-user-1", "cc-user-2"})

		pc := &engine.ProcessContext{
			DB:       s.db,
			Instance: instance,
			Node:     s.newNode(false),
		}

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should continue when not blocking")
		s.Require().Len(result.Events, 1, "Should emit one CC event")

		// Verify CC records in DB
		var records []approval.CCRecord
		err = s.db.NewSelect().
			Model(&records).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
			Scan(s.ctx)
		s.Require().NoError(err, "Should query CC records")
		s.Assert().Len(records, 2, "Should create 2 CC records")

		userIDs := make([]string, len(records))
		for i, record := range records {
			userIDs[i] = record.CCUserID
			s.Assert().Equal(instance.ID, record.InstanceID, "Record should reference instance")
			s.Assert().NotNil(record.NodeID, "Record should reference node")
			s.Assert().Equal(s.nodeID, *record.NodeID, "Record should reference correct node")
			s.Assert().False(record.IsManual, "Record should not be manual")
		}
		s.Assert().ElementsMatch([]string{"cc-user-1", "cc-user-2"}, userIDs, "Should create records for all CC users")
	})

	s.Run("WaitWhenReadConfirmRequired", func() {
		defer s.cleanTransientData()

		instance := s.newInstance()
		s.insertCCConfig([]string{"cc-user-1"})

		pc := &engine.ProcessContext{
			DB:       s.db,
			Instance: instance,
			Node:     s.newNode(true),
		}

		result, err := s.processor.Process(s.ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when read confirm required")
		s.Require().Len(result.Events, 1, "Should emit one CC event")
	})
}

func (s *CCProcessorTestSuite) TestMultipleCCConfigs() {
	instance := s.newInstance()
	s.insertCCConfig([]string{"cc-user-1"})
	s.insertCCConfig([]string{"cc-user-2", "cc-user-3"})

	pc := &engine.ProcessContext{
		DB:       s.db,
		Instance: instance,
		Node:     s.newNode(false),
	}

	result, err := s.processor.Process(s.ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should continue")
	s.Require().Len(result.Events, 1, "Should emit one CC event")

	// Verify 3 CC records created
	var records []approval.CCRecord
	err = s.db.NewSelect().
		Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query CC records")
	s.Assert().Len(records, 3, "Should create records from all CC configs")
}

func (s *CCProcessorTestSuite) TestCCConfigWithEmptyIDs() {
	instance := s.newInstance()
	s.insertCCConfig([]string{})

	pc := &engine.ProcessContext{
		DB:       s.db,
		Instance: instance,
		Node:     s.newNode(false),
	}

	result, err := s.processor.Process(s.ctx, pc)
	s.Require().NoError(err, "Should not error with empty CC user IDs")
	s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should continue when no CC users resolved")
	s.Assert().Empty(result.Events, "Should have no events when CC user list is empty")
}

func (s *CCProcessorTestSuite) TestDBError() {
	instance := s.newInstance()

	canceledCtx, cancel := context.WithCancel(s.ctx)
	cancel()

	pc := &engine.ProcessContext{
		DB:       s.db,
		Instance: instance,
		Node:     s.newNode(false),
	}

	_, err := s.processor.Process(canceledCtx, pc)
	s.Require().Error(err, "Should return error when context is canceled")
	s.Assert().Contains(err.Error(), "load cc configs", "Should wrap with select error context")
}
