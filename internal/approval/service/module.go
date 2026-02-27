package service

import "go.uber.org/fx"

// Module provides all approval services.
var Module = fx.Module(
	"vef:approval:service",

	fx.Provide(
		NewTaskService,
		NewNodeService,
		NewValidationService,
	),
)
