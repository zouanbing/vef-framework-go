package resource_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/api"
	approvalPkg "github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/encoding"
	"github.com/ilxqx/vef-framework-go/internal/app"
	approval "github.com/ilxqx/vef-framework-go/internal/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/apptest"
	"github.com/ilxqx/vef-framework-go/internal/database"
	"github.com/ilxqx/vef-framework-go/internal/orm"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/result"
	"github.com/ilxqx/vef-framework-go/security"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type stubOrgService struct{}

func (s *stubOrgService) GetSuperior(_ context.Context, _ string) (string, string, error) {
	return "", "", nil
}

func (s *stubOrgService) GetDeptLeaders(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

type stubUserService struct{}

func (s *stubUserService) GetUsersByRole(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

type stubSerialNoGenerator struct{ counter int }

func (s *stubSerialNoGenerator) Generate(_ context.Context, flowCode string) (string, error) {
	s.counter++
	return flowCode + "-" + strings.Repeat("0", 4) + string(rune('0'+s.counter)), nil
}

// ---------------------------------------------------------------------------
// ApprovalSuite — base
// ---------------------------------------------------------------------------

const testJWTSecret = "af6675678bd81ad7c93c4a51d122ef61e9750fe5d42ceac1c33b293f36bc14c2"

type ApprovalSuite struct {
	suite.Suite

	ctx       context.Context
	pgc       *testx.PostgresContainer
	app       *app.App
	stop      func()
	authToken string
	bunDB     *bun.DB
}

func (s *ApprovalSuite) makeAPIRequest(body api.Request) *http.Response {
	jsonBody, err := encoding.ToJSON(body)
	s.Require().NoError(err)

	req := httptest.NewRequest(fiber.MethodPost, "/api", strings.NewReader(jsonBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, security.AuthSchemeBearer+" "+s.authToken)

	resp, err := s.app.Test(req)
	s.Require().NoError(err)

	return resp
}

func (s *ApprovalSuite) readBody(resp *http.Response) result.Result {
	body, err := io.ReadAll(resp.Body)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.T().Errorf("failed to close response body: %v", closeErr)
		}
	}()

	s.Require().NoError(err)
	res, err := encoding.FromJSON[result.Result](string(body))
	s.Require().NoError(err)

	return *res
}

func (s *ApprovalSuite) readDataAsMap(data any) map[string]any {
	m, ok := data.(map[string]any)
	s.Require().True(ok, "Expected data to be a map")

	return m
}

func (s *ApprovalSuite) readDataAsSlice(data any) []any {
	sl, ok := data.([]any)
	s.Require().True(ok, "Expected data to be a slice")

	return sl
}

// ---------------------------------------------------------------------------
// Table creation helpers (PostgreSQL DDL from database_migration.sql)
// ---------------------------------------------------------------------------

func createApprovalTables(t *testing.T, bunDB *bun.DB) {
	ctx := context.Background()

	// Use the official PostgreSQL migration DDL (without COMMENT ON / CREATE INDEX for brevity)
	ddls := []string{
		`CREATE TABLE IF NOT EXISTS apv_flow_category (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			code VARCHAR(64) NOT NULL UNIQUE,
			name VARCHAR(128) NOT NULL,
			icon VARCHAR(128),
			parent_id VARCHAR(32),
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT true,
			remark VARCHAR(256)
		)`,
		`CREATE TABLE IF NOT EXISTS apv_flow (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			tenant_id VARCHAR(32) NOT NULL,
			category_id VARCHAR(32) NOT NULL,
			code VARCHAR(64) NOT NULL UNIQUE,
			name VARCHAR(128) NOT NULL,
			icon VARCHAR(128),
			description VARCHAR(512),
			binding_mode VARCHAR(16) NOT NULL DEFAULT 'standalone',
			business_table VARCHAR(64),
			business_pk_field VARCHAR(64),
			business_title_field VARCHAR(64),
			business_status_field VARCHAR(64),
			admin_user_ids JSONB NOT NULL DEFAULT '[]',
			is_all_initiate_allowed BOOLEAN NOT NULL DEFAULT true,
			instance_title_template VARCHAR(256) NOT NULL DEFAULT '',
			is_active BOOLEAN NOT NULL DEFAULT false,
			current_version INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS apv_flow_initiator (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			flow_id VARCHAR(32) NOT NULL,
			initiator_kind VARCHAR(16) NOT NULL,
			initiator_ids JSONB NOT NULL DEFAULT '[]'
		)`,
		`CREATE TABLE IF NOT EXISTS apv_flow_version (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			flow_id VARCHAR(32) NOT NULL,
			version INTEGER NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'draft',
			storage_mode VARCHAR(8) NOT NULL DEFAULT 'json',
			flow_schema JSONB,
			form_schema JSONB,
			published_at TIMESTAMP,
			published_by VARCHAR(32)
		)`,
		`CREATE TABLE IF NOT EXISTS apv_flow_node (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			flow_version_id VARCHAR(32) NOT NULL,
			node_key VARCHAR(64) NOT NULL,
			node_kind VARCHAR(16) NOT NULL,
			name VARCHAR(128) NOT NULL,
			description VARCHAR(512),
			execution_type VARCHAR(16) NOT NULL DEFAULT 'manual',
			approval_method VARCHAR(16) NOT NULL DEFAULT 'parallel',
			pass_rule VARCHAR(16) NOT NULL DEFAULT 'all',
			pass_ratio NUMERIC(3,2) NOT NULL DEFAULT 1.00,
			empty_handler_action VARCHAR(16) NOT NULL DEFAULT 'auto_pass',
			fallback_user_ids JSONB NOT NULL DEFAULT '[]',
			admin_user_ids JSONB NOT NULL DEFAULT '[]',
			same_applicant_action VARCHAR(32) NOT NULL DEFAULT 'self_approve',
			is_rollback_allowed BOOLEAN NOT NULL DEFAULT true,
			rollback_type VARCHAR(16) NOT NULL DEFAULT 'previous',
			rollback_data_strategy VARCHAR(16),
			is_add_assignee_allowed BOOLEAN NOT NULL DEFAULT true,
			add_assignee_types JSONB NOT NULL DEFAULT '["before","after","parallel"]',
			is_remove_assignee_allowed BOOLEAN NOT NULL DEFAULT true,
			field_permissions JSONB NOT NULL DEFAULT '{}',
			is_manual_cc_allowed BOOLEAN NOT NULL DEFAULT true,
			is_transfer_allowed BOOLEAN NOT NULL DEFAULT true,
			is_opinion_required BOOLEAN NOT NULL DEFAULT false,
			timeout_hours INTEGER NOT NULL DEFAULT 0,
			duplicate_handler_action VARCHAR(32) NOT NULL DEFAULT 'none',
			sub_flow_config JSONB,
			branches JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS apv_flow_node_assignee (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			node_id VARCHAR(32) NOT NULL,
			assignee_kind VARCHAR(16) NOT NULL,
			assignee_ids JSONB NOT NULL DEFAULT '[]',
			form_field VARCHAR(64),
			sort_order INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS apv_flow_node_cc (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			node_id VARCHAR(32) NOT NULL,
			cc_kind VARCHAR(16) NOT NULL,
			cc_ids JSONB NOT NULL DEFAULT '[]',
			form_field VARCHAR(64)
		)`,
		`CREATE TABLE IF NOT EXISTS apv_flow_edge (
			id VARCHAR(32) PRIMARY KEY,
			flow_version_id VARCHAR(32) NOT NULL,
			source_node_id VARCHAR(32) NOT NULL,
			target_node_id VARCHAR(32) NOT NULL,
			source_handle VARCHAR(64)
		)`,
		`CREATE TABLE IF NOT EXISTS apv_flow_form_field (
			id VARCHAR(32) PRIMARY KEY,
			flow_version_id VARCHAR(32) NOT NULL,
			name VARCHAR(64) NOT NULL,
			kind VARCHAR(32) NOT NULL,
			label VARCHAR(128) NOT NULL,
			placeholder VARCHAR(256),
			default_value TEXT,
			is_required BOOLEAN DEFAULT false,
			is_readonly BOOLEAN DEFAULT false,
			validation JSONB,
			sort_order INTEGER NOT NULL DEFAULT 0,
			meta JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS apv_instance (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			flow_id VARCHAR(32) NOT NULL,
			flow_version_id VARCHAR(32) NOT NULL,
			parent_instance_id VARCHAR(32),
			parent_node_id VARCHAR(32),
			title VARCHAR(256) NOT NULL,
			serial_no VARCHAR(64) NOT NULL UNIQUE,
			applicant_id VARCHAR(32) NOT NULL,
			applicant_dept_id VARCHAR(32),
			status VARCHAR(16) NOT NULL DEFAULT 'running',
			current_node_id VARCHAR(32),
			finished_at TIMESTAMP,
			business_record_id VARCHAR(128),
			form_data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS apv_task (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			instance_id VARCHAR(32) NOT NULL,
			node_id VARCHAR(32) NOT NULL,
			assignee_id VARCHAR(32) NOT NULL,
			delegate_from_id VARCHAR(32),
			sort_order INTEGER NOT NULL DEFAULT 0,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			read_at TIMESTAMP,
			parent_task_id VARCHAR(32),
			add_assignee_type VARCHAR(16),
			deadline TIMESTAMP,
			is_timeout BOOLEAN NOT NULL DEFAULT false,
			finished_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS apv_action_log (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			instance_id VARCHAR(32) NOT NULL,
			node_id VARCHAR(32),
			task_id VARCHAR(32),
			action VARCHAR(16) NOT NULL,
			operator_id VARCHAR(32) NOT NULL,
			operator_name VARCHAR(128),
			operator_dept VARCHAR(128),
			ip_address VARCHAR(64),
			user_agent VARCHAR(512),
			opinion TEXT,
			meta JSONB,
			transfer_to_id VARCHAR(32),
			rollback_to_node_id VARCHAR(32),
			add_assignee_type VARCHAR(16),
			add_assignee_to_ids JSONB DEFAULT '[]',
			remove_assignee_ids JSONB DEFAULT '[]',
			cc_user_ids JSONB DEFAULT '[]',
			attachments JSONB DEFAULT '[]'
		)`,
		`CREATE TABLE IF NOT EXISTS apv_parallel_record (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			instance_id VARCHAR(32) NOT NULL,
			node_id VARCHAR(32) NOT NULL,
			task_id VARCHAR(32) NOT NULL,
			assignee_id VARCHAR(32) NOT NULL,
			result VARCHAR(16),
			opinion TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS apv_cc_record (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			instance_id VARCHAR(32) NOT NULL,
			node_id VARCHAR(32),
			task_id VARCHAR(32),
			cc_user_id VARCHAR(32) NOT NULL,
			is_manual BOOLEAN NOT NULL DEFAULT false,
			read_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS apv_delegation (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			delegator_id VARCHAR(32) NOT NULL,
			delegatee_id VARCHAR(32) NOT NULL,
			flow_category_id VARCHAR(32),
			flow_id VARCHAR(32),
			start_time TIMESTAMP,
			end_time TIMESTAMP,
			is_active BOOLEAN NOT NULL DEFAULT true,
			reason VARCHAR(256)
		)`,
		`CREATE TABLE IF NOT EXISTS apv_form_snapshot (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			instance_id VARCHAR(32) NOT NULL,
			node_id VARCHAR(32) NOT NULL,
			form_data JSONB NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS apv_event_outbox (
			id VARCHAR(32) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
			created_by VARCHAR(32) NOT NULL DEFAULT 'system',
			updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
			event_id VARCHAR(64) NOT NULL UNIQUE,
			event_type VARCHAR(128) NOT NULL,
			payload JSONB NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			retry_count INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			processed_at TIMESTAMP
		)`,
	}

	for _, ddl := range ddls {
		_, err := bunDB.ExecContext(ctx, ddl)
		if err != nil {
			t.Fatalf("Failed to create table: %v\nDDL: %s", err, ddl[:80])
		}
	}
}

// ---------------------------------------------------------------------------
// Setup / Teardown
// ---------------------------------------------------------------------------

// stubUserLoader implements security.UserLoader for test JWT validation.
type stubUserLoader struct {
	principal *security.Principal
}

func (l *stubUserLoader) LoadByUsername(_ context.Context, _ string) (*security.Principal, string, error) {
	return l.principal, "", nil
}

func (l *stubUserLoader) LoadByID(_ context.Context, _ string) (*security.Principal, error) {
	return l.principal, nil
}

func (s *ApprovalSuite) SetupSuite() {
	s.ctx = context.Background()

	s.pgc = testx.NewPostgresContainer(s.ctx, s.T())
	dsConfig := s.pgc.DataSource

	bunDB, err := database.New(dsConfig)
	s.Require().NoError(err)

	createApprovalTables(s.T(), bunDB)

	bunDB.RegisterModel(
		(*approvalPkg.FlowCategory)(nil),
		(*approvalPkg.Flow)(nil),
		(*approvalPkg.FlowInitiator)(nil),
		(*approvalPkg.FlowVersion)(nil),
		(*approvalPkg.FlowNode)(nil),
		(*approvalPkg.FlowNodeAssignee)(nil),
		(*approvalPkg.FlowNodeCC)(nil),
		(*approvalPkg.FlowEdge)(nil),
		(*approvalPkg.FlowFormField)(nil),
		(*approvalPkg.Instance)(nil),
		(*approvalPkg.Task)(nil),
		(*approvalPkg.ActionLog)(nil),
		(*approvalPkg.ParallelRecord)(nil),
		(*approvalPkg.CCRecord)(nil),
		(*approvalPkg.Delegation)(nil),
		(*approvalPkg.FormSnapshot)(nil),
		(*approvalPkg.EventOutbox)(nil),
	)

	ormDB := orm.New(bunDB)
	s.bunDB = bunDB

	orgSvc := &stubOrgService{}
	userSvc := &stubUserService{}
	serialGen := &stubSerialNoGenerator{}
	testUser := security.NewUser("test-user-001", "Test User", "admin")
	userLoader := &stubUserLoader{principal: testUser}

	// Audience must match lo.SnakeCase(appName) = "test_app"
	jwtCfg := &security.JWTConfig{
		Secret:   testJWTSecret,
		Audience: "test_app",
	}
	jwtInstance, err := security.NewJWT(jwtCfg)
	s.Require().NoError(err)

	claims := security.NewJWTClaimsBuilder().
		WithSubject(testUser.ID + "@" + testUser.Name).
		WithRoles(testUser.Roles).
		WithType("access")
	s.authToken, err = jwtInstance.Generate(claims, 1*time.Hour, 0)
	s.Require().NoError(err)

	s.app, s.stop = apptest.NewTestAppWithDB(
		s.T(),
		bunDB,
		fx.Replace(
			dsConfig,
			jwtCfg,
		),
		fx.Supply(
			fx.Annotate(orgSvc, fx.As(new(approvalPkg.OrganizationService))),
			fx.Annotate(userSvc, fx.As(new(approvalPkg.UserService))),
			fx.Annotate(serialGen, fx.As(new(service.SerialNoGenerator))),
			fx.Annotate(userLoader, fx.As(new(security.UserLoader))),
		),
		approval.Module,
		fx.Decorate(func() orm.DB { return ormDB }),
	)
}

func (s *ApprovalSuite) TearDownSuite() {
	if s.stop != nil {
		s.stop()
	}
}

// ---------------------------------------------------------------------------
// Category CRUD Tests
// ---------------------------------------------------------------------------

func (s *ApprovalSuite) TestCategoryCreate() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":      "cat-001",
			"name":      "请假审批",
			"sortOrder": 1,
			"isActive":  true,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "create should succeed, got: %v", body)

	data := s.readDataAsMap(body.Data)
	s.NotEmpty(data["id"], "should return created category id")
}

