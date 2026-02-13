package testx

import (
	"context"
	"testing"

	"github.com/uptrace/bun"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/orm"
)

// DBEnv encapsulates the database environment for cross-database integration tests.
// Contains both the raw bun.IDB connection and a wrapped orm.DB for convenience.
type DBEnv struct {
	T        *testing.T
	Ctx      context.Context
	BunDB    bun.IDB
	DB       orm.DB
	DBType   config.DBType
	DsConfig *config.DataSourceConfig
}
