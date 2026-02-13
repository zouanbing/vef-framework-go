package publisher

import "go.uber.org/fx"

// Module provides the event publisher.
var Module = fx.Module(
	"vef:approval:publisher",

	fx.Provide(NewEventPublisher),
)
