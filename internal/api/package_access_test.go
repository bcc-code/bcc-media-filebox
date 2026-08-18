package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/mail"
)

// seedAuthor creates the users row created_by_user_id points at. An empty email
// is its own code path: nobody to tell, as opposed to a failed send.
func seedAuthor(t *testing.T, q *db.Queries, email string) db.User {
	t.Helper()
	user, err := q.UpsertUser(context.Background(), db.UpsertUserParams{
		Provider: "bcc",
		Subject:  "author-1",
		Email:    sql.NullString{String: email, Valid: email != ""},
		Name:     sql.NullString{String: "John Doe", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed author: %v", err)
	}
	return user
}

// seedPackageState creates one package with a single share, so the access-count
// aggregates have a row to work on.
func seedPackageState(t *testing.T, q *db.Queries, authorID int64, expiresAt time.Time, maxDownloads sql.NullInt64) db.Package {
	t.Helper()
	ctx := context.Background()

	pkg, err := q.CreatePackage(ctx, db.CreatePackageParams{
		ID:                 "pkgstate",
		CreatedByUserID:    authorID,
		Name:               "Summer conference rushes",
		VerificationMethod: "none",
		ExpiresAt:          expiresAt,
		MaxDownloads:       maxDownloads,
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	if err := q.CreateUpload(ctx, db.CreateUploadParams{
		ID:       "up1",
		Filename: "rushes.mov",
		Size:     4823400000,
		UserID:   "bcc:author-1",
	}); err != nil {
		t.Fatalf("create upload: %v", err)
	}
	if _, err := q.CreateShare(ctx, db.CreateShareParams{ID: "sh1", PackageID: pkg.ID, UploadID: "up1"}); err != nil {
		t.Fatalf("create share: %v", err)
	}
	return pkg
}

func postAccessRequest(t *testing.T, h *Handlers, packageID, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/packages/"+packageID+"/access-request", strings.NewReader(body))
	r.SetPathValue("id", packageID)
	w := httptest.NewRecorder()
	h.RequestPackageAccess(w, r)
	return w
}

// The handler tests run with mail off: notifications fire on a detached
// goroutine, so a fakeSender would race. The mails are asserted separately, by
// calling notifyAccessRequest / notifyAccessGranted directly.
func TestRequestPackageAccessRecordsExpiredRequest(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-24*time.Hour), sql.NullInt64{})

	w := postAccessRequest(t, h, pkg.ID, `{"email":"recipient@example.com","message":"Still need these."}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d (%s), want 202", w.Code, w.Body.String())
	}

	requests, err := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected one pending request, got %d", len(requests))
	}
	// Captured at request time: the author may read the mail after extending.
	if requests[0].Reason != mail.ReasonExpired {
		t.Errorf("reason = %q, want %q", requests[0].Reason, mail.ReasonExpired)
	}
	if requests[0].Message != "Still need these." {
		t.Errorf("message = %q, want the requester's note", requests[0].Message)
	}
}

// The address is taken at face value, but must be stored canonically — otherwise
// wrapping it in a display name sidesteps the cooldown.
func TestRequestPackageAccessCanonicalisesEmail(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})

	if w := postAccessRequest(t, h, pkg.ID, `{"email":"  Jane Roe <jane@example.com>  "}`); w.Code != http.StatusAccepted {
		t.Fatalf("status = %d (%s), want 202", w.Code, w.Body.String())
	}
	requests, _ := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if len(requests) != 1 || requests[0].Email != "jane@example.com" {
		t.Fatalf("stored email = %+v, want the bare address", requests)
	}
	if w := postAccessRequest(t, h, pkg.ID, `{"email":"jane@example.com"}`); w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 — the cooldown must see both forms as one address", w.Code)
	}
}

func TestNotifyAccessRequestMailsAuthor(t *testing.T) {
	q := newTestDB(t)
	ctx := context.Background()
	sender := &fakeSender{}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-24*time.Hour), sql.NullInt64{})
	req, err := q.CreatePackageAccessRequest(ctx, db.CreatePackageAccessRequestParams{
		ID: "req1", PackageID: pkg.ID, Email: "recipient@example.com",
		Reason: mail.ReasonExpired, Message: "Still need these.",
	})
	if err != nil {
		t.Fatalf("seed request: %v", err)
	}

	h.notifyAccessRequest(pkg, req, 0)

	if len(sender.sent) != 1 {
		t.Fatalf("expected one mail to the author, got %d", len(sender.sent))
	}
	msg := sender.sent[0]
	if msg.To[0] != "john.doe@bcc.no" {
		t.Errorf("To = %v, want the package author", msg.To)
	}
	// Mirror of a share notification: a reply must reach the asker.
	if msg.ReplyTo != "recipient@example.com" {
		t.Errorf("Reply-To = %q, want the requester", msg.ReplyTo)
	}
	if !strings.Contains(msg.HTML, "https://filebox.example.com/send?package=pkgstate&amp;tab=sent") {
		t.Errorf("mail is missing the author's manage link:\n%s", msg.HTML)
	}
	if !strings.Contains(msg.Text, "Still need these.") {
		t.Error("mail is missing the requester's note")
	}
}

func TestRequestPackageAccessRejectsAvailablePackage(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(24*time.Hour), sql.NullInt64{Int64: 3, Valid: true})

	w := postAccessRequest(t, h, pkg.ID, `{"email":"recipient@example.com"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d (%s), want 409 — a working link is not a channel for mailing its author",
			w.Code, w.Body.String())
	}
	requests, _ := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if len(requests) != 0 {
		t.Errorf("expected no request row, got %d", len(requests))
	}
}

// Every file at its budget counts as unavailable, even before expiry.
func TestRequestPackageAccessAcceptsExhaustedDownloadLimit(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(24*time.Hour), sql.NullInt64{Int64: 1, Valid: true})
	if _, err := q.IncrementShareAccessCount(context.Background(), "sh1"); err != nil {
		t.Fatalf("increment access count: %v", err)
	}

	w := postAccessRequest(t, h, pkg.ID, `{"email":"recipient@example.com"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d (%s), want 202", w.Code, w.Body.String())
	}
	requests, _ := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if len(requests) != 1 || requests[0].Reason != mail.ReasonLimitReached {
		t.Fatalf("expected one limit_reached request, got %+v", requests)
	}
}

// The endpoint is public, so the cooldown is the only bound on how much mail one
// leaked link can generate.
func TestRequestPackageAccessEnforcesCooldownPerEmail(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})

	if w := postAccessRequest(t, h, pkg.ID, `{"email":"recipient@example.com"}`); w.Code != http.StatusAccepted {
		t.Fatalf("first request status = %d (%s), want 202", w.Code, w.Body.String())
	}
	if w := postAccessRequest(t, h, pkg.ID, `{"email":"recipient@example.com"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d (%s), want 429", w.Code, w.Body.String())
	}
	// A different person must still get through.
	if w := postAccessRequest(t, h, pkg.ID, `{"email":"other@example.com"}`); w.Code != http.StatusAccepted {
		t.Fatalf("other requester status = %d (%s), want 202", w.Code, w.Body.String())
	}

	requests, _ := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if len(requests) != 2 {
		t.Errorf("expected two request rows, got %d", len(requests))
	}
}

// The endpoint is public, so the body must be capped before it is decoded —
// truncating the message afterwards bounds what gets stored, not what an
// anonymous caller can make the handler read and parse.
func TestRequestPackageAccessRejectsOversizedBody(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})

	body := `{"email":"recipient@example.com","message":"` + strings.Repeat("A", 2*maxAccessRequestBody) + `"}`
	w := postAccessRequest(t, h, pkg.ID, body)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d (%s), want 413", w.Code, w.Body.String())
	}
	requests, _ := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if len(requests) != 0 {
		t.Errorf("expected no request row, got %d", len(requests))
	}
}

