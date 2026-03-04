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
		return &AddCCTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// AddCCTestSuite tests the AddCCHandler.
type AddCCTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.AddCCHandler
	fixture *MinimalFixture
	nodeID  string
}

func (s *AddCCTestSuite) SetupSuite() {
	s.handler = command.NewAddCCHandler(s.db, dispatcher.NewEventPublisher())
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "cc")

	node := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "cc-node",
		Kind:              approval.NodeApproval,
		Name:              "CC Test Node",
		IsManualCCAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err)
	s.nodeID = node.ID
}

func (s *AddCCTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.CCRecord)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *AddCCTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *AddCCTestSuite) insertInstance(currentNodeID string) *approval.Instance {
	nodeIDPtr := new(string)
	*nodeIDPtr = currentNodeID
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "CC Test",
		InstanceNo:    "CC-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: nodeIDPtr,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)
	return inst
}

func (s *AddCCTestSuite) TestAddCCSuccess() {
	inst := s.insertInstance(s.nodeID)

	_, err := s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1", "cc-user-2"},
	})
	s.Require().NoError(err, "Should add CC without error")

	// Verify CC records
	var records []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx))
	s.Assert().Len(records, 2, "Should create 2 CC records")
}

func (s *AddCCTestSuite) TestAddCCDuplicateFiltered() {
	inst := s.insertInstance(s.nodeID)

	// Insert existing CC record
	nodeIDPtr := new(string)
	*nodeIDPtr = s.nodeID
	existing := &approval.CCRecord{
		InstanceID: inst.ID,
		NodeID:     nodeIDPtr,
		CCUserID:   "cc-user-1",
		IsManual:   true,
	}
	_, err := s.db.NewInsert().Model(existing).Exec(s.ctx)
	s.Require().NoError(err)

	// Add CC with one existing and one new user
	_, err = s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1", "cc-user-3"},
	})
	s.Require().NoError(err, "Should add CC without error")

	// Verify only new CC record added (total = 2: 1 existing + 1 new)
	var records []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx))
	s.Assert().Len(records, 2, "Should have 2 CC records total (1 existing + 1 new)")
}

func (s *AddCCTestSuite) TestAddCCManualNotAllowed() {
	// Create node with manual CC disabled
	node := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "no-cc-node",
		Kind:              approval.NodeApproval,
		Name:              "No CC Node",
		IsManualCCAllowed: false,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err)
	defer func() {
		_, _ = s.db.NewDelete().Model(node).WherePK().Exec(s.ctx)
	}()

	inst := s.insertInstance(node.ID)

	_, err = s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1"},
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrManualCcNotAllowed, "Should return ErrManualCcNotAllowed")
}

func (s *AddCCTestSuite) TestAddCCInstanceNotFound() {
	_, err := s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: "non-existent",
		CCUserIDs:  []string{"cc-user-1"},
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrInstanceNotFound, "Should return ErrInstanceNotFound")
}