func (s *ApprovalSuite) TestCategoryCreateValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code": "cat-missing-name",
			// name is required but missing
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.False(body.IsOk(), "should fail validation when name is missing")
}

func (s *ApprovalSuite) TestCategoryFindAll() {
	s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":     "cat-find-all",
			"name":     "查找全部",
			"isActive": true,
		},
	})

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "find_all should succeed")
	s.NotNil(body.Data, "should return data")

	list := s.readDataAsSlice(body.Data)
	s.NotEmpty(list, "should return at least one category")
}

func (s *ApprovalSuite) TestCategoryFindPage() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "find_page",
			Version:  "v1",
		},
		Params: map[string]any{},
		Meta: map[string]any{
			"page": 1,
			"size": 10,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "find_page should succeed")
	s.NotNil(body.Data, "should return page data")

	data := s.readDataAsMap(body.Data)
	s.Contains(data, "items", "page result should contain items")
	s.Contains(data, "total", "page result should contain total")
}

func (s *ApprovalSuite) TestCategoryUpdate() {
	createResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":     "cat-update",
			"name":     "更新前",
			"isActive": true,
		},
	})
	createBody := s.readBody(createResp)
	s.Require().True(createBody.IsOk())
	id := s.readDataAsMap(createBody.Data)["id"]

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":       id,
			"code":     "cat-update",
			"name":     "更新后",
			"isActive": true,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "update should succeed")
}

