package orm_test

import (
	"testing"

	"github.com/ilxqx/vef-framework-go/internal/testx"
)

// registry holds all ORM test suite factories, populated by init() functions in each suite file.
var registry = testx.NewRegistry[BaseTestSuite]()

// baseFactory creates an OrmTestSuite from a DBEnv — called once per database.
func baseFactory(env *testx.DBEnv) *BaseTestSuite {
	return &BaseTestSuite{
		ctx:   env.Ctx,
		db:    env.DB,
		rawDB: env.RawDB,
		ds:    env.DS,
	}
}

// TestAll runs every registered ORM suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}
