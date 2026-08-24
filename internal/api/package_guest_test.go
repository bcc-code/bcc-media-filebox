package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"filebox/internal/auth"
	"filebox/internal/mail"
)

// Send is reserved for fully authenticated identities: a guest session must
// not be able to create packages, even though it carries a non-nil Caller.
func TestCreatePackageRejectsGuests(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	body := `{"name":"pkg","uploadIds":["u1"],"recipients":["r@example.com"],"expiresInDays":7}`

	cases := []struct {
		name   string
		caller *auth.Caller
	}{
		{"no session", nil},
		{"guest session", &auth.Caller{UserID: 1, Provider: "guest", Subject: "abc", Email: "g@example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/packages", strings.NewReader(body))
			if tc.caller != nil {
				r = r.WithContext(auth.WithCaller(r.Context(), tc.caller))
			}
			w := httptest.NewRecorder()
			h.CreatePackage(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d (%s), want 403", w.Code, w.Body.String())
			}
		})
	}
}