func (s *ApprovalSuite) TestCategoryDelete() {
	createResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":     "cat-delete",
			"name":     "待删除",
			"isActive": false,
		},
	})
	createBody := s.readBody(createResp)
	s.Require().True(createBody.IsOk())
	id := s.readDataAsMap(createBody.Data)["id"]

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": id,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "delete should succeed")
}

// ---------------------------------------------------------------------------
// Delegation CRUD Tests
// ---------------------------------------------------------------------------

func (s *ApprovalSuite) TestDelegationCreate() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"delegatorId": "user-001",
			"delegateeId": "user-002",
			"isActive":    true,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "create should succeed, got: %v", body)

	data := s.readDataAsMap(body.Data)
	s.NotEmpty(data["id"], "should return created delegation id")
}

func (s *ApprovalSuite) TestDelegationCreateValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"delegatorId": "user-001",
			// delegateeId is required but missing
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.False(body.IsOk(), "should fail validation when delegateeId is missing")
}

func (s *ApprovalSuite) TestDelegationFindAll() {
	createResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"delegatorId": "user-fa-001",
			"delegateeId": "user-fa-002",
			"isActive":    true,
		},
	})
	createBody := s.readBody(createResp)
	s.Require().True(createBody.IsOk())

	// DelegationSearch embeds api.M (via apis.Sortable), so search params
	// are decoded from Meta, not Params. String fields with search:"eq" apply
	// even when empty, so we must pass specific values.
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "find_all",
			Version:  "v1",
		},
		Meta: map[string]any{
			"delegatorId": "user-fa-001",
			"delegateeId": "user-fa-002",
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "find_all should succeed")

	list := s.readDataAsSlice(body.Data)
	s.NotEmpty(list, "should return at least one delegation")
}

