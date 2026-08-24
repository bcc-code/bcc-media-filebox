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
		if err := q.CompleteUpload(ctx, sub+"-up"); err != nil {
			t.Fatalf("complete upload for %s: %v", sub, err)
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

func TestCreatePackageRejectsAnIncompleteUpload(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, _ := twoSenders(t, q)

	if err := q.CreateUpload(context.Background(), db.CreateUploadParams{
		ID: "still-uploading", Filename: "unfinished.mov", Size: 2048, UserID: alice.CanonicalUserID(),
	}); err != nil {
		t.Fatalf("seed incomplete upload: %v", err)
	}

	w := createPackageAs(t, h, alice, `{"name":"too soon","uploadIds":["still-uploading"],"expiresInDays":7}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d (%s), want 409", w.Code, w.Body.String())
	}
	if list := listPackagesAs(t, h, alice); list.Total != 0 {
		t.Errorf("a refused create left %d packages behind", list.Total)
	}
}

// A restored draft and the manual server picker can discover the same upload
// at nearly the same time. The frontend merges by ID, and the API must remain a
// second line of defence so one source cannot become two package members.
func TestCreatePackageRejectsDuplicateUploadID(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, _ := twoSenders(t, q)

	w := createPackageAs(t, h, alice, `{"name":"duplicate","uploadIds":["alice-up","alice-up"],"expiresInDays":7}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (%s), want 400", w.Code, w.Body.String())
	}
	if list := listPackagesAs(t, h, alice); list.Total != 0 {
		t.Errorf("a refused duplicate created %d package(s)", list.Total)
	}
}

func TestCreatePackageRejectsACompletedPartialUpload(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, _ := twoSenders(t, q)
	ctx := context.Background()

	if err := q.CreateUpload(ctx, db.CreateUploadParams{
		ID: "partial", Filename: "partial.mov", Size: 2048, UserID: alice.CanonicalUserID(), IsPartial: 1,
	}); err != nil {
		t.Fatalf("seed partial upload: %v", err)
	}
	if err := q.CompleteUpload(ctx, "partial"); err != nil {
		t.Fatalf("complete partial upload: %v", err)
	}

	w := createPackageAs(t, h, alice, `{"name":"partial package","uploadIds":["partial"],"expiresInDays":7}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d (%s), want 409", w.Code, w.Body.String())
	}
	if list := listPackagesAs(t, h, alice); list.Total != 0 {
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

// A typo in a recipient address is rejected outright: it would otherwise be
// dropped, leaving that person with no mail and no way to ask for the package.
func TestCreatePackageRejectsInvalidRecipient(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, _ := twoSenders(t, q)

	body := `{"name":"p","uploadIds":["alice-up"],"expiresInDays":7,"recipients":["jane@example.com","not-an-email"]}`
	w := createPackageAs(t, h, alice, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (%s), want 400", w.Code, w.Body.String())
	}
	if list := listPackagesAs(t, h, alice); list.Total != 0 {
		t.Errorf("a refused create left %d packages behind", list.Total)
	}
}

// Recipients are stored canonically and deduped, so a display-name form and a
// repeat of the same address don't turn into extra rows or extra mail.
func TestCreatePackageCanonicalisesRecipients(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, _ := twoSenders(t, q)

	body := `{"name":"p","uploadIds":["alice-up"],"expiresInDays":7,` +
		`"recipients":["Jane Roe <jane@example.com>","JANE@example.com","  "]}`
	if w := createPackageAs(t, h, alice, body); w.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s), want 201", w.Code, w.Body.String())
	}
	list := listPackagesAs(t, h, alice)
	if got := list.Packages[0].Recipients; len(got) != 1 || got[0] != "jane@example.com" {
		t.Errorf("recipients = %v, want just the bare address once", got)
	}
}

// notify_on_download decides whether downloads are reported at all, so it has to
// survive the create — the column was there long before anything wrote to it.
func TestCreatePackagePersistsNotifyOnDownload(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	alice, _ := twoSenders(t, q)

	for _, tc := range []struct {
		notify string
		want   int64
	}{{"true", 1}, {"false", 0}} {
		body := `{"name":"p","uploadIds":["alice-up"],"expiresInDays":7,"notifyOnDownload":` + tc.notify + `}`
		w := createPackageAs(t, h, alice, body)
		if w.Code != http.StatusCreated {
			t.Fatalf("notify=%s: status = %d (%s)", tc.notify, w.Code, w.Body.String())
		}
		var created CreatePackageResponse
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode create: %v", err)
		}
		pkg, err := q.GetPackageByID(context.Background(), created.PackageID)
		if err != nil {
			t.Fatalf("reload package: %v", err)
		}
		if pkg.NotifyOnDownload != tc.want {
			t.Errorf("notify=%s: notify_on_download = %d, want %d", tc.notify, pkg.NotifyOnDownload, tc.want)
		}
	}
}