// A message between the message limit and the body cap is accepted and stored
// truncated — the cap must not be so tight that it rejects a legal request.
func TestRequestPackageAccessTruncatesLongMessage(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})

	body := `{"email":"recipient@example.com","message":"` + strings.Repeat("A", maxAccessRequestMessage+500) + `"}`
	if w := postAccessRequest(t, h, pkg.ID, body); w.Code != http.StatusAccepted {
		t.Fatalf("status = %d (%s), want 202", w.Code, w.Body.String())
	}
	requests, _ := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if len(requests) != 1 {
		t.Fatalf("expected one request, got %d", len(requests))
	}
	if len(requests[0].Message) != maxAccessRequestMessage {
		t.Errorf("stored message is %d chars, want it truncated to %d", len(requests[0].Message), maxAccessRequestMessage)
	}
}

func TestRequestPackageAccessRejectsBadInput(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})

	for name, body := range map[string]string{
		"no email":       `{"email":""}`,
		"not an email":   `{"email":"not-an-address"}`,
		"malformed json": `{`,
	} {
		t.Run(name, func(t *testing.T) {
			if w := postAccessRequest(t, h, pkg.ID, body); w.Code != http.StatusBadRequest {
				t.Errorf("status = %d (%s), want 400", w.Code, w.Body.String())
			}
		})
	}
}