func (s *ApprovalSuite) TestDelegationFindPage() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "find_page",
			Version:  "v1",
		},
		Params: map[string]any{},
		Meta: map[string]any{
			"page": 1,
			"size": 10,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "find_page should succeed")

	data := s.readDataAsMap(body.Data)
	s.Contains(data, "items", "page result should contain items")
	s.Contains(data, "total", "page result should contain total")
}

func (s *ApprovalSuite) TestDelegationUpdate() {
	createResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"delegatorId": "user-upd-001",
			"delegateeId": "user-upd-002",
			"isActive":    true,
		},
	})
	createBody := s.readBody(createResp)
	s.Require().True(createBody.IsOk())
	id := s.readDataAsMap(createBody.Data)["id"]

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":          id,
			"delegatorId": "user-upd-001",
			"delegateeId": "user-upd-003",
			"isActive":    false,
			"reason":      "test update",
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "update should succeed")
}

func (s *ApprovalSuite) TestDelegationDelete() {
	createResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"delegatorId": "user-del-001",
			"delegateeId": "user-del-002",
			"isActive":    false,
		},
	})
	createBody := s.readBody(createResp)
	s.Require().True(createBody.IsOk())
	id := s.readDataAsMap(createBody.Data)["id"]

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": id,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "delete should succeed")
}

// ---------------------------------------------------------------------------
// Flow Resource Tests
// ---------------------------------------------------------------------------

func (s *ApprovalSuite) TestFlowDeploy() {
	catResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":     "cat-flow-deploy",
			"name":     "流程部署分类",
			"isActive": true,
		},
	})
	catBody := s.readBody(catResp)
	s.Require().True(catBody.IsOk(), "category create should succeed")
	categoryID := s.readDataAsMap(catBody.Data)["id"].(string)

	definition := `{
		"nodes": [
			{"id": "start", "type": "start", "position": {"x": 0, "y": 0}, "data": {"label": "开始"}},
			{"id": "end", "type": "end", "position": {"x": 200, "y": 0}, "data": {"label": "结束"}}
		],
		"edges": [
			{"source": "start", "target": "end"}
		]
	}`

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "deploy",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode":   "leave-apply",
			"flowName":   "请假流程",
			"categoryId": categoryID,
			"definition": definition,
			"operatorId": "admin-001",
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "deploy should succeed, got: %v", body)
	s.NotNil(body.Data, "should return flow data")
}

func (s *ApprovalSuite) TestFlowPublishVersionAndGetGraph() {
	catResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":     "cat-pub-ver",
			"name":     "发布版本分类",
			"isActive": true,
		},
	})
	catBody := s.readBody(catResp)
	s.Require().True(catBody.IsOk())
	categoryID := s.readDataAsMap(catBody.Data)["id"].(string)

	definition := `{
		"nodes": [
			{"id": "start", "type": "start", "position": {"x": 0, "y": 0}, "data": {"label": "开始"}},
			{"id": "end", "type": "end", "position": {"x": 200, "y": 0}, "data": {"label": "结束"}}
		],
		"edges": [
			{"source": "start", "target": "end"}
		]
	}`
	deployResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "deploy",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode":   "pub-ver-flow",
			"flowName":   "发布版本流程",
			"categoryId": categoryID,
			"definition": definition,
			"operatorId": "admin-001",
		},
	})
	deployBody := s.readBody(deployResp)
	s.Require().True(deployBody.IsOk(), "deploy should succeed")
	flowData := s.readDataAsMap(deployBody.Data)
	flowID := flowData["id"].(string)

	// Test publish_version with invalid version ID (error path)
	pubResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "publish_version",
			Version:  "v1",
		},
		Params: map[string]any{
			"versionId":  "nonexistent-version",
			"operatorId": "admin-001",
		},
	})
	pubBody := s.readBody(pubResp)
	s.False(pubBody.IsOk(), "publish with nonexistent version should fail")

	graphResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "get_graph",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowId": flowID,
		},
	})
	graphBody := s.readBody(graphResp)
	s.False(graphBody.IsOk(), "get_graph should fail when no published version")

	graph2Resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "get_graph",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowId": "nonexistent-flow",
		},
	})
	graph2Body := s.readBody(graph2Resp)
	s.False(graph2Body.IsOk(), "get_graph should fail for nonexistent flow")
}

