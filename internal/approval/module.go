package approval

import (
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/internal/approval/resource"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
)

// Module is the approval workflow engine module.
var Module = fx.Module(
	"vef:approval",

	strategy.Module,
	engine.Module,
	publisher.Module,
	service.Module,
	resource.Module,
)
