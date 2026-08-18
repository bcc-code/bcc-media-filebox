package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/mail"
)

// twoSenders creates two accounts, each owning one upload, as the starting point
// for the isolation checks below.
func twoSenders(t *testing.T, q *db.Queries) (a, b *auth.Caller) {
	t.Helper()
	ctx := context.Background()
	for i, sub := range []string{"alice", "bob"} {
		user, err := q.UpsertUser(ctx, db.UpsertUserParams{
			Provider: "bcc", Subject: sub,
			Email: sql.NullString{String: sub + "@bcc.no", Valid: true},
			Name:  sql.NullString{String: strings.ToUpper(sub[:1]) + sub[1:], Valid: true},
		})
		if err != nil {
			t.Fatalf("seed %s: %v", sub, err)
		}
		if err := q.CreateUpload(ctx, db.CreateUploadParams{
			ID: sub + "-up", Filename: sub + ".mov", Size: 1024, UserID: "bcc:" + sub,
		}); err != nil {
			t.Fatalf("seed upload for %s: %v", sub, err)
		}
		c := &auth.Caller{UserID: user.ID, Provider: "bcc", Subject: sub, Email: sub + "@bcc.no", Name: sub}
		if i == 0 {
			a = c
		} else {
			b = c
		}
	}
	return a, b
}

func createPackageAs(t *testing.T, h *Handlers, caller *auth.Caller, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/packages", strings.NewReader(body))
	r = r.WithContext(auth.WithCaller(r.Context(), caller))
	w := httptest.NewRecorder()
	h.CreatePackage(w, r)
	return w
}

func listPackagesAs(t *testing.T, h *Handlers, caller *auth.Caller) ListPackagesResponse {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/packages", nil)
	r = r.WithContext(auth.WithCaller(r.Context(), caller))
	w := httptest.NewRecorder()
	h.ListPackagesByUser(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("list: status = %d (%s)", w.Code, w.Body.String())
	}
	var out ListPackagesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return out
}

// Two accounts sending at once must not see or touch each other's packages.
func TestPackagesAreIsolatedBetweenSenders(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, bob := twoSenders(t, q)

	for _, c := range []*auth.Caller{alice, bob} {
		body := `{"name":"` + c.Subject + `'s package","uploadIds":["` + c.Subject + `-up"],"expiresInDays":7}`
		if w := createPackageAs(t, h, c, body); w.Code != http.StatusCreated {
			t.Fatalf("%s create: status = %d (%s)", c.Subject, w.Code, w.Body.String())
		}
	}

	for _, c := range []*auth.Caller{alice, bob} {
		list := listPackagesAs(t, h, c)
		if list.Total != 1 || len(list.Packages) != 1 {
			t.Fatalf("%s sees %d packages (total %d), want exactly their own", c.Subject, len(list.Packages), list.Total)
		}
		if want := c.Subject + "'s package"; list.Packages[0].Name != want {
			t.Errorf("%s sees %q, want %q", c.Subject, list.Packages[0].Name, want)
		}
	}
}

// Packaging someone else's upload must be refused, or one sender could share
// another's files by guessing an upload id.
func TestCreatePackageRejectsAnotherSendersUpload(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	_, bob := twoSenders(t, q)

	w := createPackageAs(t, h, bob, `{"name":"stolen","uploadIds":["alice-up"],"expiresInDays":7}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d (%s), want 403", w.Code, w.Body.String())
	}
	if list := listPackagesAs(t, h, bob); list.Total != 0 {
		t.Errorf("a refused create left %d packages behind", list.Total)
	}
}

// Every mutation on someone else's package must be refused, not just extend.
func TestSenderCannotMutateAnotherSendersPackage(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, bob := twoSenders(t, q)

	if w := createPackageAs(t, h, alice, `{"name":"alice's package","uploadIds":["alice-up"],"expiresInDays":7}`); w.Code != http.StatusCreated {
		t.Fatalf("alice create: %d (%s)", w.Code, w.Body.String())
	}
	pkgID := listPackagesAs(t, h, alice).Packages[0].PackageID

	as := func(method, path, body string, fn func(http.ResponseWriter, *http.Request)) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.SetPathValue("id", pkgID)
		r = r.WithContext(auth.WithCaller(r.Context(), bob))
		w := httptest.NewRecorder()
		fn(w, r)
		return w.Code
	}

	if code := as(http.MethodPost, "/extend", `{"expiresInDays":30}`, h.ExtendPackage); code != http.StatusForbidden {
		t.Errorf("extend: %d, want 403", code)
	}
	if code := as(http.MethodDelete, "/", "", h.RevokePackage); code != http.StatusForbidden {
		t.Errorf("revoke: %d, want 403", code)
	}

	// Alice's package must be untouched by the attempts above.
	reloaded, err := q.GetPackageByID(context.Background(), pkgID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != "active" || !reloaded.ExpiresAt.Before(time.Now().AddDate(0, 0, 8)) {
		t.Errorf("alice's package was altered: status=%q expires=%v", reloaded.Status, reloaded.ExpiresAt)
	}
}

// Concurrent sends must not lose rows or collide on ids. The whole app shares one
// SQLite connection (SetMaxOpenConns(1) in cmd/server), so this also shows what
// that serialization costs when several senders submit at once.
func TestConcurrentPackageCreationFromBothSenders(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, bob := twoSenders(t, q)

	const perSender = 15
	start := time.Now()
	var wg sync.WaitGroup
	for _, c := range []*auth.Caller{alice, bob} {
		for i := 0; i < perSender; i++ {
			wg.Add(1)
			go func(c *auth.Caller, i int) {
				defer wg.Done()
				body := fmt.Sprintf(`{"name":"%s-%d","uploadIds":["%s-up"],"expiresInDays":7}`, c.Subject, i, c.Subject)
				if w := createPackageAs(t, h, c, body); w.Code != http.StatusCreated {
					t.Errorf("%s-%d: status = %d (%s)", c.Subject, i, w.Code, w.Body.String())
				}
			}(c, i)
		}
	}
	wg.Wait()
	elapsed := time.Since(start)

	seen := map[string]bool{}
	for _, c := range []*auth.Caller{alice, bob} {
		list := listPackagesAs(t, h, c)
		if list.Total != perSender {
			t.Errorf("%s has %d packages, want %d — a concurrent create was lost", c.Subject, list.Total, perSender)
		}
		for _, p := range list.Packages {
			if seen[p.PackageID] {
				t.Errorf("duplicate package id %q across senders", p.PackageID)
			}
			seen[p.PackageID] = true
			if !strings.HasPrefix(p.Name, c.Subject+"-") {
				t.Errorf("%s sees %q, which belongs to the other sender", c.Subject, p.Name)
			}
		}
	}
	t.Logf("%d concurrent creates in %v (%.1fms each, serialized on one connection)",
		2*perSender, elapsed.Round(time.Millisecond), float64(elapsed.Milliseconds())/float64(2*perSender))
}
