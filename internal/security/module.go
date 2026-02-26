package security

import (
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/event"
	"github.com/ilxqx/vef-framework-go/internal/log"
	"github.com/ilxqx/vef-framework-go/password"
	"github.com/ilxqx/vef-framework-go/security"
)

var logger = log.Named("security")

var Module = fx.Module(
	"vef:security",
	fx.Decorate(func(cfg *config.SecurityConfig) *config.SecurityConfig {
		if cfg.RefreshNotBefore <= 0 {
			cfg.RefreshNotBefore = AccessTokenExpires / 2
		}

		if cfg.LoginRateLimit <= 0 {
			cfg.LoginRateLimit = 6
		}

		if cfg.RefreshRateLimit <= 0 {
			cfg.RefreshRateLimit = 1
		}

		return cfg
	}),
	fx.Decorate(
		fx.Annotate(
			func(loader security.RolePermissionsLoader, bus event.Bus) security.RolePermissionsLoader {
				if loader == nil {
					return nil
				}

				return security.NewCachedRolePermissionsLoader(loader, bus)
			},
			fx.ParamTags(`optional:"true"`),
		),
	),
	fx.Provide(
		password.NewBcryptEncoder,
		fx.Annotate(
			func(config *config.AppConfig) (*security.JWT, error) {
				return security.NewJWT(&security.JWTConfig{
					Audience: lo.SnakeCase(config.Name),
				})
			},
		),
		fx.Annotate(
			NewJWTAuthenticator,
			fx.ResultTags(`group:"vef:security:authenticators"`),
		),
		fx.Annotate(
			NewJWTRefreshAuthenticator,
			fx.ParamTags(``, `optional:"true"`),
			fx.ResultTags(`group:"vef:security:authenticators"`),
		),
		NewJWTTokenGenerator,
		fx.Annotate(
			NewSignatureAuthenticator,
			fx.ParamTags(`optional:"true"`, `optional:"true"`),
			fx.ResultTags(`group:"vef:security:authenticators"`),
		),
		fx.Annotate(
			NewPasswordAuthenticator,
			fx.ParamTags(`optional:"true"`, `optional:"true"`),
			fx.ResultTags(`group:"vef:security:authenticators"`),
		),
		fx.Annotate(
			NewAuthManager,
			fx.ParamTags(`group:"vef:security:authenticators"`),
		),
		fx.Annotate(
			NewRbacPermissionChecker,
			fx.ParamTags(`optional:"true"`),
		),
		fx.Annotate(
			NewRbacDataPermissionResolver,
			fx.ParamTags(`optional:"true"`),
		),
		fx.Annotate(
			NewAuthResource,
			fx.ResultTags(`group:"vef:api:resources"`),
		),
	),
)
