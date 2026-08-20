package api

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	db "filebox/internal/db/gen"
	"filebox/internal/mail"
)

// seedListPackage creates a package owned by userID with the given shares
// (filename -> access count), recipients and pending requests.
func seedListPackage(t *testing.T, q *db.Queries, userID int64, id string, maxDownloads *int64,
	shares map[string]int64, recipients, asks []string) db.Package {
	t.Helper()
	ctx := context.Background()

	var limit sql.NullInt64
	if maxDownloads != nil {
		limit = sql.NullInt64{Int64: *maxDownloads, Valid: true}
	}
	pkg, err := q.CreatePackage(ctx, db.CreatePackageParams{
		ID: id, CreatedByUserID: userID, Name: id, VerificationMethod: "none",
		MaxDownloads: limit, ExpiresAt: time.Now().Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create package %s: %v", id, err)
	}

	for name, count := range shares {
		upID := id + "-" + name
		if err := q.CreateUpload(ctx, db.CreateUploadParams{
			ID: upID, Filename: name, Size: 100, UserID: "bcc:lister",
		}); err != nil {
			t.Fatalf("create upload: %v", err)
		}
		share, err := q.CreateShare(ctx, db.CreateShareParams{ID: upID + "-sh", UploadID: upID, PackageID: id})
		if err != nil {
			t.Fatalf("create share: %v", err)
		}
		for i := int64(0); i < count; i++ {
			if _, err := q.IncrementShareAccessCount(ctx, share.ID); err != nil {
				t.Fatalf("bump access count: %v", err)
			}
		}
	}
	for _, email := range recipients {
		if _, err := q.CreatePackageRecipient(ctx, db.CreatePackageRecipientParams{PackageID: id, Email: email}); err != nil {
			t.Fatalf("create recipient: %v", err)
		}
	}
	for i, email := range asks {
		if _, err := q.CreatePackageAccessRequest(ctx, db.CreatePackageAccessRequestParams{
			ID: id + "-ask" + string(rune('a'+i)), PackageID: id, Email: email, Reason: "expired",
		}); err != nil {
			t.Fatalf("create access request: %v", err)
		}
	}
	return pkg
}

// The list is assembled from batch queries keyed by package id, so every
// per-package value has to land on the right package — including the cases where
// a package has no rows of a given kind at all.
func TestBuildPackageListItemsKeepsRowsWithTheirPackage(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	ctx := context.Background()

	user, err := q.UpsertUser(ctx, db.UpsertUserParams{Provider: "bcc", Subject: "lister"})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	limit := int64(2)

	full := seedListPackage(t, q, user.ID, "full", &limit,
		map[string]int64{"a.mov": 2, "b.mov": 1}, []string{"r1@example.com", "r2@example.com"}, []string{"ask@example.com"})
	bare := seedListPackage(t, q, user.ID, "bare", nil, nil, nil, nil)
	hit := seedListPackage(t, q, user.ID, "hit", &limit, map[string]int64{"c.mov": 2}, nil, nil)

	items, err := h.buildPackageListItems(ctx, []db.Package{full, bare, hit})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	// full: two files, download count is the most-downloaded file, and the
	// least-downloaded one has budget left so the limit is not hit.
	if got := items[0]; got.PackageID != "full" || got.FileCount != 2 || got.TotalSize != 200 ||
		got.DownloadCount != 2 || got.IsDownloadLimitHit || len(got.Recipients) != 2 || len(got.PendingRequests) != 1 {
		t.Errorf("full: %+v", got)
	}
	if got := items[0].PendingRequests[0]; got.Email != "ask@example.com" || got.Reason != "expired" {
		t.Errorf("full request: %+v", got)
	}

	// bare has no rows in any batch query: every aggregate must read as zero and
	// the slices must be empty rather than another package's.
	if got := items[1]; got.PackageID != "bare" || got.FileCount != 0 || got.TotalSize != 0 ||
		got.DownloadCount != 0 || got.IsDownloadLimitHit || len(got.Recipients) != 0 || len(got.PendingRequests) != 0 {
		t.Errorf("bare: %+v", got)
	}

	// hit's only file has spent its budget, so every file has.
	if got := items[2]; got.PackageID != "hit" || got.FileCount != 1 || !got.IsDownloadLimitHit {
		t.Errorf("hit: %+v", got)
	}
}

// The cause has to survive: the handlers log this error, so a bare "failed to
// list package files" would leave nothing to diagnose from.
func TestBuildPackageListItemsWrapsTheCause(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := h.buildPackageListItems(ctx, []db.Package{{ID: "whatever"}})
	if err == nil {
		t.Fatal("build succeeded on a canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error does not unwrap to context.Canceled: %v", err)
	}
	if !strings.Contains(err.Error(), "list package files") {
		t.Errorf("error lost its label: %v", err)
	}
}

// An empty page must not send an empty IN() list to SQLite.
func TestBuildPackageListItemsWithNoPackages(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")

	items, err := h.buildPackageListItems(context.Background(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
}

// A shared source is retained while its package references it. Foreign-key
// enforcement prevents cleanup from silently turning a package manifest into
// an orphan with different limit semantics.
func TestSharedUploadCannotBeDeletedWhilePackageReferencesIt(t *testing.T) {
	q := newTestDB(t)
	h := NewHandlers(q, t.TempDir(), nil, mail.NoopSender{}, "https://filebox.example.com")
	ctx := context.Background()

	user, err := q.UpsertUser(ctx, db.UpsertUserParams{Provider: "bcc", Subject: "lister"})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	limit := int64(1)
	pkg := seedListPackage(t, q, user.ID, "orphan", &limit, map[string]int64{"gone.mov": 1}, nil, nil)

	if err := q.DeleteUpload(ctx, "orphan-gone.mov"); err == nil {
		t.Fatal("deleting a shared upload unexpectedly succeeded")
	}

	items, err := h.buildPackageListItems(ctx, []db.Package{pkg})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if items[0].FileCount != 1 {
		t.Errorf("fileCount = %d, want retained source", items[0].FileCount)
	}
	if !items[0].IsDownloadLimitHit {
		t.Error("isDownloadLimitHit = false, want true (the orphaned share still counts)")
	}
}
