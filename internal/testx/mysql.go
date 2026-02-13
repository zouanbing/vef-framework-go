package testx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ilxqx/vef-framework-go/config"
)

type MySQLContainer struct {
	DsConfig *config.DataSourceConfig
}

func NewMySQLContainer(ctx context.Context, t testing.TB) *MySQLContainer {
	t.Helper()

	container, err := mysql.Run(
		ctx,
		MySQLImage,
		mysql.WithDatabase(TestDatabaseName),
		mysql.WithUsername(TestUsername),
		mysql.WithPassword(TestPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("port: 3306  MySQL Community Server - GPL").
				WithStartupTimeout(DefaultContainerTimeout),
		),
	)
	require.NoError(t, err)
	t.Log("MySQL container started successfully")

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate mysql container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "3306")
	require.NoError(t, err)

	return &MySQLContainer{
		DsConfig: &config.DataSourceConfig{
			Type:     "mysql",
			Host:     host,
			Port:     uint16(port.Int()),
			User:     TestUsername,
			Password: TestPassword,
			Database: TestDatabaseName,
		},
	}
}
