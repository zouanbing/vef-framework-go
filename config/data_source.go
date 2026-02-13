package config

// DBType represents supported database types.
type DBType string

// Supported database types.
const (
	Oracle    DBType = "oracle"
	SQLServer DBType = "sqlserver"
	Postgres  DBType = "postgres"
	MySQL     DBType = "mysql"
	SQLite    DBType = "sqlite"
)

// DataSourceConfig defines database connection settings.
type DataSourceConfig struct {
	Type           DBType `config:"type"`
	Host           string           `config:"host"`
	Port           uint16           `config:"port"`
	User           string           `config:"user"`
	Password       string           `config:"password"`
	Database       string           `config:"database"`
	Schema         string           `config:"schema"`
	Path           string           `config:"path"`
	EnableSQLGuard bool             `config:"enable_sql_guard"`
}
