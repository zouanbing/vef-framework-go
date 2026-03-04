package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &StartInstanceTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// MockInstanceNoGenerator is a test implementation of InstanceNoGenerator.
type MockInstanceNoGenerator struct {
	counter int
}

func (g *MockInstanceNoGenerator) Generate(_ context.Context, _ string) (string, error) {
	g.counter++
	return "TEST-" + string(rune('0'+g.counter)), nil
}

// StartInstanceTestSuite tests the StartInstanceHandler.
type StartInstanceTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.StartInstanceHandler
	fixture *FlowFixture
}

func (s *StartInstanceTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine()
	pub := dispatcher.NewEventPublisher()
	validSvc := service.NewValidationService(nil)

	s.handler = command.NewStartInstanceHandler(s.db, eng, &MockInstanceNoGenerator{}, pub, validSvc)
}

func (s *StartInstanceTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.ActionLog)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *StartInstanceTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *StartInstanceTestSuite) TestStartSuccess() {
	applicant := approval.OperatorInfo{ID: "user-1", Name: "User One"}
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		FormData:  map[string]any{"reason": "test"},
	})
	s.Require().NoError(err, "Should start instance without error")
	s.Require().NotNil(instance)

	s.Assert().Equal(approval.InstanceRunning, instance.Status, "Instance should be running")
	s.Assert().Equal("user-1", instance.ApplicantID, "Should set applicant ID")
	s.Assert().NotEmpty(instance.InstanceNo, "Should generate instance number")
	s.Assert().NotEmpty(instance.Title, "Should generate title")

	// Verify action log created
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
		Scan(s.ctx))
	s.Assert().GreaterOrEqual(len(logs), 1, "Should have at least 1 action log (submit)")
}

func (s *StartInstanceTestSuite) TestStartFlowNotFound() {
	applicant := approval.OperatorInfo{ID: "user-1", Name: "User One"}
	_, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "non-existent-flow",
		Applicant: applicant,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound)
}

func (s *StartInstanceTestSuite) TestStartFlowNotActive() {
	// Deactivate the flow
	_, err := s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("is_active", false).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
		Exec(s.ctx)
	s.Require().NoError(err)

	defer func() {
		_, _ = s.db.NewUpdate().
			Model((*approval.Flow)(nil)).
			Set("is_active", true).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
			Exec(s.ctx)
	}()

	applicant := approval.OperatorInfo{ID: "user-1", Name: "User One"}
	_, err = s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
	})
	s.Require().Error(err)
	s.Assert().ErrorIs(err, shared.ErrFlowNotActive)
}

func (s *StartInstanceTestSuite) TestStartWithFormData() {
	applicant := approval.OperatorInfo{ID: "user-2", Name: "User Two"}
	formData := map[string]any{
		"amount":      1000,
		"description": "Business trip",
	}
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		FormData:  formData,
	})
	s.Require().NoError(err, "Should start instance with form data")
	s.Require().NotNil(instance)
	s.Assert().NotNil(instance.FormData, "Should store form data")
}
