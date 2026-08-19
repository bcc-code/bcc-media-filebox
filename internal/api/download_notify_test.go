package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/mail"
)

// testNotifier is a notifier with millisecond windows, so the coalescing
// behaviour can be asserted without minute-long sleeps.
func testNotifier(t *testing.T, quiet, max time.Duration) (*downloadNotifier, <-chan *downloadBatch) {
	t.Helper()
	// Buffered: a bug that flushes twice must not deadlock the notifier goroutine,
	// it must show up as a second batch on the channel.
	sent := make(chan *downloadBatch, 4)
	n := newDownloadNotifier(func(b *downloadBatch) { sent <- b })
	n.quiet, n.max = quiet, max
	return n, sent
}

func waitForBatch(t *testing.T, sent <-chan *downloadBatch) *downloadBatch {
	t.Helper()
	select {
	case b := <-sent:
		return b
	case <-time.After(3 * time.Second):
		t.Fatal("no batch flushed")
		return nil
	}
}

func TestDownloadNotifierCoalescesIntoOneBatch(t *testing.T) {
	n, sent := testNotifier(t, 80*time.Millisecond, 2*time.Second)
	pkg := db.Package{ID: "pkg1"}
	at := time.Now()

	n.record(pkg, downloadHit{filename: "a.mov", size: 100, who: "Anna Berg", at: at})
	n.record(pkg, downloadHit{filename: "b.mov", size: 200, at: at})
	n.record(pkg, downloadHit{filename: "a.mov", size: 100, who: "Bo Lie", at: at})

	b := waitForBatch(t, sent)
	if got := len(b.order); got != 2 {
		t.Fatalf("files in batch = %d, want 2 (%v)", got, b.order)
	}
	if got := b.files["a.mov"].count; got != 2 {
		t.Errorf("a.mov count = %d, want 2", got)
	}
	if got := b.files["b.mov"].count; got != 1 {
		t.Errorf("b.mov count = %d, want 1", got)
	}
	// First-seen order, and the anonymous hit contributes no name.
	if got := strings.Join(b.who, ","); got != "Anna Berg,Bo Lie" {
		t.Errorf("downloaders = %q, want \"Anna Berg,Bo Lie\"", got)
	}

	// A second flush would mean the same downloads reported twice.
	select {
	case extra := <-sent:
		t.Fatalf("second batch flushed for the same window: %v", extra.order)
	case <-time.After(200 * time.Millisecond):
	}
}

// A hit inside the quiet window pushes the send out, so a recipient clicking
// through files at a human pace still gets one mail.
func TestDownloadNotifierQuietWindowRestarts(t *testing.T) {
	n, sent := testNotifier(t, 150*time.Millisecond, 5*time.Second)
	pkg := db.Package{ID: "pkg1"}

	n.record(pkg, downloadHit{filename: "a.mov", size: 100, at: time.Now()})
	time.Sleep(90 * time.Millisecond)
	n.record(pkg, downloadHit{filename: "b.mov", size: 200, at: time.Now()})

	b := waitForBatch(t, sent)
	if got := len(b.order); got != 2 {
		t.Fatalf("files in batch = %d, want 2 (%v) — the second hit started its own batch", got, b.order)
	}
}

// maxWait is what stops a package under steady download from never reporting:
// with a quiet window far longer than the cap, the cap has to win.
func TestDownloadNotifierCapsWaitAtMaxWait(t *testing.T) {
	n, sent := testNotifier(t, 10*time.Second, 100*time.Millisecond)
	pkg := db.Package{ID: "pkg1"}

	start := time.Now()
	n.record(pkg, downloadHit{filename: "a.mov", size: 100, at: start})

	waitForBatch(t, sent)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("flushed after %v, want the maxWait cap to fire", elapsed)
	}
}

func TestDownloadNotifierSeparatesPackages(t *testing.T) {
	n, sent := testNotifier(t, 60*time.Millisecond, 2*time.Second)
	at := time.Now()

	n.record(db.Package{ID: "pkg1"}, downloadHit{filename: "a.mov", size: 100, at: at})
	n.record(db.Package{ID: "pkg2"}, downloadHit{filename: "b.mov", size: 200, at: at})

	ids := map[string]bool{}
	for range 2 {
		ids[waitForBatch(t, sent).pkg.ID] = true
	}
	if !ids["pkg1"] || !ids["pkg2"] {
		t.Errorf("batches flushed for %v, want one each for pkg1 and pkg2", ids)
	}
}