func TestExtendPackageGrantsPendingRequestsAndUnrevokes(t *testing.T) {
	q := newTestDB(t)
	ctx := context.Background()
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{Int64: 1, Valid: true})
	if err := q.RevokePackage(ctx, pkg.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := q.CreatePackageAccessRequest(ctx, db.CreatePackageAccessRequestParams{
		ID: "req1", PackageID: pkg.ID, Email: "recipient@example.com",
		Reason: mail.ReasonRevoked,
	}); err != nil {
		t.Fatalf("seed request: %v", err)
	}

	caller := &auth.Caller{UserID: author.ID, Name: "John Doe", Email: "john.doe@bcc.no", Provider: "bcc", Subject: "author-1"}
	r := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkg.ID+"/extend", strings.NewReader(`{"expiresInDays":7,"maxDownloads":5}`))
	r.SetPathValue("id", pkg.ID)
	r = r.WithContext(auth.WithCaller(r.Context(), caller))
	w := httptest.NewRecorder()
	h.ExtendPackage(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}

	var item PackageListItem
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.Status != "active" {
		t.Errorf("status = %q — extending a revoked package must reopen it", item.Status)
	}
	if item.IsExpired {
		t.Error("extended package is still marked expired")
	}
	if item.MaxDownloads == nil || *item.MaxDownloads != 5 {
		t.Errorf("maxDownloads = %v, want 5", item.MaxDownloads)
	}
	if len(item.PendingRequests) != 0 {
		t.Errorf("expected the pending request to be granted, got %d still pending", len(item.PendingRequests))
	}

	granted, err := q.GetPackageAccessRequest(ctx, "req1")
	if err != nil {
		t.Fatalf("reload request: %v", err)
	}
	if granted.Status != "granted" || !granted.ResolvedAt.Valid {
		t.Errorf("request status = %q resolved = %v, want granted and resolved", granted.Status, granted.ResolvedAt.Valid)
	}
}

// Granting must not be silent — the person waiting has no other way to find out.
func TestNotifyAccessGrantedMailsRequesters(t *testing.T) {
	q := newTestDB(t)
	ctx := context.Background()
	sender := &fakeSender{}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(7*24*time.Hour), sql.NullInt64{Int64: 5, Valid: true})
	granted := make([]db.PackageAccessRequest, 0, 2)
	for i, email := range []string{"one@example.com", "two@example.com"} {
		req, err := q.CreatePackageAccessRequest(ctx, db.CreatePackageAccessRequestParams{
			ID: string(rune('a' + i)), PackageID: pkg.ID, Email: email,
			Reason: mail.ReasonExpired,
		})
		if err != nil {
			t.Fatalf("seed request: %v", err)
		}
		granted = append(granted, req)
	}

	h.notifyAccessGranted(pkg, granted, &auth.Caller{Name: "John Doe", Email: "john.doe@bcc.no"})

	if len(sender.sent) != 2 {
		t.Fatalf("expected one mail per requester, got %d", len(sender.sent))
	}
	for _, msg := range sender.sent {
		// Requesters must not see each other.
		if len(msg.To) != 1 {
			t.Errorf("expected exactly one To address per message, got %v", msg.To)
		}
		if !strings.Contains(msg.Subject, "works again") {
			t.Errorf("subject = %q, want the renewed wording", msg.Subject)
		}
		if !strings.Contains(msg.HTML, "https://filebox.example.com/s/pkgstate") {
			t.Error("granted mail is missing the share link")
		}
		if !strings.Contains(msg.Text, "rushes.mov") {
			t.Error("granted mail is missing the file list")
		}
	}
}

