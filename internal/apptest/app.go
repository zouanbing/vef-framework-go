package apptest

import (
	"context"
	"database/sql"
	"testing"

	"github.com/uptrace/bun"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/api"
	"github.com/ilxqx/vef-framework-go/internal/app"
	iconfig "github.com/ilxqx/vef-framework-go/internal/config"
	"github.com/ilxqx/vef-framework-go/internal/cron"
	"github.com/ilxqx/vef-framework-go/internal/database"
	"github.com/ilxqx/vef-framework-go/internal/event"
	"github.com/ilxqx/vef-framework-go/internal/mcp"
	"github.com/ilxqx/vef-framework-go/internal/middleware"
	"github.com/ilxqx/vef-framework-go/internal/mold"
	"github.com/ilxqx/vef-framework-go/internal/monitor"
	"github.com/ilxqx/vef-framework-go/internal/orm"
	"github.com/ilxqx/vef-framework-go/internal/redis"
	"github.com/ilxqx/vef-framework-go/internal/schema"
	"github.com/ilxqx/vef-framework-go/internal/security"
	"github.com/ilxqx/vef-framework-go/internal/storage"
)

// MockConfig implements config.Config for testing without file dependencies.
type MockConfig struct{}

func (*MockConfig) Unmarshal(_ string, _ any) error {
	return nil
}

// NewTestApp creates a test application with Fx dependency injection.
// Returns the app instance and a cleanup function.
func NewTestApp(t testing.TB, options ...fx.Option) (*app.App, func()) {
	return newTestApp(t, buildOptions(options...))
}

// NewTestAppWithDB creates a test application that uses an existing *bun.DB
// instead of creating a new connection via database.Module.
// This avoids redundant database connections when tests already manage their own.
func NewTestAppWithDB(t testing.TB, db *bun.DB, options ...fx.Option) (*app.App, func()) {
	return newTestApp(t, buildOptionsWithDB(db, options...))
}

// NewTestAppWithErr creates a test application and returns any startup errors.
// Useful for testing error conditions during app initialization.
func NewTestAppWithErr(t testing.TB, options ...fx.Option) (*app.App, func(), error) {
	return newTestAppWithErr(t, buildOptions(options...))
}

// NewTestAppWithDBAndErr creates a test application with an existing *bun.DB
// and returns any startup errors.
func NewTestAppWithDBAndErr(t testing.TB, db *bun.DB, options ...fx.Option) (*app.App, func(), error) {
	return newTestAppWithErr(t, buildOptionsWithDB(db, options...))
}

func newTestApp(t testing.TB, opts []fx.Option) (*app.App, func()) {
	var testApp *app.App

	opts = append(opts, fx.Populate(&testApp))
	fxApp := fxtest.New(t, opts...)
	fxApp.RequireStart()

	return testApp, fxApp.RequireStop
}

func newTestAppWithErr(t testing.TB, opts []fx.Option) (*app.App, func(), error) {
	var testApp *app.App

	opts = append(opts, fx.Populate(&testApp))
	fxApp := fx.New(opts...)

	startCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	err := fxApp.Start(startCtx)
	cleanup := createCleanupFunc(t, fxApp)

	return testApp, cleanup, err
}

func createCleanupFunc(t testing.TB, fxApp *fx.App) func() {
	return func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
		defer cancel()

		if err := fxApp.Stop(stopCtx); err != nil {
			t.Logf("Failed to stop app: %v", err)
		}
	}
}

func coreOptions() []fx.Option {
	return []fx.Option{
		fx.NopLogger,
		fx.Replace(
			fx.Annotate(&MockConfig{}, fx.As(new(config.Config))),
			&config.AppConfig{
				Name:      "test-app",
				Port:      0,
				BodyLimit: "100mib",
			},
		),
		iconfig.Module,
		orm.Module,
		middleware.Module,
		api.Module,
		security.Module,
		event.Module,
		cron.Module,
		redis.Module,
		mold.Module,
		storage.Module,
		monitor.Module,
		schema.Module,
		mcp.Module,
		app.Module,
	}
}

func buildOptions(options ...fx.Option) []fx.Option {
	opts := append(coreOptions(), database.Module)
	return append(opts, options...)
}

func buildOptionsWithDB(db *bun.DB, options ...fx.Option) []fx.Option {
	dbProvider := fx.Provide(
		fx.Annotate(
			func() *bun.DB { return db },
			fx.As(new(bun.IDB)),
			fx.As(fx.Self()),
		),
		func(db *bun.DB) *sql.DB { return db.DB },
	)

	opts := append(coreOptions(), dbProvider)
	return append(opts, options...)
}
