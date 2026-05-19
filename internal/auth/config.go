package auth

import (
	"fmt"
	"os"
	"strings"
)

// ProviderConfig holds the environment-loaded settings for one OIDC provider.
type ProviderConfig struct {
	ID           string
	DisplayName  string
	Issuer       string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Config is the full auth configuration. Providers may be empty, in which case
// Enabled() returns false and the server skips all auth wiring (guest-only mode).
type Config struct {
	Providers    []ProviderConfig
	SessionKey   []byte
	BaseURL      string
	CookieSecure bool
}

// Enabled reports whether at least one OIDC provider is configured.
func (c *Config) Enabled() bool {
	return c != nil && len(c.Providers) > 0
}

// knownProviders is the registry of recognised OIDC integrations. Adding a new
// provider here unlocks the OIDC_<ID>_* env-var convention for it.
var knownProviders = []struct {
	id          string
	defaultName string
}{
	{"bcc", "BCC Login"},
	{"azure", "Microsoft"},
}

// LoadConfig reads OIDC_<ID>_{ISSUER,CLIENT_ID,CLIENT_SECRET,DISPLAY_NAME,SCOPES}
// env vars plus SESSION_KEY. Returns a Config with no providers (and no error)
// when nothing is configured — that's the guest-only mode.
func LoadConfig(baseURL string) (*Config, error) {
	var providers []ProviderConfig
	for _, p := range knownProviders {
		prefix := "OIDC_" + strings.ToUpper(p.id) + "_"
		issuer := os.Getenv(prefix + "ISSUER")
		clientID := os.Getenv(prefix + "CLIENT_ID")
		clientSecret := os.Getenv(prefix + "CLIENT_SECRET")
		if issuer == "" && clientID == "" && clientSecret == "" {
			continue
		}
		if issuer == "" || clientID == "" || clientSecret == "" {
			return nil, fmt.Errorf("%s ISSUER, CLIENT_ID and CLIENT_SECRET must all be set together", prefix)
		}
		displayName := os.Getenv(prefix + "DISPLAY_NAME")
		if displayName == "" {
			displayName = p.defaultName
		}
		scopes := strings.Fields(os.Getenv(prefix + "SCOPES"))
		if len(scopes) == 0 {
			scopes = []string{"openid", "profile", "email"}
		}
		providers = append(providers, ProviderConfig{
			ID:           p.id,
			DisplayName:  displayName,
			Issuer:       issuer,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       scopes,
		})
	}

	cfg := &Config{
		BaseURL:      baseURL,
		CookieSecure: strings.HasPrefix(strings.ToLower(baseURL), "https://"),
	}

	if len(providers) == 0 {
		return cfg, nil
	}

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		return nil, fmt.Errorf("SESSION_KEY is required when OAuth providers are configured (32+ random bytes)")
	}
	if len(sessionKey) < 32 {
		return nil, fmt.Errorf("SESSION_KEY must be at least 32 bytes (got %d)", len(sessionKey))
	}

	cfg.Providers = providers
	cfg.SessionKey = []byte(sessionKey)
	return cfg, nil
}
