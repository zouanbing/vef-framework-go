package security

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cast"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/result"
	"github.com/ilxqx/vef-framework-go/security"
)

const (
	TokenTypeAccess    = "access"
	TokenTypeRefresh   = "refresh"
	TokenTypeChallenge = "challenge"
	AccessTokenExpires = time.Minute * 30
	ChallengeTokenExpires = 5 * time.Minute
)

const (
	claimPending  = "pnd"
	claimResolved = "rsd"
)

// ChallengeTokenClaims holds the parsed challenge token state.
type ChallengeTokenClaims struct {
	Principal *security.Principal
	Pending   []string
	Resolved  []string
}

type JWTTokenGenerator struct {
	jwt              *security.JWT
	tokenExpires     time.Duration
	refreshNotBefore time.Duration
}

func NewJWTTokenGenerator(jwt *security.JWT, securityConfig *config.SecurityConfig) security.TokenGenerator {
	return &JWTTokenGenerator{
		jwt:              jwt,
		tokenExpires:     securityConfig.TokenExpires,
		refreshNotBefore: securityConfig.RefreshNotBefore,
	}
}

func (g *JWTTokenGenerator) Generate(principal *security.Principal) (*security.AuthTokens, error) {
	jwtID := id.GenerateUUID()

	accessToken, err := g.generateAccessToken(jwtID, principal)
	if err != nil {
		logger.Errorf("Failed to generate access token for principal %q: %v", principal.ID, err)

		return nil, err
	}

	refreshToken, err := g.generateRefreshToken(jwtID, principal)
	if err != nil {
		logger.Errorf("Failed to generate refresh token for principal %q: %v", principal.ID, err)

		return nil, err
	}

	return &security.AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateAccessToken encodes id@name in subject to avoid DB lookups during authentication.
func (g *JWTTokenGenerator) generateAccessToken(jwtID string, principal *security.Principal) (string, error) {
	claimsBuilder := security.NewJWTClaimsBuilder().
		WithID(jwtID).
		WithSubject(fmt.Sprintf("%s@%s", principal.ID, principal.Name)).
		WithRoles(principal.Roles).
		WithDetails(principal.Details).
		WithType(TokenTypeAccess)

	return g.jwt.Generate(claimsBuilder, AccessTokenExpires, 0)
}

func (g *JWTTokenGenerator) generateRefreshToken(jwtID string, principal *security.Principal) (string, error) {
	claimsBuilder := security.NewJWTClaimsBuilder().
		WithID(jwtID).
		WithSubject(fmt.Sprintf("%s@%s", principal.ID, principal.Name)).
		WithType(TokenTypeRefresh)

	return g.jwt.Generate(claimsBuilder, g.tokenExpires, g.refreshNotBefore)
}

// GenerateChallengeToken creates a short-lived ephemeral JWT that tracks challenge state.
func (g *JWTTokenGenerator) GenerateChallengeToken(principal *security.Principal, pending, resolved []string) (string, error) {
	claimsBuilder := security.NewJWTClaimsBuilder().
		WithID(id.GenerateUUID()).
		WithSubject(fmt.Sprintf("%s@%s", principal.ID, principal.Name)).
		WithRoles(principal.Roles).
		WithDetails(principal.Details).
		WithType(TokenTypeChallenge).
		WithClaim(claimPending, pending).
		WithClaim(claimResolved, resolved)

	return g.jwt.Generate(claimsBuilder, ChallengeTokenExpires, 0)
}

// ParseChallengeToken parses an ephemeral challenge token and returns the challenge state.
func (g *JWTTokenGenerator) ParseChallengeToken(token string) (*ChallengeTokenClaims, error) {
	claimsAccessor, err := g.jwt.Parse(token)
	if err != nil {
		return nil, err
	}

	if claimsAccessor.Type() != TokenTypeChallenge {
		return nil, result.ErrTokenInvalid
	}

	subjectParts := strings.SplitN(claimsAccessor.Subject(), "@", 2)
	if len(subjectParts) < 2 {
		return nil, result.ErrTokenInvalid
	}

	principal := security.NewUser(subjectParts[0], subjectParts[1], claimsAccessor.Roles()...)
	principal.AttemptUnmarshalDetails(claimsAccessor.Details())

	return &ChallengeTokenClaims{
		Principal: principal,
		Pending:   cast.ToStringSlice(claimsAccessor.Claim(claimPending)),
		Resolved:  cast.ToStringSlice(claimsAccessor.Claim(claimResolved)),
	}, nil
}
