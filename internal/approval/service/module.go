package service

import "go.uber.org/fx"

// Module provides all approval services.
var Module = fx.Module(
	"vef:approval:service",

	fx.Provide(
		// Legacy services (kept until resource layer is refactored).
		NewFlowService,
		NewInstanceService,
		NewQueryService,

		// Domain services.
		NewTaskService,
		NewNodeService,
		NewValidationService,
	),
)
