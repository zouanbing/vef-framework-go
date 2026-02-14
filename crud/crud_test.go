package crud_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/orm"
	"github.com/ilxqx/vef-framework-go/internal/testx"
)

// registry holds all CRUD test suite factories, populated by init() in each suite file.
// Execution order is irrelevant: each suite reloads fixtures in SetupSuite.
var registry = testx.NewRegistry[BaseTestSuite]()

// baseFactory creates a BaseSuite from a DBEnv — called once per database.
func baseFactory(env *testx.DBEnv) *BaseTestSuite {
	setupTestFixtures(env.T, env.Ctx, env.DB, env.RawDB, env.DS.Kind)

	return &BaseTestSuite{
		ctx:   env.Ctx,
		db:    env.DB,
		bunDB: env.BunDB,
		ds:    env.DS,
	}
}

// TestAll runs every registered CRUD suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}

// setupTestFixtures creates tables and loads test data from fixture files.
func setupTestFixtures(t *testing.T, ctx context.Context, db orm.DB, rawDB *sql.DB, kind config.DBKind) {
	t.Helper()

	models := []any{
		(*TestAuditUser)(nil),
		(*TestUser)(nil),
		(*TestCategory)(nil),
		(*TestCompositePKItem)(nil),
		(*ExportUser)(nil),
		(*ImportUser)(nil),
	}

	db.RegisterModel(models...)
	require.NoError(t, db.ResetModel(ctx, models...), "Failed to reset models for %s", kind)

	fixtures, err := testfixtures.New(
		testfixtures.Database(rawDB),
		testfixtures.Dialect(string(kind)),
		testfixtures.Directory("fixtures"),
		testfixtures.DangerousSkipTestDatabaseCheck(),
	)
	require.NoError(t, err, "Failed to create fixtures loader for %s", kind)
	require.NoError(t, fixtures.Load(), "Failed to load fixtures for %s", kind)
}
