package tus

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	dbpkg "filebox/internal/db"
	db "filebox/internal/db/gen"

	"github.com/pressly/goose/v3"
	"github.com/tus/tusd/v2/pkg/filestore"
	"github.com/tus/tusd/v2/pkg/handler"
	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *db.Queries {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := sql.Open("sqlite", dbpkg.SQLiteDSN(path))
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

type concatFixture struct {
	ep       *EventProcessor
	tempDir  string
	info     handler.FileInfo
	partials [][]byte
}

// newConcatFixture lays out a final upload plus partials the way tusd's
// filestore does after a DeferredConcater final POST: empty final bin, one bin
// and .info sidecar per partial, and an uploads row for every partial.
func newConcatFixture(t *testing.T, partials [][]byte) concatFixture {
	t.Helper()
	tempDir := t.TempDir()
	queries := newTestDB(t)
	ep := NewEventProcessor(queries, tempDir, tempDir, nil)

	info := handler.FileInfo{ID: "final", IsFinal: true}
	for i, data := range partials {
		id := "part" + strconv.Itoa(i)
		info.PartialUploads = append(info.PartialUploads, id)
		info.Size += int64(len(data))
		if err := os.WriteFile(filepath.Join(tempDir, id), data, 0644); err != nil {
			t.Fatal(err)
		}
		sidecar, _ := json.Marshal(handler.FileInfo{ID: id, Size: int64(len(data)), Offset: int64(len(data)), IsPartial: true})
		if err := os.WriteFile(filepath.Join(tempDir, id+".info"), sidecar, 0644); err != nil {
			t.Fatal(err)
		}
		if err := queries.CreatePendingUpload(context.Background(), db.CreatePendingUploadParams{ID: id, Size: int64(len(data)), IsPartial: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tempDir, "final"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := queries.CreatePendingUpload(context.Background(), db.CreatePendingUploadParams{ID: "final", Size: info.Size}); err != nil {
		t.Fatal(err)
	}
	return concatFixture{ep: ep, tempDir: tempDir, info: info, partials: partials}
}

func (f concatFixture) finalBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.tempDir, "final"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (f concatFixture) expected() []byte { return bytes.Join(f.partials, nil) }

func (f concatFixture) status(t *testing.T) string {
	t.Helper()
	row, err := f.ep.queries.GetUpload(context.Background(), "final")
	if err != nil {
		t.Fatal(err)
	}
	return row.Status
}

func TestAssembleConcatAppendsInOrderAndRemovesPartials(t *testing.T) {
	f := newConcatFixture(t, [][]byte{bytes.Repeat([]byte("a"), 10), bytes.Repeat([]byte("b"), 5), bytes.Repeat([]byte("c"), 7)})

	if err := f.ep.assembleConcat(f.info); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if got := f.finalBytes(t); !bytes.Equal(got, f.expected()) {
		t.Fatalf("final = %q, want %q", got, f.expected())
	}
	for _, id := range f.info.PartialUploads {
		if _, err := os.Stat(filepath.Join(f.tempDir, id)); !os.IsNotExist(err) {
			t.Errorf("partial %s still present (err=%v)", id, err)
		}
		if _, err := os.Stat(filepath.Join(f.tempDir, id+".info")); err != nil {
			t.Errorf("partial sidecar %s should survive until cleanupFinalization: %v", id, err)
		}
	}
	if err := f.ep.assembleConcat(f.info); err != nil {
		t.Fatalf("second assemble must be a no-op, got %v", err)
	}
	if got := f.finalBytes(t); !bytes.Equal(got, f.expected()) {
		t.Fatalf("final after re-run = %q, want %q", got, f.expected())
	}
}

func TestAssembleConcatResumesAfterInterruptedAppend(t *testing.T) {
	f := newConcatFixture(t, [][]byte{bytes.Repeat([]byte("a"), 10), bytes.Repeat([]byte("b"), 6), bytes.Repeat([]byte("c"), 4)})

	// Simulate a crash: partial 0 fully appended and removed, partial 1 half
	// appended and still on disk, partial 2 untouched.
	half := append(append([]byte{}, f.partials[0]...), f.partials[1][:3]...)
	if err := os.WriteFile(filepath.Join(f.tempDir, "final"), half, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.tempDir, "part0")); err != nil {
		t.Fatal(err)
	}

	if err := f.ep.assembleConcat(f.info); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if got := f.finalBytes(t); !bytes.Equal(got, f.expected()) {
		t.Fatalf("final = %q, want %q", got, f.expected())
	}
}

func TestAssembleConcatUsesDatabaseSizeWhenSidecarMissing(t *testing.T) {
	f := newConcatFixture(t, [][]byte{bytes.Repeat([]byte("a"), 3), bytes.Repeat([]byte("b"), 8)})
	if err := os.Remove(filepath.Join(f.tempDir, "part1.info")); err != nil {
		t.Fatal(err)
	}
	if err := f.ep.assembleConcat(f.info); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if got := f.finalBytes(t); !bytes.Equal(got, f.expected()) {
		t.Fatalf("final = %q, want %q", got, f.expected())
	}
}

func TestAssembleConcatFailsWhenPartialLostBeforeAppend(t *testing.T) {
	f := newConcatFixture(t, [][]byte{bytes.Repeat([]byte("a"), 10), bytes.Repeat([]byte("b"), 5)})
	if err := os.Remove(filepath.Join(f.tempDir, "part1")); err != nil {
		t.Fatal(err)
	}

	if err := f.ep.assembleConcat(f.info); err == nil {
		t.Fatal("expected error for a missing partial that was never appended")
	}
	if got := f.status(t); got != "failed" {
		t.Fatalf("status = %q, want failed", got)
	}
}

// TestDeferredConcaterFinalPostDoesNotCopy drives a real tusd handler with the
// filestore plus DeferredConcater and checks that the final POST succeeds, emits
// the completion event with the partial IDs, and leaves the final bin empty for
// the asynchronous assembly.
func TestDeferredConcaterFinalPostDoesNotCopy(t *testing.T) {
	tempDir := t.TempDir()
	store := filestore.New(tempDir)
	composer := handler.NewStoreComposer()
	store.UseIn(composer)
	composer.UseConcater(DeferredConcater{})

	h, err := handler.NewHandler(handler.Config{
		BasePath:              "/files/",
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	completed := make(chan handler.HookEvent, 8)
	go func() {
		for ev := range h.CompleteUploads {
			completed <- ev
		}
	}()
	srv := httptest.NewServer(http.StripPrefix("/files/", h))
	defer srv.Close()

	do := func(req *http.Request) *http.Response {
		req.Header.Set("Tus-Resumable", "1.0.0")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}

	parts := []string{"hello ", "world"}
	var urls []string
	for _, p := range parts {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/files/", nil)
		req.Header.Set("Upload-Length", strconv.Itoa(len(p)))
		req.Header.Set("Upload-Concat", "partial")
		res := do(req)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("create partial: %d", res.StatusCode)
		}
		loc := res.Header.Get("Location")
		urls = append(urls, loc)

		req, _ = http.NewRequest(http.MethodPatch, loc, strings.NewReader(p))
		req.Header.Set("Content-Type", "application/offset+octet-stream")
		req.Header.Set("Upload-Offset", "0")
		if res := do(req); res.StatusCode != http.StatusNoContent {
			t.Fatalf("patch partial: %d", res.StatusCode)
		}
		<-completed // partial completion event
	}

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/files/", nil)
	req.Header.Set("Upload-Concat", "final;"+strings.Join(urls, " "))
	res := do(req)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("final POST: %d", res.StatusCode)
	}
	finalID := path(res.Header.Get("Location"))

	ev := <-completed
	if ev.Upload.ID != finalID || len(ev.Upload.PartialUploads) != 2 {
		t.Fatalf("unexpected completion event: %+v", ev.Upload)
	}
	stat, err := os.Stat(filepath.Join(tempDir, finalID))
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != 0 {
		t.Fatalf("final bin has %d bytes; deferred concater must not copy", stat.Size())
	}
	for _, u := range urls {
		if _, err := os.Stat(filepath.Join(tempDir, path(u))); err != nil {
			t.Fatalf("partial %s should still exist: %v", u, err)
		}
	}

	// Now run the asynchronous assembly exactly as finalizeUpload would.
	ep := NewEventProcessor(newTestDB(t), tempDir, tempDir, nil)
	if err := ep.assembleConcat(ev.Upload); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(tempDir, finalID))
	if string(got) != "hello world" {
		t.Fatalf("assembled %q", got)
	}
}

func path(u string) string {
	segs := strings.Split(strings.TrimRight(u, "/"), "/")
	return segs[len(segs)-1]
}