func (s *ApprovalSuite) TestFlowDeployValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "deploy",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode": "missing-fields",
			// flowName and categoryId are required but missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "deploy should fail with missing required fields")
}

func (s *ApprovalSuite) TestFlowDeployInvalidDefinition() {
	catResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":     "cat-invalid-def",
			"name":     "无效定义分类",
			"isActive": true,
		},
	})
	catBody := s.readBody(catResp)
	s.Require().True(catBody.IsOk())
	categoryID := s.readDataAsMap(catBody.Data)["id"].(string)

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "deploy",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode":   "invalid-def-flow",
			"flowName":   "无效定义流程",
			"categoryId": categoryID,
			"definition": `{"nodes": []}`,
			"operatorId": "admin-001",
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "deploy should fail with empty nodes")
}

// ---------------------------------------------------------------------------
// Instance Resource Tests
// ---------------------------------------------------------------------------

func (s *ApprovalSuite) TestInstanceStart() {
	catResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":     "cat-instance-start",
			"name":     "实例启动分类",
			"isActive": true,
		},
	})
	catBody := s.readBody(catResp)
	s.Require().True(catBody.IsOk())
	categoryID := s.readDataAsMap(catBody.Data)["id"].(string)

	definition := `{
		"nodes": [
			{"id": "start", "type": "start", "position": {"x": 0, "y": 0}, "data": {"label": "开始"}},
			{"id": "approval1", "type": "approval", "position": {"x": 200, "y": 0}, "data": {"label": "审批节点", "assignees": [{"kind": "user", "ids": ["approver-001"]}]}},
			{"id": "end", "type": "end", "position": {"x": 400, "y": 0}, "data": {"label": "结束"}}
		],
		"edges": [
			{"source": "start", "target": "approval1"},
			{"source": "approval1", "target": "end"}
		]
	}`
	deployResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "deploy",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode":   "instance-start-flow",
			"flowName":   "实例启动流程",
			"categoryId": categoryID,
			"definition": definition,
			"operatorId": "admin-001",
		},
	})
	deployBody := s.readBody(deployResp)
	s.Require().True(deployBody.IsOk(), "deploy should succeed, got: %v", deployBody)

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "start",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode":    "instance-start-flow",
			"title":       "测试请假申请",
			"applicantId": "user-001",
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "start should fail when no published version")
}

func (s *ApprovalSuite) TestInstanceStartValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "start",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode": "some-flow",
			// title and applicantId are required but missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "start should fail validation when required fields missing")
}

func (s *ApprovalSuite) TestInstanceWithdrawValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "withdraw",
			Version:  "v1",
		},
		Params: map[string]any{
			// instanceId and operatorId are required but missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "withdraw should fail validation")
}

func (s *ApprovalSuite) TestInstanceProcessTaskValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "process_task",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": "some-instance",
			// other required fields missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "process_task should fail validation")
}

func (s *ApprovalSuite) TestInstanceAddCcValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "add_cc",
			Version:  "v1",
		},
		Params: map[string]any{
			// required fields missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "add_cc should fail validation")
}

func (s *ApprovalSuite) TestInstanceAddAssigneeValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "add_assignee",
			Version:  "v1",
		},
		Params: map[string]any{
			// required fields missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "add_assignee should fail validation")
}

func (s *ApprovalSuite) TestInstanceRemoveAssigneeValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "remove_assignee",
			Version:  "v1",
		},
		Params: map[string]any{
			// required fields missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "remove_assignee should fail validation")
}

func (s *ApprovalSuite) TestInstanceGetDetailValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "get_detail",
			Version:  "v1",
		},
		Params: map[string]any{
			// instanceId is required but missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "get_detail should fail validation")
}

func (s *ApprovalSuite) TestInstanceGetActionLogsValidation() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "get_action_logs",
			Version:  "v1",
		},
		Params: map[string]any{
			// instanceId is required but missing
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "get_action_logs should fail validation")
}

// ---------------------------------------------------------------------------
// Instance Resource Tests (query)
// ---------------------------------------------------------------------------

func (s *ApprovalSuite) TestInstanceFindInstances() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "find_instances",
			Version:  "v1",
		},
		Params: map[string]any{
			"page":     1,
			"pageSize": 10,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "find_instances should succeed, got: %v", body)

	data := s.readDataAsMap(body.Data)
	s.Contains(data, "list", "should contain list")
	s.Contains(data, "total", "should contain total")
}

func (s *ApprovalSuite) TestInstanceFindTasks() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "find_tasks",
			Version:  "v1",
		},
		Params: map[string]any{
			"page":     1,
			"pageSize": 10,
		},
	})

	s.Equal(200, resp.StatusCode)
	body := s.readBody(resp)
	s.True(body.IsOk(), "find_tasks should succeed, got: %v", body)

	data := s.readDataAsMap(body.Data)
	s.Contains(data, "list", "should contain list")
	s.Contains(data, "total", "should contain total")
}

// ---------------------------------------------------------------------------
// Helpers for end-to-end instance tests
// ---------------------------------------------------------------------------

