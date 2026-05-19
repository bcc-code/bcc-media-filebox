package auth

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"

	db "filebox/internal/db/gen"
)

// Handlers serves /auth/* and /api/me. It is safe to instantiate with a nil
// manager and store — in that case all routes return 404 / unauthenticated.
type Handlers struct {
	manager  *Manager
	sessions *SessionStore
	queries  *db.Queries
	baseURL  string
}

func NewHandlers(manager *Manager, sessions *SessionStore, queries *db.Queries, baseURL string) *Handlers {
	return &Handlers{manager: manager, sessions: sessions, queries: queries, baseURL: baseURL}
}

// Register wires the auth routes onto a mux. The /api/me route is always
// registered (even with auth disabled) so the frontend can detect guest mode
// uniformly.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/providers", h.ListProviders)
	mux.HandleFunc("GET /auth/login/{provider}", h.Login)
	mux.HandleFunc("GET /auth/callback/{provider}", h.Callback)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	mux.HandleFunc("POST /auth/guest", h.Guest)
	mux.HandleFunc("GET /api/me", h.Me)
}

// guestRequest is the body of POST /auth/guest. Name and email are both
// required — guests are first-class users in the `users` table, identified
// by their email under the "guest" provider namespace.
type guestRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (h *Handlers) Guest(w http.ResponseWriter, r *http.Request) {
	if h.sessions == nil {
		http.Error(w, "guest sessions unavailable", http.StatusServiceUnavailable)
		return
	}
	var req guestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(req.Email))
	if err != nil {
		http.Error(w, "valid email is required", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(addr.Address)

	if _, err := h.queries.GetNonGuestUserByEmail(r.Context(), email); err == nil {
		http.Error(w, "this email is registered with single sign-on — please sign in instead", http.StatusConflict)
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("guest email lookup: %v", err)
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}

	user, err := h.queries.UpsertUser(r.Context(), db.UpsertUserParams{
		Provider: "guest",
		Subject:  email,
		Email:    sql.NullString{String: email, Valid: true},
		Name:     sql.NullString{String: name, Valid: true},
	})
	if err != nil {
		log.Printf("guest upsert: %v", err)
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}
	if _, err := h.sessions.Create(r.Context(), w, user.ID); err != nil {
		log.Printf("guest session: %v", err)
		http.Error(w, "session create failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type providerInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	out := []providerInfo{}
	if h.manager != nil {
		for _, p := range h.manager.List() {
			out = append(out, providerInfo{ID: p.ID, DisplayName: p.DisplayName})
		}
	}
	_ = json.NewEncoder(w).Encode(out)
}

// oauthStateCookie is JSON-serialised into a short-lived cookie that ties a
// callback to its initiating /auth/login request. Tampering is self-defeating:
// state mismatch, nonce mismatch, or wrong PKCE verifier all fail the flow.
type oauthStateCookie struct {
	Provider string `json:"p"`
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Redirect string `json:"r"`
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		http.Error(w, "oauth disabled", http.StatusNotFound)
		return
	}
	providerID := r.PathValue("provider")
	p, ok := h.manager.Provider(providerID)
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	state, err := randomToken(24)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	nonce, err := randomToken(24)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	verifier, err := randomToken(48)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	challenge := pkceChallenge(verifier)
	redirectURL := p.RedirectURL(h.baseURL, r)

	raw, err := json.Marshal(oauthStateCookie{
		Provider: providerID,
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		Redirect: redirectURL,
	})
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookieName,
		Value:    base64.RawURLEncoding.EncodeToString(raw),
		Path:     "/auth",
		MaxAge:   int(StateTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.sessions.Secure(),
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, p.AuthCodeURL(redirectURL, state, nonce, challenge), http.StatusFound)
}

func (h *Handlers) Callback(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		http.Error(w, "oauth disabled", http.StatusNotFound)
		return
	}
	providerID := r.PathValue("provider")
	p, ok := h.manager.Provider(providerID)
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	c, err := r.Cookie(StateCookieName)
	if err != nil {
		http.Error(w, "missing oauth state cookie", http.StatusBadRequest)
		return
	}
	rawJSON, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		http.Error(w, "invalid state cookie", http.StatusBadRequest)
		return
	}
	var sc oauthStateCookie
	if err := json.Unmarshal(rawJSON, &sc); err != nil {
		http.Error(w, "invalid state cookie", http.StatusBadRequest)
		return
	}

	// Always consume the state cookie, regardless of outcome.
	http.SetCookie(w, &http.Cookie{
		Name: StateCookieName, Value: "", Path: "/auth", MaxAge: -1,
		HttpOnly: true, Secure: h.sessions.Secure(), SameSite: http.SameSiteLaxMode,
	})

	if sc.Provider != providerID {
		http.Error(w, "provider mismatch", http.StatusBadRequest)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Error(w, "oauth error: "+errParam, http.StatusBadRequest)
		return
	}
	if got := r.URL.Query().Get("state"); got != sc.State {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	token, err := p.Exchange(ctx, sc.Redirect, code, sc.Verifier)
	if err != nil {
		log.Printf("oauth %s exchange: %v", providerID, err)
		http.Error(w, "code exchange failed", http.StatusBadGateway)
		return
	}
	idToken, err := p.VerifyIDToken(ctx, token)
	if err != nil {
		log.Printf("oauth %s id-token verify: %v", providerID, err)
		http.Error(w, "id token verify failed", http.StatusBadGateway)
		return
	}
	if idToken.Nonce != sc.Nonce {
		http.Error(w, "nonce mismatch", http.StatusBadRequest)
		return
	}

	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		log.Printf("oauth %s claims: %v", providerID, err)
		http.Error(w, "claim parse failed", http.StatusBadGateway)
		return
	}
	if idToken.Subject == "" {
		http.Error(w, "id token missing subject", http.StatusBadGateway)
		return
	}

	user, err := h.queries.UpsertUser(ctx, db.UpsertUserParams{
		Provider: providerID,
		Subject:  idToken.Subject,
		Email:    sql.NullString{String: claims.Email, Valid: claims.Email != ""},
		Name:     sql.NullString{String: claims.Name, Valid: claims.Name != ""},
	})
	if err != nil {
		log.Printf("upsert user: %v", err)
		http.Error(w, "user storage failed", http.StatusInternalServerError)
		return
	}
	if user.Email.Valid && user.Email.String != "" {
		if _, err := RecomputeRoleForUser(ctx, h.queries, user.ID, user.Email.String, user.Provider); err != nil {
			log.Printf("recompute role for %d: %v", user.ID, err)
		}
	}
	if _, err := h.sessions.Create(ctx, w, user.ID); err != nil {
		log.Printf("create session: %v", err)
		http.Error(w, "session create failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if h.sessions != nil {
		h.sessions.Delete(r.Context(), w, r)
	}
	w.WriteHeader(http.StatusNoContent)
}

type meResponse struct {
	Authenticated bool   `json:"authenticated"`
	UserID        string `json:"userId,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Email         string `json:"email,omitempty"`
	Name          string `json:"name,omitempty"`
	Role          string `json:"role,omitempty"`
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	caller := CallerFrom(r.Context())
	if caller == nil {
		_ = json.NewEncoder(w).Encode(meResponse{Authenticated: false})
		return
	}
	_ = json.NewEncoder(w).Encode(meResponse{
		Authenticated: true,
		UserID:        caller.CanonicalUserID(),
		Provider:      caller.Provider,
		Email:         caller.Email,
		Name:          caller.Name,
		Role:          caller.Role,
	})
}