func TestExtendPackageRejectsNonOwner(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})

	r := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkg.ID+"/extend", strings.NewReader(`{"expiresInDays":7}`))
	r.SetPathValue("id", pkg.ID)
	r = r.WithContext(auth.WithCaller(r.Context(), &auth.Caller{UserID: author.ID + 1}))
	w := httptest.NewRecorder()
	h.ExtendPackage(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d (%s), want 403", w.Code, w.Body.String())
	}
	if reloaded, _ := q.GetPackageByID(context.Background(), pkg.ID); reloaded.ExpiresAt.After(time.Now()) {
		t.Error("a non-owner's extend must not touch the package")
	}
}

// A negative budget used to be read as "unlimited", the opposite of the ask.
func TestExtendPackageRejectsNegativeMaxDownloads(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{Int64: 3, Valid: true})

	body := `{"expiresInDays":7,"maxDownloads":-5}`
	r := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkg.ID+"/extend", strings.NewReader(body))
	r.SetPathValue("id", pkg.ID)
	r = r.WithContext(auth.WithCaller(r.Context(), &auth.Caller{UserID: author.ID}))
	w := httptest.NewRecorder()
	h.ExtendPackage(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (%s), want 400", w.Code, w.Body.String())
	}
	reloaded, _ := q.GetPackageByID(context.Background(), pkg.ID)
	if reloaded.ExpiresAt.After(time.Now()) || reloaded.MaxDownloads.Int64 != 3 {
		t.Errorf("rejected extend changed the package: expires=%v maxDownloads=%v", reloaded.ExpiresAt, reloaded.MaxDownloads)
	}
}

// A request id is only meaningful under its own package.
func TestDismissPackageAccessRequestChecksOwnershipAndPackage(t *testing.T) {
	q := newTestDB(t)
	ctx := context.Background()
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})
	if _, err := q.CreatePackageAccessRequest(ctx, db.CreatePackageAccessRequestParams{
		ID: "req1", PackageID: pkg.ID, Email: "recipient@example.com",
		Reason: mail.ReasonExpired,
	}); err != nil {
		t.Fatalf("seed request: %v", err)
	}

	dismiss := func(packageID, requestID string, callerID int64) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodDelete, "/api/packages/"+packageID+"/access-requests/"+requestID, nil)
		r.SetPathValue("id", packageID)
		r.SetPathValue("requestId", requestID)
		r = r.WithContext(auth.WithCaller(r.Context(), &auth.Caller{UserID: callerID}))
		w := httptest.NewRecorder()
		h.DismissPackageAccessRequest(w, r)
		return w
	}

	if w := dismiss("someone-elses-package", "req1", author.ID); w.Code != http.StatusNotFound {
		t.Errorf("mismatched package: status = %d, want 404", w.Code)
	}
	if w := dismiss(pkg.ID, "req1", author.ID+1); w.Code != http.StatusForbidden {
		t.Errorf("non-owner: status = %d, want 403", w.Code)
	}
	if w := dismiss(pkg.ID, "req1", author.ID); w.Code != http.StatusNoContent {
		t.Fatalf("owner: status = %d (%s), want 204", w.Code, w.Body.String())
	}

	remaining, _ := q.ListPendingPackageAccessRequests(ctx, pkg.ID)
	if len(remaining) != 0 {
		t.Errorf("expected the request to be dismissed, got %d still pending", len(remaining))
	}
}

