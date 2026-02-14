package crud_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/guregu/null/v6"
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go"
	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/encoding"
	"github.com/ilxqx/vef-framework-go/internal/app"
	"github.com/ilxqx/vef-framework-go/internal/apptest"
	"github.com/ilxqx/vef-framework-go/internal/orm"
	"github.com/ilxqx/vef-framework-go/result"
)

// TestAuditUser is a test model for audit user.
type TestAuditUser struct {
	bun.BaseModel `bun:"table:test_audit_user,alias:tau"`
	orm.Model

	Name string `json:"name" bun:",notnull"`
}

// TestUser is a test model for all tests.
type TestUser struct {
	bun.BaseModel `bun:"table:test_user,alias:tu"`
	orm.Model

	Name        string `json:"name"        bun:",notnull"`
	Email       string `json:"email"       bun:",unique,notnull"`
	Description string `json:"description"`
	Age         int    `json:"age"         bun:",notnull"`
	Status      string `json:"status"      bun:",notnull,default:'active'"`
}

// TestUserSearch is the search parameters for TestUser.
type TestUserSearch struct {
	api.P

	ID      null.String `json:"id"      search:"eq"`
	Keyword null.String `json:"keyword" search:"contains,column=name|description"`
	Email   null.String `json:"email"   search:"eq"`
	Status  null.String `json:"status"  search:"eq"`
	Age     []int       `json:"age"     search:"between"`
}

// TestCategory is a test model for tree-based tests.
type TestCategory struct {
	bun.BaseModel `bun:"table:test_category,alias:tc"`
	orm.Model

	Name        string  `json:"name"               bun:",notnull"`
	Code        string  `json:"code"               bun:",unique,notnull"`
	Description string  `json:"description"`
	ParentID    *string `json:"parentId"`
	Sort        int     `json:"sort"               bun:",notnull,default:0"`
	Children    any     `json:"children,omitempty" bun:"-"`
}

// TestCategorySearch is the search parameters for TestCategory.
type TestCategorySearch struct {
	api.P

	ID       null.String `json:"id"       search:"eq"`
	Keyword  null.String `json:"keyword"  search:"contains,column=name|description"`
	Code     null.String `json:"code"     search:"eq"`
	ParentID null.String `json:"parentId" search:"eq"`
}

// TestCompositePKItem is a test model with composite primary keys.
type TestCompositePKItem struct {
	bun.BaseModel `bun:"table:test_composite_pk_item,alias:tcpi"`

	TenantID  string `json:"tenantId"  bun:",pk,notnull"`
	ItemCode  string `json:"itemCode"  bun:",pk,notnull"`
	Name      string `json:"name"      bun:",notnull"`
	Quantity  int    `json:"quantity"  bun:",notnull,default:0"`
	Status    string `json:"status"    bun:",notnull,default:'active'"`
	CreatedAt string `json:"createdAt" bun:",notnull"`
	CreatedBy string `json:"createdBy" bun:",notnull"`
}

type BaseSuite struct {
	suite.Suite

	ctx      context.Context
	app      *app.App
	stop     func()
	db       orm.DB
	bunDB    bun.IDB
	dbKind   config.DBKind
	dsConfig *config.DataSourceConfig
}

// clone creates a shallow copy of BaseSuite without copying the embedded suite.Suite
// (which contains sync.RWMutex). Each cloned suite gets a fresh suite.Suite.
func (suite *BaseSuite) clone() BaseSuite {
	return BaseSuite{
		ctx:      suite.ctx,
		db:       suite.db,
		bunDB:    suite.bunDB,
		dbKind:   suite.dbKind,
		dsConfig: suite.dsConfig,
	}
}

func (suite *BaseSuite) setupBaseSuite(resourceCtors ...any) {
	// Reload fixtures so each suite starts from a clean, known state
	// regardless of execution order.
	setupTestFixtures(suite.T(), suite.ctx, suite.bunDB, suite.dbKind)

	opts := make([]fx.Option, 0, len(resourceCtors)+2)
	for _, ctor := range resourceCtors {
		opts = append(opts, vef.ProvideAPIResource(ctor))
	}
	opts = append(opts,
		fx.Replace(suite.dsConfig),
		fx.Decorate(func() bun.IDB { return suite.bunDB }),
	)

	suite.app, suite.stop = apptest.NewTestApp(suite.T(), opts...)
}

func (suite *BaseSuite) tearDownBaseSuite() {
	if suite.stop != nil {
		suite.stop()
	}
}

func (suite *BaseSuite) makeAPIRequest(body api.Request) *http.Response {
	jsonBody, err := encoding.ToJSON(body)
	suite.Require().NoError(err)

	req := httptest.NewRequest(fiber.MethodPost, "/api", strings.NewReader(jsonBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := suite.app.Test(req)
	suite.Require().NoError(err)

	return resp
}

func (suite *BaseSuite) readBody(resp *http.Response) result.Result {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	res, err := encoding.FromJSON[result.Result](string(body))
	suite.Require().NoError(err)

	return *res
}

func (suite *BaseSuite) readDataAsSlice(data any) []any {
	slice, ok := data.([]any)
	suite.Require().True(ok, "Expected data to be a slice")

	return slice
}

func (suite *BaseSuite) readDataAsMap(data any) map[string]any {
	m, ok := data.(map[string]any)
	suite.Require().True(ok, "Expected data to be a map")

	return m
}
