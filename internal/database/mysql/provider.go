package mysql

import (
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/samber/lo"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/schema"

	"github.com/ilxqx/vef-framework-go/config"
)

type Provider struct {
	dbType config.DBType
}

func NewProvider() *Provider {
	return &Provider{
		dbType: config.MySQL,
	}
}

func (p *Provider) Type() config.DBType {
	return p.dbType
}

func (p *Provider) Connect(cfg *config.DatasourceConfig) (*sql.DB, schema.Dialect, error) {
	if err := p.ValidateConfig(cfg); err != nil {
		return nil, nil, err
	}

	connector, err := mysql.NewConnector(p.buildConfig(cfg))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create mysql connector: %w", err)
	}

	return sql.OpenDB(connector), mysqldialect.New(), nil
}

func (*Provider) ValidateConfig(cfg *config.DatasourceConfig) error {
	if cfg.Database == "" {
		return ErrMySQLDatabaseRequired
	}

	return nil
}

func (*Provider) QueryVersion(db *bun.DB) (string, error) {
	return queryVersion(db)
}

func (*Provider) buildConfig(cfg *config.DatasourceConfig) *mysql.Config {
	mysqlCfg := mysql.NewConfig()
	mysqlCfg.User = lo.Ternary(cfg.User != "", cfg.User, "root")
	mysqlCfg.Passwd = cfg.Password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = fmt.Sprintf(
		"%s:%d",
		lo.Ternary(cfg.Host != "", cfg.Host, "127.0.0.1"),
		lo.Ternary(cfg.Port != 0, cfg.Port, uint16(3306)),
	)
	mysqlCfg.DBName = cfg.Database
	mysqlCfg.ParseTime = true
	mysqlCfg.Collation = "utf8mb4_unicode_ci"

	return mysqlCfg
}
