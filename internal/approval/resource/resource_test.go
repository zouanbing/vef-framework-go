package resource_test

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/config"
	internalApproval "github.com/ilxqx/vef-framework-go/internal/approval"
	"github.com/ilxqx/vef-framework-go/internal/apptest"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/security"
)

// --- Mock implementations ---

// MockAssigneeService is a no-op implementation of approval.AssigneeService for testing.
type MockAssigneeService struct{}

func (m *MockAssigneeService) GetSuperior(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *MockAssigneeService) GetDeptLeaders(_ context.Context, _ string) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockAssigneeService) GetRoleUsers(_ context.Context, _ string) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

// MockPrincipalDeptResolver is a test implementation of approval.PrincipalDeptResolver.
type MockPrincipalDeptResolver struct{}

func (m *MockPrincipalDeptResolver) Resolve(_ context.Context, _ *security.Principal) (*string, *string, error) {
	return nil, nil, nil
}

// MockInstanceNoGenerator is a test implementation of approval.InstanceNoGenerator.
type MockInstanceNoGenerator struct {
	counter atomic.Int64
}

func (g *MockInstanceNoGenerator) Generate(_ context.Context, flowCode string) (string, error) {
	n := g.counter.Add(1)
	return fmt.Sprintf("%s-%04d", flowCode, n), nil
}

// --- Test suite ---

// CategoryResourceTestSuite tests the category CRUD resource via HTTP.
type CategoryResourceTestSuite struct {
	apptest.Suite

	ctx   context.Context
	bunDB *bun.DB
	token string
}

func TestCategoryResource(t *testing.T) {
	suite.Run(t, new(CategoryResourceTestSuite))
}

func (s *CategoryResourceTestSuite) SetupSuite() {
	s.ctx = context.Background()

	pgContainer := testx.NewPostgresContainer(s.ctx, s.T())

	s.SetupApp(
		fx.Replace(
			pgContainer.DataSource,
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
			&config.ApprovalConfig{AutoMigrate: true},
		),
		internalApproval.Module,
		fx.Provide(
			fx.Annotate(func() approval.AssigneeService { return &MockAssigneeService{} }, fx.As(new(approval.AssigneeService))),
			fx.Annotate(func() approval.PrincipalDeptResolver { return &MockPrincipalDeptResolver{} }, fx.As(new(approval.PrincipalDeptResolver))),
			fx.Annotate(func() approval.InstanceNoGenerator { return &MockInstanceNoGenerator{} }, fx.As(new(approval.InstanceNoGenerator))),
		),
		fx.Populate(&s.bunDB),
	)

	// Migration is handled by the approval module's migration.Module (auto-migrate on start)

	s.token = s.GenerateToken(security.NewUser("test-admin", "admin"))
}

func (s *CategoryResourceTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *CategoryResourceTestSuite) TearDownTest() {
	_, _ = s.bunDB.NewDelete().Model((*approval.FlowCategory)(nil)).Where("id IS NOT NULL").Exec(s.ctx)
}

func (s *CategoryResourceTestSuite) TestCreateCategory() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
		},
		Params: map[string]any{
			"tenantId": "default",
			"code":     "test-cat",
			"name":     "Test Category",
			"isActive": true,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should create category successfully")

	data := s.ReadDataAsMap(res.Data)
	s.Assert().Equal("test-cat", data["code"])
	s.Assert().Equal("Test Category", data["name"])
	s.Assert().NotEmpty(data["id"], "Should generate an ID")
}

func (s *CategoryResourceTestSuite) TestFindAllCategories() {
	// Insert test data
	for i := 1; i <= 3; i++ {
		cat := &approval.FlowCategory{
			TenantID: "default",
			Code:     fmt.Sprintf("cat-%d", i),
			Name:     fmt.Sprintf("Category %d", i),
			IsActive: true,
		}
		_, err := s.bunDB.NewInsert().Model(cat).Exec(s.ctx)
		s.Require().NoError(err)
	}

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "find_all",
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk())

	items := s.ReadDataAsSlice(res.Data)
	s.Assert().Len(items, 3, "Should return 3 categories")
}

func (s *CategoryResourceTestSuite) TestDeleteCategory() {
	cat := &approval.FlowCategory{
		TenantID: "default",
		Code:     "del-cat",
		Name:     "Delete Me",
		IsActive: true,
	}
	_, err := s.bunDB.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "delete",
		},
		Params: map[string]any{
			"ids": []string{cat.ID},
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should delete category successfully")

	// Verify deleted
	count, err := s.bunDB.NewSelect().Model((*approval.FlowCategory)(nil)).
		Where("id = ?", cat.ID).Count(s.ctx)
	s.Require().NoError(err)
	s.Assert().Equal(0, count, "Category should be deleted")
}
