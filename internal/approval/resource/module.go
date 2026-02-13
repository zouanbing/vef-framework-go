package resource

import (
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/api"
)

// Module provides all approval API resources.
var Module = fx.Module(
	"vef:approval:resource",

	fx.Provide(
		fx.Annotate(NewFlowResource, fx.As(new(api.Resource)), fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewInstanceResource, fx.As(new(api.Resource)), fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewCategoryResource, fx.As(new(api.Resource)), fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewDelegationResource, fx.As(new(api.Resource)), fx.ResultTags(`group:"vef:api:resources"`)),
	),
)
