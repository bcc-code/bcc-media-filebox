package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	db "filebox/internal/db/gen"
	"filebox/internal/mail"
)

// seedPasswordPackage creates a live, password-protected package, the only state
// in which VerifyPackage reaches its body decode.
func seedPasswordPackage(t *testing.T, q *db.Queries, password string) db.Package {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	pkg, err := q.CreatePackage(context.Background(), db.CreatePackageParams{
		ID:                 "pwpkg",
		CreatedByUserID:    1,
		Name:               "Summer conference rushes",
		VerificationMethod: "password",
		PasswordHash:       sql.NullString{String: string(hash), Valid: true},
		ExpiresAt:          time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	return pkg
}

func postVerify(t *testing.T, h *Handlers, packageID, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/packages/"+packageID+"/verify", strings.NewReader(body))
	r.SetPathValue("id", packageID)
	w := httptest.NewRecorder()
	h.VerifyPackage(w, r)
	return w
}

// VerifyPackage is public too, so its body needs the same cap as the access
// request endpoint — a password field is no reason to read megabytes.
func TestVerifyPackageRejectsOversizedBody(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	pkg := seedPasswordPackage(t, q, "correct horse")

	body := `{"password":"` + strings.Repeat("A", 2*maxVerifyPackageBody) + `"}`
	if w := postVerify(t, h, pkg.ID, body); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d (%s), want 413", w.Code, w.Body.String())
	}
}

// The cap must not be so tight that a real attempt is rejected, and a wrong
// password must still read as wrong rather than as oversized.
func TestVerifyPackageAcceptsNormalBody(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	pkg := seedPasswordPackage(t, q, "correct horse")

	if w := postVerify(t, h, pkg.ID, `{"password":"wrong"}`); w.Code != http.StatusForbidden {
		t.Errorf("wrong password: status = %d (%s), want 403", w.Code, w.Body.String())
	}
	if w := postVerify(t, h, pkg.ID, `{"password":"correct horse"}`); w.Code != http.StatusOK {
		t.Fatalf("correct password: status = %d (%s), want 200", w.Code, w.Body.String())
	}
}
