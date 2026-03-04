package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &MarkCCReadTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// MarkCCReadTestSuite tests the MarkCCReadHandler.
type MarkCCReadTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.MarkCCReadHandler
	fixture *MinimalFixture
}

func (s *MarkCCReadTestSuite) SetupSuite() {
	eng := buildTestEngine()
	_, nodeSvc, _ := buildTestServices(eng)
	s.handler = command.NewMarkCCReadHandler(s.db, nodeSvc)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "mark-cc")
}

func (s *MarkCCReadTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db, (*approval.CCRecord)(nil), (*approval.Instance)(nil))
}

func (s *MarkCCReadTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *MarkCCReadTestSuite) createInstance(no string) string {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "CC Read Test",
		InstanceNo:    no,
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err)
	return inst.ID
}

func (s *MarkCCReadTestSuite) TestMarkReadSuccess() {
	instID := s.createInstance("MCC-001")

	// Insert unread CC records
	records := []approval.CCRecord{
		{InstanceID: instID, CCUserID: "cc-user-1", IsManual: false},
		{InstanceID: instID, CCUserID: "cc-user-1", IsManual: true},
		{InstanceID: instID, CCUserID: "cc-user-2", IsManual: false},
	}
	for i := range records {
		_, err := s.db.NewInsert().Model(&records[i]).Exec(s.ctx)
		s.Require().NoError(err)
	}

	_, err := s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-user-1",
	})
	s.Require().NoError(err, "Should mark CC as read without error")

	// Verify cc-user-1's records are marked as read
	var updatedRecords []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().Model(&updatedRecords).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instID).
				Equals("cc_user_id", "cc-user-1")
		}).
		Scan(s.ctx))

	for _, r := range updatedRecords {
		s.Assert().NotNil(r.ReadAt, "Should set ReadAt for cc-user-1")
	}

	// Verify cc-user-2's records are untouched
	var otherRecords []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().Model(&otherRecords).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instID).
				Equals("cc_user_id", "cc-user-2")
		}).
		Scan(s.ctx))

	for _, r := range otherRecords {
		s.Assert().Nil(r.ReadAt, "Should not set ReadAt for cc-user-2")
	}
}

func (s *MarkCCReadTestSuite) TestMarkReadNoRecords() {
	_, err := s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: "non-existent-instance",
		UserID:     "cc-user-1",
	})
	s.Require().NoError(err, "Should not error when no CC records exist")
}

func (s *MarkCCReadTestSuite) TestMarkReadIdempotent() {
	instID := s.createInstance("MCC-002")

	record := &approval.CCRecord{
		InstanceID: instID,
		CCUserID:   "cc-user-3",
		IsManual:   false,
	}
	_, err := s.db.NewInsert().Model(record).Exec(s.ctx)
	s.Require().NoError(err)

	// Mark read first time
	_, err = s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-user-3",
	})
	s.Require().NoError(err)

	// Mark read second time - should be idempotent (no unread records left)
	_, err = s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-user-3",
	})
	s.Require().NoError(err, "Should be idempotent")
}