func TestDownloadNotifierTruncatesOversizedBatch(t *testing.T) {
	n, sent := testNotifier(t, 60*time.Millisecond, 2*time.Second)
	pkg := db.Package{ID: "pkg1"}
	at := time.Now()

	for i := range maxBatchFiles + 7 {
		n.record(pkg, downloadHit{filename: string(rune('a'+i%26)) + string(rune('0'+i/26)) + ".mov", size: 10, at: at})
	}

	b := waitForBatch(t, sent)
	if len(b.order) != maxBatchFiles {
		t.Errorf("files in batch = %d, want the %d cap", len(b.order), maxBatchFiles)
	}
	if b.dropped != 7 {
		t.Errorf("dropped = %d, want 7", b.dropped)
	}
}

// recordDownload is the request-side gate: no toggle, no batch.
func TestRecordDownloadRespectsToggleAndMailer(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, &fakeSender{}, "https://filebox.example.com")
	r := httptest.NewRequest("GET", "/api/shares/sh1", nil)
	upload := db.Upload{Filename: "a.mov", Size: 100}

	h.recordDownload(r, db.Package{ID: "off", NotifyOnDownload: 0}, upload)
	h.recordDownload(r, db.Package{ID: "on", NotifyOnDownload: 1}, upload)

	h.downloads.mu.Lock()
	_, off := h.downloads.open["off"]
	_, on := h.downloads.open["on"]
	h.downloads.mu.Unlock()

	if off {
		t.Error("opened a batch for a package with notify_on_download = 0")
	}
	if !on {
		t.Error("no batch for a package with notify_on_download = 1")
	}

	// Mail disabled: nothing to coalesce for.
	off2 := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	off2.recordDownload(r, db.Package{ID: "on", NotifyOnDownload: 1}, upload)
	off2.downloads.mu.Lock()
	n := len(off2.downloads.open)
	off2.downloads.mu.Unlock()
	if n != 0 {
		t.Errorf("opened %d batches with mail disabled, want 0", n)
	}
}

func TestNotifyDownloadsMailsTheAuthor(t *testing.T) {
	q := newTestDB(t)
	sender := &fakeSender{}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(48*time.Hour), sql.NullInt64{Int64: 3, Valid: true})

	first := time.Now().Add(-4 * time.Minute)
	h.notifyDownloads(&downloadBatch{
		pkg:   pkg,
		files: map[string]*fileTally{"rushes.mov": {size: 4823400000, count: 2}, "notes.pdf": {size: 82043, count: 1}},
		order: []string{"rushes.mov", "notes.pdf"},
		who:   []string{"Anna Berg"},
		first: first,
		last:  time.Now(),
	})

	if len(sender.sent) != 1 {
		t.Fatalf("sent %d mails, want 1", len(sender.sent))
	}
	msg := sender.sent[0]
	if msg.To[0] != "john.doe@bcc.no" {
		t.Errorf("recipient = %q, want the author", msg.To[0])
	}
	if !strings.Contains(msg.Subject, pkg.Name) {
		t.Errorf("subject %q does not name the package", msg.Subject)
	}
	// The mail counts files without naming a downloader: a link can be forwarded,
	// so the session that fetched the file doesn't identify who took it.
	for _, want := range []string{"rushes.mov", "notes.pdf", "2 files were downloaded", "×2", "/send?"} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("text body missing %q:\n%s", want, msg.Text)
		}
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("html body missing %q", want)
		}
	}
}

func TestNotifyDownloadsSkipsAuthorWithoutEmail(t *testing.T) {
	q := newTestDB(t)
	sender := &fakeSender{}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")

	author := seedAuthor(t, q, "")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(48*time.Hour), sql.NullInt64{})

	h.notifyDownloads(&downloadBatch{
		pkg:   pkg,
		files: map[string]*fileTally{"rushes.mov": {size: 100, count: 1}},
		order: []string{"rushes.mov"},
		first: time.Now(),
		last:  time.Now(),
	})

	if len(sender.sent) != 0 {
		t.Errorf("sent %d mails for an author with no address, want 0", len(sender.sent))
	}
}

