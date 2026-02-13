package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/schema"

	"github.com/ilxqx/vef-framework-go/config"
)

type Provider struct {
	dbType config.DBType
}

func NewProvider() *Provider {
	return &Provider{
		dbType: config.SQLite,
	}
}

func (p *Provider) Type() config.DBType {
	return p.dbType
}

func (p *Provider) Connect(cfg *config.DataSourceConfig) (*sql.DB, schema.Dialect, error) {
	if err := p.ValidateConfig(cfg); err != nil {
		return nil, nil, err
	}

	db, err := sql.Open(sqliteshim.ShimName, p.buildDsn(cfg))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	return db, sqlitedialect.New(), nil
}

func (*Provider) ValidateConfig(_ *config.DataSourceConfig) error {
	return nil
}

func (*Provider) QueryVersion(db *bun.DB) (string, error) {
	return queryVersion(db)
}

// buildDsn returns the DSN for SQLite. When no path is specified, it uses
// file::memory: with shared cache to ensure multiple connections share
// the same in-memory database.
func (*Provider) buildDsn(cfg *config.DataSourceConfig) string {
	if cfg.Path == "" {
		return "file::memory:?mode=memory&cache=shared"
	}

	return "file:" + cfg.Path
}
