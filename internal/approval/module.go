package approval

import (
	"context"

	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/migration"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/resource"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/internal/approval/timeout"
	"github.com/ilxqx/vef-framework-go/orm"
)

// Module is the approval workflow engine module.
var Module = fx.Module(
	"vef:approval",

	strategy.Module,
	engine.Module,
	dispatcher.Module,
	service.Module,
	resource.Module,
	timeout.Module,

	fx.Invoke(autoMigrate),
)

func autoMigrate(ctx context.Context, cfg *config.ApprovalConfig, db orm.DB, ds *config.DataSourceConfig) error {
	if !cfg.AutoMigrate {
		return nil
	}

	return migration.Migrate(ctx, db, ds.Kind)
}
