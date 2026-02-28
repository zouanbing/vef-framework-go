package security

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/event"
	"github.com/ilxqx/vef-framework-go/httpx"
	"github.com/ilxqx/vef-framework-go/i18n"
	"github.com/ilxqx/vef-framework-go/result"
	"github.com/ilxqx/vef-framework-go/security"
)

// AuthResourceParams holds the dependencies for AuthResource construction.
type AuthResourceParams struct {
	fx.In

	AuthManager        security.AuthManager
	TokenGenerator     security.TokenGenerator
	UserInfoLoader     security.UserInfoLoader     `optional:"true"`
	ChallengeProviders []security.ChallengeProvider `group:"vef:security:challenge_providers"`
	Publisher          event.Publisher
	SecurityConfig     *config.SecurityConfig
}

// NewAuthResource creates a new authentication resource with the provided auth manager and token generator.
func NewAuthResource(params AuthResourceParams) api.Resource {
	return &AuthResource{
		authManager:        params.AuthManager,
		tokenGenerator:     params.TokenGenerator,
		userInfoLoader:     params.UserInfoLoader,
		challengeProviders: params.ChallengeProviders,
		publisher:          params.Publisher,
		Resource: api.NewRPCResource(
			"security/auth",
			api.WithOperations(
				api.OperationSpec{
					Action:    "login",
					Public:    true,
					RateLimit: &api.RateLimitConfig{Max: params.SecurityConfig.LoginRateLimit},
				},
				api.OperationSpec{
					Action:    "refresh",
					Public:    true,
					RateLimit: &api.RateLimitConfig{Max: params.SecurityConfig.RefreshRateLimit},
				},
				api.OperationSpec{
					Action: "logout",
				},
				api.OperationSpec{
					Action:    "resolve_challenge",
					Public:    true,
					RateLimit: &api.RateLimitConfig{Max: params.SecurityConfig.LoginRateLimit},
				},
				api.OperationSpec{
					Action: "get_user_info",
				},
			),
		),
	}
}

// AuthResource handles authentication-related API endpoints.
type AuthResource struct {
	api.Resource

	authManager        security.AuthManager
	tokenGenerator     security.TokenGenerator
	userInfoLoader     security.UserInfoLoader
	challengeProviders []security.ChallengeProvider
	publisher          event.Publisher
}

// LoginParams represents the request parameters for user login.
type LoginParams struct {
	api.P

	// Authentication contains user credentials
	security.Authentication
}

// Login authenticates a user and returns a LoginResult.
// When challenge providers are configured and applicable, the result contains
// a challenge token and pending challenges instead of auth tokens.
func (a *AuthResource) Login(ctx fiber.Ctx, params LoginParams) error {
	loginIP := httpx.GetIP(ctx)
	userAgent := ctx.Get(fiber.HeaderUserAgent)
	traceID := contextx.RequestID(ctx)
	username := params.Principal

	principal, err := a.authManager.Authenticate(ctx.Context(), params.Authentication)
	if err != nil {
		var (
			failReason string
			errorCode  int
		)

		if resErr, ok := result.AsErr(err); ok {
			failReason = resErr.Message
			errorCode = resErr.Code
		} else {
			failReason = err.Error()
			errorCode = result.ErrCodeUnknown
		}

		loginEvent := security.NewLoginEvent(security.LoginEventParams{
			AuthType:   params.Type,
			UserID:     "",
			Username:   username,
			LoginIP:    loginIP,
			UserAgent:  userAgent,
			TraceID:    traceID,
			IsOk:       false,
			FailReason: failReason,
			ErrorCode:  errorCode,
		})
		a.publisher.Publish(loginEvent)

		return err
	}

	// Evaluate challenges after successful authentication
	challenges, err := a.evaluateChallenges(ctx.Context(), principal)
	if err != nil {
		return err
	}

	// Publish login success event
	loginEvent := security.NewLoginEvent(security.LoginEventParams{
		AuthType:  params.Type,
		UserID:    principal.ID,
		Username:  username,
		LoginIP:   loginIP,
		UserAgent: userAgent,
		TraceID:   traceID,
		IsOk:      true,
	})
	a.publisher.Publish(loginEvent)

	if len(challenges) > 0 {
		pending := make([]string, len(challenges))
		for i, c := range challenges {
			pending[i] = c.Type
		}

		generator := a.tokenGenerator.(*JWTTokenGenerator)
		challengeToken, err := generator.GenerateChallengeToken(principal, pending, nil)
		if err != nil {
			return err
		}

		return result.Ok(&security.LoginResult{
			ChallengeToken: challengeToken,
			Challenges:     challenges,
		}).Response(ctx)
	}

	credentials, err := a.tokenGenerator.Generate(principal)
	if err != nil {
		return err
	}

	return result.Ok(&security.LoginResult{
		Tokens: credentials,
	}).Response(ctx)
}

// RefreshParams represents the request parameters for token refresh operation.
type RefreshParams struct {
	api.P

	RefreshToken string `json:"refreshToken"`
}

