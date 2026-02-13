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

	bunDB, ok := db.(*bun.DB)
	require.True(t, ok, "Expected *bun.DB, got %T", db)

	bunDB.RegisterModel(
		(*TestAuditUser)(nil),
		(*TestUser)(nil),
		(*TestCategory)(nil),
		(*TestCompositePKItem)(nil),
		(*ExportUser)(nil),
		(*ImportUser)(nil),
	)

	fixture := dbfixture.New(bunDB, dbfixture.WithRecreateTables())
	err := fixture.Load(ctx, os.DirFS("testdata"), "fixture.yaml")
	require.NoError(t, err, "Failed to load fixtures for %s", dbType)
}
