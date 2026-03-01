package security

import "github.com/ilxqx/vef-framework-go/internal/log"

var logger = log.Named("security")

type AuthTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type Authentication struct {
	Type        string `json:"type"`
	Principal   string `json:"principal"`
	Credentials any    `json:"credentials"`
}

type ExternalAppConfig struct {
	Enabled     bool   `json:"enabled"`
	IPWhitelist string `json:"ipWhitelist"`
}

// LoginChallenge describes a challenge the user must complete during login.
type LoginChallenge struct {
	Type     string `json:"type"`
	Data     any    `json:"data,omitempty"`
	Required bool   `json:"required"`
}

// LoginResult represents the response of a login attempt.
// When challenges are pending, Tokens is nil and ChallengeToken + Challenges are set.
// When all challenges are resolved (or none were needed), Tokens is set.
type LoginResult struct {
	Tokens         *AuthTokens      `json:"tokens,omitempty"`
	ChallengeToken string           `json:"challengeToken,omitempty"`
	Challenges     []LoginChallenge `json:"challenges,omitempty"`
}

// ChallengeState holds the state tracked by a challenge token.
type ChallengeState struct {
	Principal *Principal
	Pending   []string
	Resolved  []string
}
