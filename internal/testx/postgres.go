package testx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ilxqx/vef-framework-go/config"
)

type PostgresContainer struct {
	DsConfig *config.DataSourceConfig
}

func NewPostgresContainer(ctx context.Context, t testing.TB) *PostgresContainer {
	t.Helper()

	container, err := postgres.Run(
		ctx,
		PostgresImage,
		postgres.WithDatabase(TestDatabaseName),
		postgres.WithUsername(TestUsername),
		postgres.WithPassword(TestPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(DefaultContainerTimeout),
		),
	)
	require.NoError(t, err)
	t.Log("PostgreSQL container started successfully")

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate postgres container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	return &PostgresContainer{
		DsConfig: &config.DataSourceConfig{
			Kind:     "postgres",
			Host:     host,
			Port:     uint16(port.Int()),
			User:     TestUsername,
			Password: TestPassword,
			Database: TestDatabaseName,
		},
	}
}