// An author whose provider gave us no address cannot be mailed, but the request
// must still be recorded — it shows up on their package card either way.
func TestNotifyAccessRequestSkipsAuthorWithoutEmail(t *testing.T) {
	q := newTestDB(t)
	sender := &fakeSender{}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")

	author := seedAuthor(t, q, "")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})
	req, err := q.CreatePackageAccessRequest(context.Background(), db.CreatePackageAccessRequestParams{
		ID: "req1", PackageID: pkg.ID, Email: "recipient@example.com",
		Reason: mail.ReasonExpired,
	})
	if err != nil {
		t.Fatalf("seed request: %v", err)
	}

	h.notifyAccessRequest(pkg, req, 0)

	if len(sender.sent) != 0 {
		t.Errorf("expected no mail with no author address, got %+v", sender.sent)
	}
	// The row is the durable record — the author still sees it on the card.
	remaining, _ := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if len(remaining) != 1 {
		t.Errorf("expected the request to survive an unsendable notification, got %d", len(remaining))
	}
}

// A fresh address always passes the cooldown, so the per-package window is what
// bounds total mail to one author.
func TestRequestPackageAccessEnforcesPerPackageCeiling(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})

	for i := 0; i < maxAccessRequestsPerWindow; i++ {
		body := fmt.Sprintf(`{"email":"user%d@example.com"}`, i)
		if w := postAccessRequest(t, h, pkg.ID, body); w.Code != http.StatusAccepted {
			t.Fatalf("request %d: status = %d (%s), want 202", i, w.Code, w.Body.String())
		}
	}
	// A new address would otherwise sail past the per-email cooldown.
	if w := postAccessRequest(t, h, pkg.ID, `{"email":"one-too-many@example.com"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d (%s), want 429", w.Code, w.Body.String())
	}
	requests, _ := q.ListPendingPackageAccessRequests(context.Background(), pkg.ID)
	if len(requests) != maxAccessRequestsPerWindow {
		t.Errorf("stored %d requests, want the ceiling of %d", len(requests), maxAccessRequestsPerWindow)
	}
}

// The window must free up on its own — a latching cap could be used to lock
// legitimate recipients out until the author intervened.
func TestRequestPackageAccessCeilingIsSelfHealing(t *testing.T) {
	orig := accessRequestWindow
	accessRequestWindow = time.Second
	t.Cleanup(func() { accessRequestWindow = orig })

	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})

	for i := 0; i < maxAccessRequestsPerWindow; i++ {
		postAccessRequest(t, h, pkg.ID, fmt.Sprintf(`{"email":"user%d@example.com"}`, i))
	}
	if w := postAccessRequest(t, h, pkg.ID, `{"email":"blocked@example.com"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the ceiling to bite, got %d", w.Code)
	}

	// unixepoch has one-second resolution, so wait past the whole window.
	time.Sleep(2 * time.Second)

	if w := postAccessRequest(t, h, pkg.ID, `{"email":"blocked@example.com"}`); w.Code != http.StatusAccepted {
		t.Fatalf("after the window: status = %d (%s), want 202", w.Code, w.Body.String())
	}
}

// A package mailed to named addresses only takes requests from those addresses —
// whoever else the link reached is not the sender's correspondent.
func TestRequestPackageAccessRejectsNonRecipient(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	ctx := context.Background()

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(-time.Hour), sql.NullInt64{})
	if _, err := q.CreatePackageRecipient(ctx, db.CreatePackageRecipientParams{
		PackageID: pkg.ID, Email: "jane@example.com",
	}); err != nil {
		t.Fatalf("seed recipient: %v", err)
	}

	w := postAccessRequest(t, h, pkg.ID, `{"email":"stranger@example.com"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d (%s), want 403", w.Code, w.Body.String())
	}
	if requests, _ := q.ListPendingPackageAccessRequests(ctx, pkg.ID); len(requests) != 0 {
		t.Errorf("a refused request was recorded anyway: %+v", requests)
	}

	// The address it was mailed to gets through, whatever case they type it in.
	if w := postAccessRequest(t, h, pkg.ID, `{"email":"Jane@Example.com"}`); w.Code != http.StatusAccepted {
		t.Fatalf("recipient status = %d (%s), want 202", w.Code, w.Body.String())
	}
}
