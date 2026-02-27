package approval

import (
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/internal/approval/behavior"
	"github.com/ilxqx/vef-framework-go/internal/approval/command"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/migration"
	"github.com/ilxqx/vef-framework-go/internal/approval/query"
	"github.com/ilxqx/vef-framework-go/internal/approval/resource"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/internal/approval/timeout"
)

// Module is the approval workflow engine module.
var Module = fx.Module(
	"vef:approval",

	strategy.Module,
	behavior.Module,
	engine.Module,
	dispatcher.Module,
	service.Module,
	command.Module,
	query.Module,
	resource.Module,
	timeout.Module,
	migration.Module,
)
