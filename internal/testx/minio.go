package testx

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ilxqx/vef-framework-go/config"
)

type MinIOContainer struct {
	MinIOConfig *config.MinIOConfig
}

func NewMinIOContainer(ctx context.Context, t testing.TB) *MinIOContainer {
	t.Helper()

	container, err := minio.Run(
		ctx,
		MinIOImage,
		minio.WithUsername(TestMinIOAccessKey),
		minio.WithPassword(TestMinIOSecretKey),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForListeningPort("9000/tcp"),
				wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
				wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp"),
			).WithDeadline(DefaultContainerTimeout),
		),
	)
	require.NoError(t, err)
	t.Log("MinIO container started successfully")

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate MinIO container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	return &MinIOContainer{
		MinIOConfig: &config.MinIOConfig{
			Endpoint:  fmt.Sprintf("%s:%s", host, port.Port()),
			AccessKey: TestMinIOAccessKey,
			SecretKey: TestMinIOSecretKey,
			UseSSL:    false,
			Bucket:    TestMinioBucket,
		},
	}
}
