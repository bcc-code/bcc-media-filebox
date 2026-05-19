package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Provider wraps a single discovered OIDC issuer plus its oauth2 settings.
type Provider struct {
	Config       ProviderConfig
	oidcProvider *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config oauth2.Config
}

// Manager keeps discovered providers indexed by id. Nil-safe.
type Manager struct {
	providers map[string]*Provider
	order     []string
	config    *Config
}

func NewManager(ctx context.Context, cfg *Config) (*Manager, error) {
	m := &Manager{
		providers: make(map[string]*Provider),
		config:    cfg,
	}
	for _, pc := range cfg.Providers {
		p, err := newProvider(ctx, pc)
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", pc.ID, err)
		}
		m.providers[pc.ID] = p
		m.order = append(m.order, pc.ID)
	}
	return m, nil
}

func newProvider(ctx context.Context, pc ProviderConfig) (*Provider, error) {
	op, err := oidc.NewProvider(ctx, pc.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}
	return &Provider{
		Config:       pc,
		oidcProvider: op,
		verifier:     op.Verifier(&oidc.Config{ClientID: pc.ClientID}),
		oauth2Config: oauth2.Config{
			ClientID:     pc.ClientID,
			ClientSecret: pc.ClientSecret,
			Endpoint:     op.Endpoint(),
			Scopes:       pc.Scopes,
		},
	}, nil
}

// Provider returns the named provider or false if it isn't configured.
func (m *Manager) Provider(id string) (*Provider, bool) {
	if m == nil {
		return nil, false
	}
	p, ok := m.providers[id]
	return p, ok
}

// List returns provider configs in stable declaration order.
func (m *Manager) List() []ProviderConfig {
	if m == nil {
		return nil
	}
	out := make([]ProviderConfig, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.providers[id].Config)
	}
	return out
}

// RedirectURL composes the absolute callback URL. Resolution order:
//  1. Config.BaseURL — the deterministic production setting.
//  2. X-Forwarded-Host / X-Forwarded-Proto — set by Vite's proxy in dev so
//     the URL points at :5173 even though the request lands on :8080.
//  3. r.Host — last-ditch fallback for direct connections.
func (p *Provider) RedirectURL(baseURL string, r *http.Request) string {
	path := "/auth/callback/" + p.Config.ID
	if baseURL != "" {
		return strings.TrimRight(baseURL, "/") + path
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	return scheme + "://" + host + path
}

// AuthCodeURL builds the authorization-endpoint URL with PKCE S256 + nonce.
func (p *Provider) AuthCodeURL(redirectURL, state, nonce, codeChallenge string) string {
	cfg := p.oauth2Config
	cfg.RedirectURL = redirectURL
	return cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oidc.Nonce(nonce),
	)
}

// Exchange swaps the authorization code for tokens, supplying the PKCE verifier.
func (p *Provider) Exchange(ctx context.Context, redirectURL, code, codeVerifier string) (*oauth2.Token, error) {
	cfg := p.oauth2Config
	cfg.RedirectURL = redirectURL
	return cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
}

// VerifyIDToken extracts the id_token from the token response and validates it
// against the provider's signing keys, audience, expiry, and issuer.
func (p *Provider) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}
	return p.verifier.Verify(ctx, raw)
}

// randomToken returns base64url-encoded random bytes for state/nonce/verifier/session ids.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceChallenge derives the S256 code_challenge from a verifier.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