// deployAndPublishFlow creates a category, deploys a flow with an approval node,
// queries the version ID from the DB, and publishes it.
// Returns (flowID, categoryID).
func (s *ApprovalSuite) deployAndPublishFlow(flowCode, categoryCode string) (string, string) {
	s.T().Helper()

	catResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/category",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"code":     categoryCode,
			"name":     "E2E测试分类-" + categoryCode,
			"isActive": true,
		},
	})
	catBody := s.readBody(catResp)
	s.Require().True(catBody.IsOk(), "category create should succeed: %v", catBody)
	categoryID := s.readDataAsMap(catBody.Data)["id"].(string)

	definition := `{
		"nodes": [
			{"id": "start", "type": "start", "position": {"x": 0, "y": 0}, "data": {"label": "开始"}},
			{"id": "approval1", "type": "approval", "position": {"x": 200, "y": 0}, "data": {
				"label": "审批节点",
				"approvalMethod": "parallel",
				"passRule": "all",
				"isTransferAllowed": true,
				"isRollbackAllowed": true,
				"isAddAssigneeAllowed": true,
				"isRemoveAssigneeAllowed": true,
				"isManualCCAllowed": true,
				"assignees": [{"kind": "user", "ids": ["approver-001"]}]
			}},
			{"id": "end", "type": "end", "position": {"x": 400, "y": 0}, "data": {"label": "结束"}}
		],
		"edges": [
			{"source": "start", "target": "approval1"},
			{"source": "approval1", "target": "end"}
		]
	}`

	deployResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "deploy",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode":   flowCode,
			"flowName":   "E2E测试流程-" + flowCode,
			"categoryId": categoryID,
			"definition": definition,
			"operatorId": "admin-001",
		},
	})
	deployBody := s.readBody(deployResp)
	s.Require().True(deployBody.IsOk(), "deploy should succeed: %v", deployBody)
	flowData := s.readDataAsMap(deployBody.Data)
	flowID := flowData["id"].(string)

	var versionID string
	err := s.bunDB.NewSelect().
		TableExpr("apv_flow_version").
		Column("id").
		Where("flow_id = ?", flowID).
		Where("status = ?", "draft").
		OrderExpr("version DESC").
		Limit(1).
		Scan(s.ctx, &versionID)
	s.Require().NoError(err, "should find draft version in DB")
	s.Require().NotEmpty(versionID, "version ID should not be empty")

	pubResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "publish_version",
			Version:  "v1",
		},
		Params: map[string]any{
			"versionId":  versionID,
			"operatorId": "admin-001",
		},
	})
	pubBody := s.readBody(pubResp)
	s.Require().True(pubBody.IsOk(), "publish_version should succeed: %v", pubBody)

	return flowID, categoryID
}

// startInstance starts an instance and returns the instance ID.
func (s *ApprovalSuite) startInstance(flowCode, applicantID string) string {
	s.T().Helper()

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "start",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowCode":    flowCode,
			"title":       "E2E测试申请-" + flowCode,
			"applicantId": applicantID,
		},
	})
	body := s.readBody(resp)
	s.Require().True(body.IsOk(), "start instance should succeed: %v", body)
	data := s.readDataAsMap(body.Data)
	instanceID := data["id"].(string)
	s.Require().NotEmpty(instanceID, "instance ID should not be empty")

	return instanceID
}

// findPendingTask queries the DB for the first pending task of the given instance
// assigned to the given assignee. Returns (taskID, nodeID).
func (s *ApprovalSuite) findPendingTask(instanceID, assigneeID string) (string, string) {
	s.T().Helper()

	type taskRow struct {
		ID     string `bun:"id"`
		NodeID string `bun:"node_id"`
	}

	var row taskRow
	err := s.bunDB.NewSelect().
		TableExpr("apv_task").
		Column("id", "node_id").
		Where("instance_id = ?", instanceID).
		Where("assignee_id = ?", assigneeID).
		Where("status = ?", "pending").
		Limit(1).
		Scan(s.ctx, &row)
	s.Require().NoError(err, "should find pending task for assignee %s", assigneeID)
	s.Require().NotEmpty(row.ID, "task ID should not be empty")

	return row.ID, row.NodeID
}

// ---------------------------------------------------------------------------
// Instance End-to-End Tests
// ---------------------------------------------------------------------------

func (s *ApprovalSuite) TestInstanceStartSuccess() {
	flowID, _ := s.deployAndPublishFlow("e2e-start-flow", "cat-e2e-start")
	s.NotEmpty(flowID)

	instanceID := s.startInstance("e2e-start-flow", "user-e2e-001")
	s.NotEmpty(instanceID)
}

func (s *ApprovalSuite) TestInstanceGetDetailSuccess() {
	s.deployAndPublishFlow("e2e-detail-flow", "cat-e2e-detail")
	instanceID := s.startInstance("e2e-detail-flow", "user-e2e-002")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "get_detail",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "get_detail should succeed: %v", body)
	data := s.readDataAsMap(body.Data)

	s.Contains(data, "instance", "detail should contain instance")
	s.Contains(data, "tasks", "detail should contain tasks")
	s.Contains(data, "actionLogs", "detail should contain actionLogs")
	s.Contains(data, "flowNodes", "detail should contain flowNodes")

	instance := s.readDataAsMap(data["instance"])
	s.Equal(instanceID, instance["id"], "instance ID should match")
	s.Equal("running", instance["status"], "instance should be running")

	tasks := s.readDataAsSlice(data["tasks"])
	s.NotEmpty(tasks, "should have at least one task")

	actionLogs := s.readDataAsSlice(data["actionLogs"])
	s.NotEmpty(actionLogs, "should have at least one action log")
}

func (s *ApprovalSuite) TestInstanceGetDetailNotFound() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "get_detail",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": "nonexistent-instance-id",
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "get_detail should fail for nonexistent instance")
}

