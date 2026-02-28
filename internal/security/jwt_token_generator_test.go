package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/security"
)

type JWTTokenGeneratorTestSuite struct {
	suite.Suite
	generator *JWTTokenGenerator
}

func (s *JWTTokenGeneratorTestSuite) SetupSuite() {
	jwt, err := security.NewJWT(&security.JWTConfig{
		Secret:   security.DefaultJWTSecret,
		Audience: security.DefaultJWTAudience,
	})
	require.NoError(s.T(), err, "Should create JWT instance without error")

	tokenGen := NewJWTTokenGenerator(jwt, &config.SecurityConfig{
		TokenExpires:     24 * time.Hour,
		RefreshNotBefore: 20 * time.Minute,
	})

	var ok bool
	s.generator, ok = tokenGen.(*JWTTokenGenerator)
	require.True(s.T(), ok, "Should type-assert to *JWTTokenGenerator")
}

// TestGenerateAndParseChallengeToken verifies round-trip generation and parsing of a challenge token.
func (s *JWTTokenGeneratorTestSuite) TestGenerateAndParseChallengeToken() {
	principal := security.NewUser("user123", "Alice", "admin", "editor")
	pending := []string{"totp", "select_department"}
	resolved := []string{}

	token, err := s.generator.GenerateChallengeToken(principal, pending, resolved)
	require.NoError(s.T(), err, "Should generate challenge token without error")
	require.NotEmpty(s.T(), token, "Should return a non-empty token string")

	claims, err := s.generator.ParseChallengeToken(token)
	require.NoError(s.T(), err, "Should parse challenge token without error")
	require.NotNil(s.T(), claims, "Should return non-nil claims")

	assert.Equal(s.T(), "user123", claims.Principal.ID, "Should preserve principal ID")
	assert.Equal(s.T(), "Alice", claims.Principal.Name, "Should preserve principal name")
	assert.Equal(s.T(), []string{"admin", "editor"}, claims.Principal.Roles, "Should preserve principal roles")
	assert.Equal(s.T(), []string{"totp", "select_department"}, claims.Pending, "Should preserve pending challenges")
	assert.Empty(s.T(), claims.Resolved, "Should have empty resolved list")
}

// TestParseChallengeTokenWithResolvedState verifies parsing a token that has both pending and resolved challenges.
func (s *JWTTokenGeneratorTestSuite) TestParseChallengeTokenWithResolvedState() {
	principal := security.NewUser("user456", "Bob", "viewer")
	pending := []string{"totp"}
	resolved := []string{"select_department"}

	token, err := s.generator.GenerateChallengeToken(principal, pending, resolved)
	require.NoError(s.T(), err, "Should generate challenge token without error")

	claims, err := s.generator.ParseChallengeToken(token)
	require.NoError(s.T(), err, "Should parse challenge token without error")
	require.NotNil(s.T(), claims, "Should return non-nil claims")

	assert.Equal(s.T(), []string{"totp"}, claims.Pending, "Should have one pending challenge")
	assert.Equal(s.T(), []string{"select_department"}, claims.Resolved, "Should have one resolved challenge")
}

// TestParseChallengeTokenRejectsAccessToken verifies that an access token cannot be parsed as a challenge token.
func (s *JWTTokenGeneratorTestSuite) TestParseChallengeTokenRejectsAccessToken() {
	principal := security.NewUser("user789", "Charlie", "admin")

	tokens, err := s.generator.Generate(principal)
	require.NoError(s.T(), err, "Should generate access/refresh tokens without error")

	_, err = s.generator.ParseChallengeToken(tokens.AccessToken)
	assert.Error(s.T(), err, "Should reject access token as challenge token")
}

// TestParseChallengeTokenRejectsInvalidToken verifies that a malformed token string is rejected.
func (s *JWTTokenGeneratorTestSuite) TestParseChallengeTokenRejectsInvalidToken() {
	_, err := s.generator.ParseChallengeToken("invalid.token.here")
	assert.Error(s.T(), err, "Should reject invalid token string")
}

func TestJWTTokenGeneratorTestSuite(t *testing.T) {
	suite.Run(t, new(JWTTokenGeneratorTestSuite))
}
