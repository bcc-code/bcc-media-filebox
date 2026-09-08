package tus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	db "filebox/internal/db/gen"
)

// reapFixture builds an EventProcessor over a fresh temp dir and database.
type reapFixture struct {
	ep      *EventProcessor
	tempDir string
}

func newReapFixture(t *testing.T) reapFixture {
	t.Helper()
	tempDir := t.TempDir()
	ep := NewEventProcessor(newTestDB(t), tempDir, tempDir, nil)
	ep.TempTTL = time.Hour
	return reapFixture{ep: ep, tempDir: tempDir}
}

// writeTemp creates one of an upload's temp files and backdates it.
func (f reapFixture) writeTemp(t *testing.T, name string, age time.Duration) string {
	t.Helper()
	path := filepath.Join(f.tempDir, name)
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	return path
}

func (f reapFixture) row(t *testing.T, id, status string) {
	t.Helper()
	ctx := context.Background()
	if err := f.ep.queries.CreatePendingUpload(ctx, db.CreatePendingUploadParams{ID: id, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if status == uploadStatusCompleted {
		if err := f.ep.queries.CompleteUpload(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
}

func (f reapFixture) exists(name string) bool {
	_, err := os.Stat(filepath.Join(f.tempDir, name))
	return err == nil
}

func TestReapTempRemovesStaleOrphanWithNoRow(t *testing.T) {
	f := newReapFixture(t)
	f.writeTemp(t, "orphan", 3*time.Hour)
	f.writeTemp(t, "orphan.info", 3*time.Hour)
	f.writeTemp(t, "orphan.lock", 3*time.Hour)

	removed, err := f.ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	for _, name := range []string{"orphan", "orphan.info", "orphan.lock"} {
		if f.exists(name) {
			t.Errorf("%s should have been removed", name)
		}
	}
}

func TestReapTempKeepsFreshFiles(t *testing.T) {
	f := newReapFixture(t)
	f.writeTemp(t, "live", time.Minute)

	removed, err := f.ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
	if !f.exists("live") {
		t.Error("a file inside the TTL must survive")
	}
}

// An upload whose binary is stale but whose sidecar was just rewritten is still
// active — a positioned partial writes into the group file, not into a binary
// of its own, so its own mtime is not the signal.
func TestReapTempTreatsNewestFileAsTheUploadsAge(t *testing.T) {
	f := newReapFixture(t)
	f.writeTemp(t, "split", 3*time.Hour)
	f.writeTemp(t, "split.info", time.Minute)

	removed, err := f.ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
	if !f.exists("split") {
		t.Error("binary must survive while its sidecar is being written")
	}
}

// Completed rows belong to RecoverPending, which promotes them into their
// target on the next restart. Reaping one would be data loss, however old.
func TestReapTempNeverTouchesCompletedUploads(t *testing.T) {
	f := newReapFixture(t)
	f.writeTemp(t, "done", 30*24*time.Hour)
	f.writeTemp(t, "done.info", 30*24*time.Hour)
	f.row(t, "done", uploadStatusCompleted)

	removed, err := f.ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
	if !f.exists("done") {
		t.Fatal("a completed upload awaiting recovery must never be reaped")
	}
}

// A partial abandoned mid-transfer keeps its row forever otherwise:
// ListPendingStorageUploads filters to is_partial = 0 AND status = 'completed',
// so nothing else ever looks at it.
func TestReapTempRemovesAbandonedInProgressUploadAndItsRow(t *testing.T) {
	f := newReapFixture(t)
	f.writeTemp(t, "stalled", 3*time.Hour)
	f.row(t, "stalled", "uploading")

	removed, err := f.ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if f.exists("stalled") {
		t.Error("stalled upload should have been removed")
	}
	if _, err := f.ep.queries.GetUpload(context.Background(), "stalled"); err == nil {
		t.Error("the abandoned row should have been deleted too")
	}
}

// Files whose row has already gone must still be reaped: ids are discovered by
// scanning the directory, so this is the state a crash mid-removal leaves, and
// it has to self-heal on the next sweep.
func TestReapTempRemovesFilesWhoseRowIsAlreadyGone(t *testing.T) {
	f := newReapFixture(t)
	f.writeTemp(t, "halfreaped", 3*time.Hour)
	f.writeTemp(t, "halfreaped.info", 3*time.Hour)
	// No row at all — exactly what removeTempFiles leaves behind if the process
	// dies after deleting the row.

	removed, err := f.ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if f.exists("halfreaped") || f.exists("halfreaped.info") {
		t.Error("files left by an interrupted reap must be cleaned up")
	}
}

// A preallocated group file is full length from its first write, so size must
// never be read as evidence of completeness.
func TestReapTempIgnoresFileSize(t *testing.T) {
	f := newReapFixture(t)
	path := filepath.Join(f.tempDir, "prealloc")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, 1<<20); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}

	removed, err := f.ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1 (a full-size file is not a complete one)", removed)
	}
}

// crossDeviceMove owns its own staging files and may be mid-copy.
func TestReapTempSkipsDotPrefixedStagingFiles(t *testing.T) {
	f := newReapFixture(t)
	f.writeTemp(t, ".filebox-upload-123.part", 30*24*time.Hour)

	removed, err := f.ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
	if !f.exists(".filebox-upload-123.part") {
		t.Error("staging files are not the reaper's to remove")
	}
}

func TestReapTempMissingDirIsNotAnError(t *testing.T) {
	ep := NewEventProcessor(newTestDB(t), "", filepath.Join(t.TempDir(), "absent"), nil)
	removed, err := ep.ReapTemp(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
}

func TestTempUploadID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"abc", "abc"},
		{"abc.info", "abc"},
		{"abc.lock", "abc"},
		{"abc.stop", "abc"},
		{".filebox-upload-1.part", ""},
		{"", ""},
		{"01HZY.info", "01HZY"},
	}
	for _, c := range cases {
		if got := tempUploadID(c.in); got != c.want {
			t.Errorf("tempUploadID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