func (s *ApprovalSuite) TestInstanceGetActionLogsSuccess() {
	s.deployAndPublishFlow("e2e-logs-flow", "cat-e2e-logs")
	instanceID := s.startInstance("e2e-logs-flow", "user-e2e-003")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "get_action_logs",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "get_action_logs should succeed: %v", body)
	s.NotNil(body.Data, "should return action logs data")

	logs := s.readDataAsSlice(body.Data)
	s.NotEmpty(logs, "should have at least the submit action log")

	firstLog := s.readDataAsMap(logs[0])
	s.Equal("submit", firstLog["action"], "first log should be submit action")
}

func (s *ApprovalSuite) TestInstanceGetActionLogsEmpty() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "get_action_logs",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": "nonexistent-instance-id",
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "get_action_logs should succeed even with no data: %v", body)
}

func (s *ApprovalSuite) TestInstanceProcessTaskApprove() {
	s.deployAndPublishFlow("e2e-approve-flow", "cat-e2e-approve")
	instanceID := s.startInstance("e2e-approve-flow", "user-e2e-004")

	taskID, _ := s.findPendingTask(instanceID, "approver-001")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "process_task",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
			"taskId":     taskID,
			"action":     "approve",
			"operatorId": "approver-001",
			"opinion":    "同意",
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "process_task (approve) should succeed: %v", body)

	var status string
	err := s.bunDB.NewSelect().
		TableExpr("apv_instance").
		Column("status").
		Where("id = ?", instanceID).
		Scan(s.ctx, &status)
	s.Require().NoError(err)
	s.Equal("approved", status, "instance should be approved after all tasks approved")
}

func (s *ApprovalSuite) TestInstanceProcessTaskNotFound() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "process_task",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": "nonexistent-instance",
			"taskId":     "nonexistent-task",
			"action":     "approve",
			"operatorId": "approver-001",
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "process_task should fail for nonexistent instance")
}

func (s *ApprovalSuite) TestInstanceProcessTaskReject() {
	s.deployAndPublishFlow("e2e-reject-flow", "cat-e2e-reject")
	instanceID := s.startInstance("e2e-reject-flow", "user-e2e-005")

	taskID, _ := s.findPendingTask(instanceID, "approver-001")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "process_task",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
			"taskId":     taskID,
			"action":     "reject",
			"operatorId": "approver-001",
			"opinion":    "拒绝",
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "process_task (reject) should succeed: %v", body)

	var status string
	err := s.bunDB.NewSelect().
		TableExpr("apv_instance").
		Column("status").
		Where("id = ?", instanceID).
		Scan(s.ctx, &status)
	s.Require().NoError(err)
	s.Equal("rejected", status, "instance should be rejected")
}

func (s *ApprovalSuite) TestInstanceWithdrawSuccess() {
	s.deployAndPublishFlow("e2e-withdraw-flow", "cat-e2e-withdraw")
	instanceID := s.startInstance("e2e-withdraw-flow", "user-e2e-006")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "withdraw",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
			"operatorId": "user-e2e-006",
			"reason":     "申请人撤回",
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "withdraw should succeed: %v", body)

	var status string
	err := s.bunDB.NewSelect().
		TableExpr("apv_instance").
		Column("status").
		Where("id = ?", instanceID).
		Scan(s.ctx, &status)
	s.Require().NoError(err)
	s.Equal("withdrawn", status, "instance should be withdrawn")
}

func (s *ApprovalSuite) TestInstanceWithdrawNotApplicant() {
	s.deployAndPublishFlow("e2e-withdraw-na-flow", "cat-e2e-withdraw-na")
	instanceID := s.startInstance("e2e-withdraw-na-flow", "user-e2e-007")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "withdraw",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
			"operatorId": "other-user",
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "withdraw should fail when operator is not applicant")
}

func (s *ApprovalSuite) TestInstanceAddCcSuccess() {
	s.deployAndPublishFlow("e2e-cc-flow", "cat-e2e-cc")
	instanceID := s.startInstance("e2e-cc-flow", "user-e2e-008")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "add_cc",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
			"ccUserIds":  []string{"cc-user-001", "cc-user-002"},
			"operatorId": "user-e2e-008",
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "add_cc should succeed: %v", body)

	count, err := s.bunDB.NewSelect().
		TableExpr("apv_cc_record").
		Where("instance_id = ?", instanceID).
		Count(s.ctx)
	s.Require().NoError(err)
	s.Equal(2, count, "should have 2 CC records")
}

func (s *ApprovalSuite) TestInstanceAddCcNotFound() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "add_cc",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": "nonexistent-instance",
			"ccUserIds":  []string{"cc-user-001"},
			"operatorId": "someone",
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "add_cc should fail for nonexistent instance")
}

func (s *ApprovalSuite) TestInstanceAddAssigneeSuccess() {
	s.deployAndPublishFlow("e2e-add-asn-flow", "cat-e2e-add-asn")
	instanceID := s.startInstance("e2e-add-asn-flow", "user-e2e-009")

	taskID, _ := s.findPendingTask(instanceID, "approver-001")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "add_assignee",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
			"taskId":     taskID,
			"userIds":    []string{"new-approver-001"},
			"addType":    "parallel",
			"operatorId": "approver-001",
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "add_assignee should succeed: %v", body)

	count, err := s.bunDB.NewSelect().
		TableExpr("apv_task").
		Where("instance_id = ?", instanceID).
		Where("assignee_id = ?", "new-approver-001").
		Count(s.ctx)
	s.Require().NoError(err)
	s.Equal(1, count, "should have a task for the new assignee")
}

