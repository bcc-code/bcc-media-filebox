package api

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"filebox/internal/auth"
	dbpkg "filebox/internal/db"
	db "filebox/internal/db/gen"
	"filebox/internal/mail"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// newTestDB mirrors internal/db's helper: a throwaway SQLite with migrations
// applied and the same connection settings cmd/server uses.
func newTestDB(t *testing.T) *db.Queries {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })

	goose.SetBaseFS(dbpkg.Migrations)
	goose.SetDialect("sqlite3")
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(conn, "migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db.New(conn)
}

// fakeSender records what it was asked to send and can fail on demand.
type fakeSender struct {
	mu     sync.Mutex
	sent   []mail.Message
	failOn string
}

func (f *fakeSender) Send(_ context.Context, msg mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn != "" && msg.To[0] == f.failOn {
		return errors.New("550 relay not permitted")
	}
	f.sent = append(f.sent, msg)
	return nil
}

func seedPackage(t *testing.T, q *db.Queries, emails []string) (db.Package, []db.PackageRecipient) {
	t.Helper()
	ctx := context.Background()

	pkg, err := q.CreatePackage(ctx, db.CreatePackageParams{
		ID:                 "pkg123",
		CreatedByUserID:    1,
		Name:               "Summer conference rushes",
		Message:            "Here are the files.",
		VerificationMethod: "none",
		ExpiresAt:          time.Now().Add(7 * 24 * time.Hour),
		MaxDownloads:       sql.NullInt64{Int64: 3, Valid: true},
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	var recipients []db.PackageRecipient
	for _, e := range emails {
		r, err := q.CreatePackageRecipient(ctx, db.CreatePackageRecipientParams{PackageID: pkg.ID, Email: e})
		if err != nil {
			t.Fatalf("create recipient %s: %v", e, err)
		}
		recipients = append(recipients, r)
	}
	return pkg, recipients
}

func TestNotifyRecipientsSendsOneMailEachAndRecordsDelivery(t *testing.T) {
	q := newTestDB(t)
	sender := &fakeSender{}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")

	pkg, recipients := seedPackage(t, q, []string{"a@example.com", "b@example.com"})
	uploads := []db.Upload{{Filename: "rushes.mov", Size: 4823400000}}

	h.notifyRecipients(pkg, recipients, uploads, &auth.Caller{Name: "John Doe", Email: "john.doe@bcc.no"})

	if len(sender.sent) != 2 {
		t.Fatalf("expected one message per recipient, got %d", len(sender.sent))
	}
	// Recipients must not be able to see each other.
	for _, msg := range sender.sent {
		if len(msg.To) != 1 {
			t.Errorf("expected exactly one To address per message, got %v", msg.To)
		}
		if msg.ReplyTo != "john.doe@bcc.no" {
			t.Errorf("Reply-To = %q, want the sharing user", msg.ReplyTo)
		}
		if !strings.Contains(msg.HTML, "https://filebox.example.com/s/pkg123") {
			t.Error("message is missing the share link")
		}
		if !strings.Contains(msg.Text, "rushes.mov") {
			t.Error("message is missing the file list")
		}
	}

	for _, r := range recipients {
		got, err := q.GetPackageRecipientByEmail(context.Background(),
			db.GetPackageRecipientByEmailParams{PackageID: pkg.ID, Email: r.Email})
		if err != nil {
			t.Fatalf("reload recipient: %v", err)
		}
		if !got.SentAt.Valid {
			t.Errorf("%s: sent_at was not recorded", r.Email)
		}
		if got.SendError.Valid {
			t.Errorf("%s: unexpected send_error %q", r.Email, got.SendError.String)
		}
	}
}

// One bad address must not stop the others, and the reason must be persisted.
func TestNotifyRecipientsRecordsFailureAndContinues(t *testing.T) {
	q := newTestDB(t)
	sender := &fakeSender{failOn: "bad@example.com"}
	h := NewHandlers(q, t.TempDir(), nil, sender, "https://filebox.example.com")

	pkg, recipients := seedPackage(t, q, []string{"bad@example.com", "good@example.com"})
	h.notifyRecipients(pkg, recipients, nil, &auth.Caller{Name: "John Doe", Email: "john.doe@bcc.no"})

	if len(sender.sent) != 1 || sender.sent[0].To[0] != "good@example.com" {
		t.Fatalf("expected the good address to still be sent, got %+v", sender.sent)
	}

	bad, err := q.GetPackageRecipientByEmail(context.Background(),
		db.GetPackageRecipientByEmailParams{PackageID: pkg.ID, Email: "bad@example.com"})
	if err != nil {
		t.Fatalf("reload recipient: %v", err)
	}
	if bad.SentAt.Valid {
		t.Error("a failed send must not be marked as sent")
	}
	if !strings.Contains(bad.SendError.String, "550") {
		t.Errorf("send_error = %q, want the relay's reason", bad.SendError.String)
	}
}

// ctxSender reports whatever the send context says, so an exhausted
// notifyTimeout surfaces as a real send failure.
type ctxSender struct{}

func (ctxSender) Send(ctx context.Context, _ mail.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// A run that burns the whole notifyTimeout must still record WHY it failed.
// Sharing the expired context with the status write would leave the recipient
// with neither sent_at nor send_error — indistinguishable from never trying.
func TestNotifyRecipientsRecordsFailureAfterSendTimeout(t *testing.T) {
	orig := notifyTimeout
	notifyTimeout = time.Nanosecond
	t.Cleanup(func() { notifyTimeout = orig })

	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, ctxSender{}, "https://filebox.example.com")

	pkg, recipients := seedPackage(t, q, []string{"a@example.com"})
	h.notifyRecipients(pkg, recipients, nil, &auth.Caller{Name: "John Doe"})

	got, err := q.GetPackageRecipientByEmail(context.Background(),
		db.GetPackageRecipientByEmailParams{PackageID: pkg.ID, Email: "a@example.com"})
	if err != nil {
		t.Fatalf("reload recipient: %v", err)
	}
	if got.SentAt.Valid {
		t.Error("a timed-out send must not be recorded as delivered")
	}
	if !got.SendError.Valid {
		t.Fatal("send_error was not recorded — the failure left no trace in the DB")
	}
	if !strings.Contains(got.SendError.String, "deadline exceeded") {
		t.Errorf("send_error = %q, want the deadline reason", got.SendError.String)
	}
}

// panicSender stands in for any future nil deref inside the send path.
type panicSender struct{}

func (panicSender) Send(context.Context, mail.Message) error { panic("relay exploded") }

// notifyRecipients runs on its own goroutine, where a panic would kill the
// process rather than fail one request — so it must contain its own.
func TestNotifyRecipientsRecoversFromPanic(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, panicSender{}, "https://filebox.example.com")

	pkg, recipients := seedPackage(t, q, []string{"a@example.com"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.notifyRecipients(pkg, recipients, nil, &auth.Caller{Name: "John Doe"})
	}()
	<-done // reaching here at all means the panic never escaped the goroutine
}

// With mail switched off the whole path must be inert — no writes, no panics.
func TestNotifyRecipientsNoopWhenMailDisabled(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "")

	pkg, recipients := seedPackage(t, q, []string{"a@example.com"})
	h.notifyRecipients(pkg, recipients, nil, &auth.Caller{Name: "John Doe"})

	got, err := q.GetPackageRecipientByEmail(context.Background(),
		db.GetPackageRecipientByEmailParams{PackageID: pkg.ID, Email: "a@example.com"})
	if err != nil {
		t.Fatalf("reload recipient: %v", err)
	}
	if got.SentAt.Valid || got.SendError.Valid {
		t.Error("disabled mail must leave delivery state untouched")
	}
}
