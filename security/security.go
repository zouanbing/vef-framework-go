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