func (s *ApprovalSuite) TestInstanceAddAssigneeNotFound() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "add_assignee",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": "nonexistent-instance",
			"taskId":     "nonexistent-task",
			"userIds":    []string{"user-001"},
			"addType":    "parallel",
			"operatorId": "someone",
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "add_assignee should fail for nonexistent instance")
}

func (s *ApprovalSuite) TestInstanceRemoveAssigneeSuccess() {
	s.deployAndPublishFlow("e2e-rm-asn-flow", "cat-e2e-rm-asn")
	instanceID := s.startInstance("e2e-rm-asn-flow", "user-e2e-010")

	taskID, _ := s.findPendingTask(instanceID, "approver-001")

	addResp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "add_assignee",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId": instanceID,
			"taskId":     taskID,
			"userIds":    []string{"extra-approver-001"},
			"addType":    "parallel",
			"operatorId": "approver-001",
		},
	})
	addBody := s.readBody(addResp)
	s.Require().True(addBody.IsOk(), "add_assignee should succeed first: %v", addBody)

	extraTaskID, _ := s.findPendingTask(instanceID, "extra-approver-001")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "remove_assignee",
			Version:  "v1",
		},
		Params: map[string]any{
			"taskId":     extraTaskID,
			"operatorId": "approver-001",
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "remove_assignee should succeed: %v", body)

	var taskStatus string
	err := s.bunDB.NewSelect().
		TableExpr("apv_task").
		Column("status").
		Where("id = ?", extraTaskID).
		Scan(s.ctx, &taskStatus)
	s.Require().NoError(err)
	s.Equal("removed", taskStatus, "extra assignee task should be removed")
}

func (s *ApprovalSuite) TestInstanceRemoveAssigneeNotFound() {
	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "remove_assignee",
			Version:  "v1",
		},
		Params: map[string]any{
			"taskId":     "nonexistent-task",
			"operatorId": "someone",
		},
	})

	body := s.readBody(resp)
	s.False(body.IsOk(), "remove_assignee should fail for nonexistent task")
}

func (s *ApprovalSuite) TestFlowPublishVersionAndGetGraphSuccess() {
	flowID, _ := s.deployAndPublishFlow("e2e-graph-flow", "cat-e2e-graph")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "get_graph",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowId": flowID,
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "get_graph should succeed after publish: %v", body)
	data := s.readDataAsMap(body.Data)

	s.Contains(data, "flow", "graph should contain flow")
	s.Contains(data, "version", "graph should contain version")
	s.Contains(data, "nodes", "graph should contain nodes")
	s.Contains(data, "edges", "graph should contain edges")

	nodes := s.readDataAsSlice(data["nodes"])
	s.NotEmpty(nodes, "should have nodes in the graph")

	edges := s.readDataAsSlice(data["edges"])
	s.NotEmpty(edges, "should have edges in the graph")
}

func (s *ApprovalSuite) TestInstanceFindInstancesWithFilter() {
	s.deployAndPublishFlow("e2e-find-inst-flow", "cat-e2e-find-inst")
	s.startInstance("e2e-find-inst-flow", "user-e2e-find")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "find_instances",
			Version:  "v1",
		},
		Params: map[string]any{
			"applicantId": "user-e2e-find",
			"status":      "running",
			"page":        1,
			"pageSize":    10,
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "find_instances with filter should succeed: %v", body)

	data := s.readDataAsMap(body.Data)
	list := s.readDataAsSlice(data["list"])
	s.NotEmpty(list, "should find at least one instance for the applicant")
}

func (s *ApprovalSuite) TestInstanceFindTasksWithFilter() {
	s.deployAndPublishFlow("e2e-find-task-flow", "cat-e2e-find-task")
	instanceID := s.startInstance("e2e-find-task-flow", "user-e2e-find-task")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "find_tasks",
			Version:  "v1",
		},
		Params: map[string]any{
			"assigneeId": "approver-001",
			"instanceId": instanceID,
			"status":     "pending",
			"page":       1,
			"pageSize":   10,
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "find_tasks with filter should succeed: %v", body)

	data := s.readDataAsMap(body.Data)
	list := s.readDataAsSlice(data["list"])
	s.NotEmpty(list, "should find at least one task for the assignee")
}

func (s *ApprovalSuite) TestInstanceProcessTaskTransfer() {
	s.deployAndPublishFlow("e2e-transfer-flow", "cat-e2e-transfer")
	instanceID := s.startInstance("e2e-transfer-flow", "user-e2e-transfer")

	taskID, _ := s.findPendingTask(instanceID, "approver-001")

	resp := s.makeAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/instance",
			Action:   "process_task",
			Version:  "v1",
		},
		Params: map[string]any{
			"instanceId":   instanceID,
			"taskId":       taskID,
			"action":       "transfer",
			"operatorId":   "approver-001",
			"opinion":      "转交给其他人",
			"transferToId": "approver-002",
		},
	})

	body := s.readBody(resp)
	s.True(body.IsOk(), "process_task (transfer) should succeed: %v", body)

	var origStatus string
	err := s.bunDB.NewSelect().
		TableExpr("apv_task").
		Column("status").
		Where("id = ?", taskID).
		Scan(s.ctx, &origStatus)
	s.Require().NoError(err)
	s.Equal("transferred", origStatus, "original task should be transferred")

	count, err := s.bunDB.NewSelect().
		TableExpr("apv_task").
		Where("instance_id = ?", instanceID).
		Where("assignee_id = ?", "approver-002").
		Where("status = ?", "pending").
		Count(s.ctx)
	s.Require().NoError(err)
	s.Equal(1, count, "should have a pending task for the transfer target")
}

// ---------------------------------------------------------------------------
// Test entry point
// ---------------------------------------------------------------------------

func TestApprovalSuite(t *testing.T) {
	suite.Run(t, new(ApprovalSuite))
}