// An empty batch can reach notifyDownloads only through a bug, but it must not
// send a mail listing nothing.
func TestNotifyDownloadsIgnoresEmptyBatch(t *testing.T) {
	q := newTestDB(t)
	sender := &fakeSender{}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")

	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(48*time.Hour), sql.NullInt64{})

	h.notifyDownloads(&downloadBatch{pkg: pkg, files: map[string]*fileTally{}})

	if len(sender.sent) != 0 {
		t.Errorf("sent %d mails for an empty batch, want 0", len(sender.sent))
	}
}

func patchNotify(t *testing.T, h *Handlers, caller *auth.Caller, packageID, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("PATCH", "/api/packages/"+packageID+"/notify", strings.NewReader(body))
	r.SetPathValue("id", packageID)
	if caller != nil {
		r = r.WithContext(auth.WithCaller(r.Context(), caller))
	}
	w := httptest.NewRecorder()
	h.SetPackageNotify(w, r)
	return w
}

func TestSetPackageNotifyTogglesAndGuardsOwnership(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	author := seedAuthor(t, q, "john.doe@bcc.no")
	pkg := seedPackageState(t, q, author.ID, time.Now().Add(48*time.Hour), sql.NullInt64{})
	owner := &auth.Caller{UserID: author.ID, Provider: "bcc", Subject: "author-1"}

	if w := patchNotify(t, h, owner, pkg.ID, `{"notifyOnDownload":true}`); w.Code != 200 {
		t.Fatalf("enable: status = %d (%s)", w.Code, w.Body.String())
	}
	if reloaded, _ := q.GetPackageByID(context.Background(), pkg.ID); reloaded.NotifyOnDownload != 1 {
		t.Errorf("notify_on_download = %d after enable, want 1", reloaded.NotifyOnDownload)
	}
	if w := patchNotify(t, h, owner, pkg.ID, `{"notifyOnDownload":false}`); w.Code != 200 {
		t.Fatalf("disable: status = %d (%s)", w.Code, w.Body.String())
	}
	if reloaded, _ := q.GetPackageByID(context.Background(), pkg.ID); reloaded.NotifyOnDownload != 0 {
		t.Errorf("notify_on_download = %d after disable, want 0", reloaded.NotifyOnDownload)
	}

	stranger := &auth.Caller{UserID: author.ID + 99, Provider: "bcc", Subject: "someone-else"}
	if w := patchNotify(t, h, stranger, pkg.ID, `{"notifyOnDownload":true}`); w.Code != http.StatusForbidden {
		t.Errorf("stranger: status = %d, want 403", w.Code)
	}
	if w := patchNotify(t, h, nil, pkg.ID, `{"notifyOnDownload":true}`); w.Code != http.StatusForbidden {
		t.Errorf("anonymous: status = %d, want 403", w.Code)
	}
}

func postMute(t *testing.T, h *Handlers, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/api/notifications/mute/"+token, nil)
	r.SetPathValue("token", token)
	w := httptest.NewRecorder()
	h.MutePackageNotifications(w, r)
	return w
}

