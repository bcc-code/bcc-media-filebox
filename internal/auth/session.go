package auth

import (
	"context"
	"net/http"
	"time"

	db "filebox/internal/db/gen"
)

const (
	SessionCookieName = "filebox_session"
	StateCookieName   = "filebox_oauth_state"
	SessionTTL        = 30 * 24 * time.Hour
	StateTTL          = 10 * time.Minute
)

// SessionStore creates, looks up, and deletes server-side sessions backed by
// the sessions table. The cookie carries only an opaque random token; user
// identity is resolved on every request via GetSessionWithUser.
type SessionStore struct {
	queries *db.Queries
	secure  bool
}

func NewSessionStore(queries *db.Queries, secure bool) *SessionStore {
	return &SessionStore{queries: queries, secure: secure}
}

// Create issues a fresh session for the user and writes the cookie.
func (s *SessionStore) Create(ctx context.Context, w http.ResponseWriter, userID int64) (string, error) {
	sid, err := randomToken(32)
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(SessionTTL)
	if err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		ID:        sid,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}); err != nil {
		return "", err
	}
	s.setSessionCookie(w, sid, expiresAt)
	return sid, nil
}

// LookupByID returns the caller for an opaque session id, or nil if the
// session is missing or expired. Callers should treat nil as "guest".
func (s *SessionStore) LookupByID(ctx context.Context, sid string) (*Caller, error) {
	if sid == "" {
		return nil, nil
	}
	row, err := s.queries.GetSessionWithUser(ctx, sid)
	if err != nil {
		return nil, nil
	}
	return &Caller{
		UserID:   row.UserID,
		Provider: row.Provider,
		Subject:  row.Subject,
		Email:    row.Email.String,
		Name:     row.Name.String,
		Role:     row.Role,
	}, nil
}

// LookupRequest extracts the session cookie from r and resolves it.
func (s *SessionStore) LookupRequest(ctx context.Context, r *http.Request) (*Caller, error) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, nil
	}
	return s.LookupByID(ctx, c.Value)
}

// Delete removes the session row and clears the cookie.
func (s *SessionStore) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		_ = s.queries.DeleteSession(ctx, c.Value)
	}
	s.clearSessionCookie(w)
}

func (s *SessionStore) Secure() bool { return s.secure }

func (s *SessionStore) setSessionCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *SessionStore) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Middleware attaches the resolved Caller to the request context when the
// caller is authenticated. Guests pass through untouched (CallerFrom returns nil).
func (s *SessionStore) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller, _ := s.LookupRequest(r.Context(), r)
		if caller != nil {
			r = r.WithContext(WithCaller(r.Context(), caller))
		}
		next.ServeHTTP(w, r)
	})
}
