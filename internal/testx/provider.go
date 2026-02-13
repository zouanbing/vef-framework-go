package testx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/database"
	"github.com/ilxqx/vef-framework-go/internal/orm"
)

type dbProvider struct {
	dbType      config.DBType
	displayName string
	setup       func(t *testing.T) *DBEnv
}

var providers = []dbProvider{
	{dbType: config.Postgres, displayName: "Postgres", setup: setupPostgres},
	{dbType: config.MySQL, displayName: "MySQL", setup: setupMySQL},
	{dbType: config.SQLite, displayName: "SQLite", setup: setupSQLite},
}

// ForEachDB runs fn once per enabled database, managing container lifecycle automatically.
// Test hierarchy: t.Run("<DisplayName>", fn).
func ForEachDB(t *testing.T, fn func(t *testing.T, env *DBEnv)) {
	for _, p := range providers {
		t.Run(p.displayName, func(t *testing.T) {
			env := p.setup(t)
			fn(t, env)
		})
	}
}

// newDBEnv creates a complete DBEnv with database connection and automatic cleanup.
func newDBEnv(t *testing.T, ctx context.Context, dsConfig *config.DatasourceConfig) *DBEnv {
	db, err := database.New(dsConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Error closing database connection for %s: %v", dsConfig.Type, err)
		}
	})

	return &DBEnv{
		T:        t,
		Ctx:      ctx,
		BunDB:    db,
		DB:       orm.New(db),
		DBType:   dsConfig.Type,
		DsConfig: dsConfig,
	}
}

func setupPostgres(t *testing.T) *DBEnv {
	ctx := context.Background()
	c := NewPostgresContainer(ctx, t)
	return newDBEnv(t, ctx, c.DsConfig)
}

func setupMySQL(t *testing.T) *DBEnv {
	ctx := context.Background()
	c := NewMySQLContainer(ctx, t)
	return newDBEnv(t, ctx, c.DsConfig)
}

func setupSQLite(t *testing.T) *DBEnv {
	ctx := context.Background()
	dsConfig := &config.DatasourceConfig{Type: config.SQLite}
	return newDBEnv(t, ctx, dsConfig)
}
