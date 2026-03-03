package engine_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/approval/migration"
	"github.com/ilxqx/vef-framework-go/internal/testx"
)

// registry holds all engine test suite factories, populated by init() in each suite file.
var registry = testx.NewRegistry[testx.DBEnv]()

// baseFactory runs approval migrations and returns the DBEnv.
func baseFactory(env *testx.DBEnv) *testx.DBEnv {
	if env.DS.Kind != config.Postgres {
		env.T.Skip("Engine suite tests only run on PostgreSQL")
	}

	require.NoError(env.T, migration.Migrate(env.Ctx, env.DB, env.DS.Kind), "Should run approval migration")

	return env
}

// TestAll runs every registered engine suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}