// Refresh refreshes the access token using a valid refresh token.
// User data reload logic is handled by JwtRefreshAuthenticator.
func (a *AuthResource) Refresh(ctx fiber.Ctx, params RefreshParams) error {
	principal, err := a.authManager.Authenticate(ctx.Context(), security.Authentication{
		Type:      AuthTypeRefresh,
		Principal: params.RefreshToken,
	})
	if err != nil {
		return err
	}

	credentials, err := a.tokenGenerator.Generate(principal)
	if err != nil {
		return err
	}

	return result.Ok(credentials).Response(ctx)
}

// Logout returns success immediately.
// Token invalidation should be handled on the client side by removing stored tokens.
func (*AuthResource) Logout(ctx fiber.Ctx) error {
	return result.Ok().Response(ctx)
}

// ResolveChallengeParams represents the request for resolving a login challenge.
type ResolveChallengeParams struct {
	api.P

	ChallengeToken string `json:"challengeToken"`
	Type           string `json:"type"`
	Response       any    `json:"response"`
}

// ResolveChallenge validates a user's response to a login challenge.
// On success, either issues real auth tokens (all challenges resolved)
// or returns a new challenge token with remaining challenges.
func (a *AuthResource) ResolveChallenge(ctx fiber.Ctx, params ResolveChallengeParams) error {
	if params.ChallengeToken == "" {
		return result.Err(
			i18n.T(result.ErrMessageChallengeTokenInvalid),
			result.WithCode(result.ErrCodeChallengeTokenInvalid),
			result.WithStatus(fiber.StatusUnauthorized),
		)
	}

	generator := a.tokenGenerator.(*JWTTokenGenerator)
	claims, err := generator.ParseChallengeToken(params.ChallengeToken)
	if err != nil {
		return result.Err(
			i18n.T(result.ErrMessageChallengeTokenInvalid),
			result.WithCode(result.ErrCodeChallengeTokenInvalid),
			result.WithStatus(fiber.StatusUnauthorized),
		)
	}

	// Validate the challenge type is in the pending list
	found := false
	for _, p := range claims.Pending {
		if p == params.Type {
			found = true
			break
		}
	}

	if !found {
		return result.Err(
			i18n.T(result.ErrMessageChallengeTypeInvalid),
			result.WithCode(result.ErrCodeChallengeTypeInvalid),
			result.WithStatus(fiber.StatusBadRequest),
		)
	}

	provider := a.findChallengeProvider(params.Type)
	if provider == nil {
		return result.Err(
			i18n.T(result.ErrMessageChallengeTypeInvalid),
			result.WithCode(result.ErrCodeChallengeTypeInvalid),
			result.WithStatus(fiber.StatusBadRequest),
		)
	}

	principal, err := provider.Resolve(ctx.Context(), claims.Principal, params.Response)
	if err != nil {
		return err
	}

	// Build updated pending list (remove resolved type)
	resolved := append(claims.Resolved, params.Type)
	var remaining []string
	for _, p := range claims.Pending {
		if p != params.Type {
			remaining = append(remaining, p)
		}
	}

	if len(remaining) == 0 {
		// All challenges resolved — issue real tokens
		credentials, err := a.tokenGenerator.Generate(principal)
		if err != nil {
			return err
		}

		return result.Ok(&security.LoginResult{
			Tokens: credentials,
		}).Response(ctx)
	}

	// More challenges remain — issue new ephemeral token
	challengeToken, err := generator.GenerateChallengeToken(principal, remaining, resolved)
	if err != nil {
		return err
	}

	// Re-evaluate remaining challenges to get fresh data
	var remainingChallenges []security.LoginChallenge
	for _, challengeType := range remaining {
		p := a.findChallengeProvider(challengeType)
		if p == nil {
			continue
		}

		challenge, err := p.Evaluate(ctx.Context(), principal)
		if err != nil {
			return err
		}

		if challenge != nil {
			remainingChallenges = append(remainingChallenges, *challenge)
		}
	}

	return result.Ok(&security.LoginResult{
		ChallengeToken: challengeToken,
		Challenges:     remainingChallenges,
	}).Response(ctx)
}

// GetUserInfo retrieves user information via UserInfoLoader.
// Requires a UserInfoLoader implementation to be provided.
func (a *AuthResource) GetUserInfo(ctx fiber.Ctx, principal *security.Principal, params api.Params) error {
	if a.userInfoLoader == nil {
		return result.ErrNotImplemented(i18n.T(result.ErrMessageUserInfoLoaderNotImplemented))
	}

	userInfo, err := a.userInfoLoader.LoadUserInfo(ctx.Context(), principal, params)
	if err != nil {
		return err
	}

	return result.Ok(userInfo).Response(ctx)
}

// evaluateChallenges collects all applicable challenges for the given principal.
func (a *AuthResource) evaluateChallenges(ctx context.Context, principal *security.Principal) ([]security.LoginChallenge, error) {
	if len(a.challengeProviders) == 0 {
		return nil, nil
	}

	var challenges []security.LoginChallenge
	for _, provider := range a.challengeProviders {
		challenge, err := provider.Evaluate(ctx, principal)
		if err != nil {
			return nil, err
		}

		if challenge != nil {
			challenges = append(challenges, *challenge)
		}
	}

	return challenges, nil
}

func (a *AuthResource) findChallengeProvider(challengeType string) security.ChallengeProvider {
	for _, provider := range a.challengeProviders {
		if provider.Type() == challengeType {
			return provider
		}
	}

	return nil
}
