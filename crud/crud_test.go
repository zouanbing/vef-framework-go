package crud_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dbfixture"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/testx"
)

// registry holds all CRUD test suite factories, populated by init() functions in each suite file.
var registry = testx.NewRegistry[BaseSuite]()

// baseFactory creates a BaseSuite from a DBEnv — called once per database.
func baseFactory(env *testx.DBEnv) *BaseSuite {
	setupTestFixtures(env.T, env.Ctx, env.BunDB, env.DBType)

	return &BaseSuite{
		ctx:      env.Ctx,
		db:       env.DB,
		dbType:   env.DBType,
		dsConfig: env.DsConfig,
	}
}

// TestAll runs every registered CRUD suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}

// setupTestFixtures loads test data from fixture files using dbfixture.
func setupTestFixtures(t *testing.T, ctx context.Context, db bun.IDB, dbType config.DBType) {
	t.Helper()
	t.Logf("Setting up test fixtures for %s", dbType)

	bunDB, ok := db.(*bun.DB)
	if !ok {
		require.Fail(t, "Could not convert to *bun.DB")
	}

	// Register models
	bunDB.RegisterModel(
		(*TestAuditUser)(nil),
		(*TestUser)(nil),
		(*TestCategory)(nil),
		(*TestCompositePKItem)(nil),
		(*ExportUser)(nil),
		(*ImportUser)(nil),
	)

	// Create fixture loader with template functions
	fixture := dbfixture.New(
		bunDB,
		dbfixture.WithRecreateTables(),
	)

	// Load fixtures from testdata directory
	err := fixture.Load(ctx, os.DirFS("testdata"), "fixture.yaml")
	require.NoError(t, err, "Failed to load fixtures for %s", dbType)

	t.Logf("Test fixtures loaded for %s database", dbType)
}
