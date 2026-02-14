package testx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/database"
	"github.com/ilxqx/vef-framework-go/internal/orm"
)

// dbSetupFunc creates a DataSourceConfig (spinning up a container if needed).
type dbSetupFunc func(ctx context.Context, t *testing.T) *config.DataSourceConfig

var providers = []struct {
	name  string
	setup dbSetupFunc
}{
	{"Postgres", func(ctx context.Context, t *testing.T) *config.DataSourceConfig {
		return NewPostgresContainer(ctx, t).DsConfig
	}},
	{"MySQL", func(ctx context.Context, t *testing.T) *config.DataSourceConfig {
		return NewMySQLContainer(ctx, t).DsConfig
	}},
	{"SQLite", func(_ context.Context, _ *testing.T) *config.DataSourceConfig {
		return &config.DataSourceConfig{Kind: config.SQLite}
	}},
}

// ForEachDB runs fn once per enabled database, managing container lifecycle automatically.
// Test hierarchy: t.Run("<DisplayName>", fn).
func ForEachDB(t *testing.T, fn func(t *testing.T, env *DBEnv)) {
	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			ctx := context.Background()
			dsConfig := p.setup(ctx, t)
			env := newDBEnv(t, ctx, dsConfig)
			fn(t, env)
		})
	}
}

// newDBEnv creates a complete DBEnv with database connection and automatic cleanup.
func newDBEnv(t *testing.T, ctx context.Context, dsConfig *config.DataSourceConfig) *DBEnv {
	db, err := database.New(dsConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Error closing database connection for %s: %v", dsConfig.Kind, err)
		}
	})

	return &DBEnv{
		T:        t,
		Ctx:      ctx,
		BunDB:    db,
		DB:       orm.New(db),
		DBKind:   dsConfig.Kind,
		DsConfig: dsConfig,
	}
}