// The link in the mail is the only credential, so it must work without a session
// — and must be idempotent, since a mail can be clicked twice.
func TestMutePackageNotificationsByToken(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	author := seedAuthor(t, q, "john.doe@bcc.no")

	pkg, err := q.CreatePackage(context.Background(), db.CreatePackageParams{
		ID:                 "pkgmute",
		CreatedByUserID:    author.ID,
		Name:               "Summer conference rushes",
		VerificationMethod: "none",
		ExpiresAt:          time.Now().Add(48 * time.Hour),
		NotifyOnDownload:   1,
		NotifyMuteToken:    sql.NullString{String: "mute-token-1", Valid: true},
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	for _, attempt := range []string{"first", "second"} {
		w := postMute(t, h, "mute-token-1")
		if w.Code != 200 {
			t.Fatalf("%s click: status = %d (%s)", attempt, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), pkg.Name) {
			t.Errorf("%s click: body %q does not name the package", attempt, w.Body.String())
		}
		reloaded, _ := q.GetPackageByID(context.Background(), pkg.ID)
		if reloaded.NotifyOnDownload != 0 {
			t.Errorf("%s click: notify_on_download = %d, want 0", attempt, reloaded.NotifyOnDownload)
		}
	}

	if w := postMute(t, h, "not-a-token"); w.Code != http.StatusNotFound {
		t.Errorf("unknown token: status = %d, want 404", w.Code)
	}
}

// A package with no token must not match one whose token is NULL.
func TestMutePackageNotificationsIgnoresEmptyToken(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	author := seedAuthor(t, q, "john.doe@bcc.no")
	if _, err := q.CreatePackage(context.Background(), db.CreatePackageParams{
		ID: "pkgnull", CreatedByUserID: author.ID, Name: "No token",
		VerificationMethod: "none", ExpiresAt: time.Now().Add(48 * time.Hour), NotifyOnDownload: 1,
	}); err != nil {
		t.Fatalf("create package: %v", err)
	}

	if w := postMute(t, h, ""); w.Code != http.StatusNotFound {
		t.Errorf("empty token: status = %d, want 404", w.Code)
	}
	reloaded, _ := q.GetPackageByID(context.Background(), "pkgnull")
	if reloaded.NotifyOnDownload != 1 {
		t.Error("a package with a NULL token was muted by an empty token")
	}
}

func TestNotifyDownloadsIncludesMuteLink(t *testing.T) {
	q := newTestDB(t)
	sender := &fakeSender{}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")
	author := seedAuthor(t, q, "john.doe@bcc.no")

	pkg, err := q.CreatePackage(context.Background(), db.CreatePackageParams{
		ID: "pkgmute2", CreatedByUserID: author.ID, Name: "Rushes",
		VerificationMethod: "none", ExpiresAt: time.Now().Add(48 * time.Hour),
		NotifyOnDownload: 1, NotifyMuteToken: sql.NullString{String: "tok-abc", Valid: true},
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	h.notifyDownloads(&downloadBatch{
		pkg:   pkg,
		files: map[string]*fileTally{"a.mov": {size: 100, count: 1}},
		order: []string{"a.mov"},
		first: time.Now(), last: time.Now(),
	})

	if len(sender.sent) != 1 {
		t.Fatalf("sent %d mails, want 1", len(sender.sent))
	}
	want := "https://filebox.example.com/mute/tok-abc"
	if !strings.Contains(sender.sent[0].Text, want) {
		t.Errorf("text body missing %q:\n%s", want, sender.sent[0].Text)
	}
	if !strings.Contains(sender.sent[0].HTML, want) {
		t.Errorf("html body missing %q", want)
	}
}

// Switching off must also discard the window already collecting, or the author
// gets one more report minutes after asking for silence.
func TestSwitchingOffDropsTheOpenWindow(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, &fakeSender{}, "https://filebox.example.com")
	author := seedAuthor(t, q, "john.doe@bcc.no")

	pkg, err := q.CreatePackage(context.Background(), db.CreatePackageParams{
		ID: "pkgdrop", CreatedByUserID: author.ID, Name: "Rushes",
		VerificationMethod: "none", ExpiresAt: time.Now().Add(48 * time.Hour),
		NotifyOnDownload: 1, NotifyMuteToken: sql.NullString{String: "tok-drop", Valid: true},
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	owner := &auth.Caller{UserID: author.ID, Provider: "bcc", Subject: "author-1"}
	r := httptest.NewRequest("GET", "/api/shares/sh1", nil)

	for _, tc := range []struct {
		name string
		off  func()
	}{
		{"card toggle", func() { patchNotify(t, h, owner, pkg.ID, `{"notifyOnDownload":false}`) }},
		{"mail link", func() { postMute(t, h, "tok-drop") }},
	} {
		h.recordDownload(r, pkg, db.Upload{Filename: "a.mov", Size: 100})
		h.downloads.mu.Lock()
		open := len(h.downloads.open)
		h.downloads.mu.Unlock()
		if open != 1 {
			t.Fatalf("%s: %d open windows before switching off, want 1", tc.name, open)
		}

		tc.off()

		h.downloads.mu.Lock()
		open = len(h.downloads.open)
		h.downloads.mu.Unlock()
		if open != 0 {
			t.Errorf("%s: %d open windows after switching off, want 0", tc.name, open)
		}
	}
}
