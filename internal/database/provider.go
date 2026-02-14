package database

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/database/mysql"
	"github.com/ilxqx/vef-framework-go/internal/database/postgres"
	"github.com/ilxqx/vef-framework-go/internal/database/sqlite"
)

type DatabaseProvider interface {
	Connect(config *config.DataSourceConfig) (*sql.DB, schema.Dialect, error)
	Kind() config.DBKind
	ValidateConfig(config *config.DataSourceConfig) error
	QueryVersion(db *bun.DB) (string, error)
}

type providerRegistry struct {
	providers map[config.DBKind]DatabaseProvider
}

func newProviderRegistry() *providerRegistry {
	registry := &providerRegistry{
		providers: make(map[config.DBKind]DatabaseProvider),
	}

	registry.register(sqlite.NewProvider())
	registry.register(postgres.NewProvider())
	registry.register(mysql.NewProvider())

	return registry
}

func (r *providerRegistry) register(provider DatabaseProvider) {
	r.providers[provider.Kind()] = provider
}

func (r *providerRegistry) provider(dbKind config.DBKind) (DatabaseProvider, bool) {
	provider, exists := r.providers[dbKind]

	return provider, exists
}

var registry = newProviderRegistry()
