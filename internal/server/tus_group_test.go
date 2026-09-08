package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	dbpkg "filebox/internal/db"
	db "filebox/internal/db/gen"
	"filebox/internal/tus"

	"github.com/pressly/goose/v3"
	tushandler "github.com/tus/tusd/v2/pkg/handler"
	_ "modernc.org/sqlite"
)

func newTestServer(t *testing.T) *Server {
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
	// No sessions: every caller is a guest identified by its userid metadata,
	// which is what lets one test act as two different users.
	return &Server{queries: db.New(conn), groups: tus.NewGroupRegistry()}
}

// partHook builds the creation hook event for one part of a parallel upload.
func partHook(userID, group, boundaries string, total, partSize int64) tushandler.HookEvent {
	return tushandler.HookEvent{
		Context: context.Background(),
		Upload: tushandler.FileInfo{
			Size:      partSize,
			IsPartial: true,
			MetaData: tushandler.MetaData{
				"userid":           userID,
				tus.MetaGroup:      group,
				tus.MetaTotal:      tus.FormatInt(total),
				tus.MetaBoundaries: boundaries,
			},
		},
	}
}

func finalHook(userID, group string, total int64) tushandler.HookEvent {
	return tushandler.HookEvent{
		Context: context.Background(),
		Upload: tushandler.FileInfo{
			Size:    total,
			IsFinal: true,
			MetaData: tushandler.MetaData{
				"userid":      userID,
				"filename":    "clip.mov",
				tus.MetaGroup: group,
				tus.MetaTotal: tus.FormatInt(total),
			},
		},
	}
}

func statusOf(t *testing.T, err error) int {
	t.Helper()
	var hErr tushandler.Error
	if !errors.As(err, &hErr) {
		t.Fatalf("expected a tus handler error, got %T: %v", err, err)
	}
	return hErr.HTTPResponse.StatusCode
}

const (
	testGroup      = "01HZZZTESTGROUP000000000"
	testBoundaries = `[{"start":0,"end":10},{"start":10,"end":30},{"start":30,"end":60}]`
	testTotal      = int64(60)
)

// Each part is placed by its length alone, which is the only per-part signal
// tus-js-client leaves the server.
func TestPreUploadCreateAssignsPartOffsetByLength(t *testing.T) {
	s := newTestServer(t)
	for _, c := range []struct {
		size, wantOffset int64
	}{
		{size: 10, wantOffset: 0},
		{size: 20, wantOffset: 10},
		{size: 30, wantOffset: 30},
	} {
		_, changes, err := s.preUploadCreate(partHook("alice", testGroup, testBoundaries, testTotal, c.size))
		if err != nil {
			t.Fatalf("size %d: unexpected error: %v", c.size, err)
		}
		if got := changes.MetaData[tus.MetaPartOffset]; got != tus.FormatInt(c.wantOffset) {
			t.Errorf("size %d: partOffset = %q, want %q", c.size, got, tus.FormatInt(c.wantOffset))
		}
		if changes.ID != "" {
			t.Errorf("size %d: a part must not be given the group id", c.size)
		}
	}
}

// The final upload's id has to be the group id, or finalizeUpload looks for the
// finished bytes at tempDir/<some other id>.
func TestPreUploadCreateGivesFinalUploadTheGroupID(t *testing.T) {
	s := newTestServer(t)
	// Claim the group as alice first, the way the parts would.
	if _, _, err := s.preUploadCreate(partHook("alice", testGroup, testBoundaries, testTotal, 10)); err != nil {
		t.Fatal(err)
	}

	_, changes, err := s.preUploadCreate(finalHook("alice", testGroup, testTotal))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changes.ID != testGroup {
		t.Fatalf("final upload id = %q, want %q", changes.ID, testGroup)
	}
	if changes.MetaData[tus.MetaPartOffset] != "" {
		t.Error("the final upload owns the whole file and must carry no part offset")
	}
}

// A part the server cannot place unambiguously is refused before any bytes are
// accepted, so a mismatch can never corrupt data.
func TestPreUploadCreateRejectsUnplaceableAndUnsafeGroups(t *testing.T) {
	cases := []struct {
		name       string
		hook       tushandler.HookEvent
		wantStatus int
	}{
		{
			name:       "length matches no boundary",
			hook:       partHook("alice", testGroup, testBoundaries, testTotal, 15),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate part lengths are unresolvable",
			hook:       partHook("alice", testGroup, `[{"start":0,"end":30},{"start":30,"end":60}]`, testTotal, 30),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "boundaries do not cover the file",
			hook:       partHook("alice", testGroup, `[{"start":0,"end":10},{"start":10,"end":30}]`, testTotal, 10),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "boundaries are not contiguous",
			hook:       partHook("alice", testGroup, `[{"start":0,"end":10},{"start":20,"end":60}]`, testTotal, 10),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "boundaries are not json",
			hook:       partHook("alice", testGroup, "not-json", testTotal, 10),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "total is not a positive integer",
			hook:       partHook("alice", testGroup, testBoundaries, 0, 10),
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, c := range cases {
		s := newTestServer(t)
		_, _, err := s.preUploadCreate(c.hook)
		if err == nil {
			t.Errorf("%s: expected rejection, got none", c.name)
			continue
		}
		if got := statusOf(t, err); got != c.wantStatus {
			t.Errorf("%s: status = %d, want %d", c.name, got, c.wantStatus)
		}
	}
}

// The parts of one upload write into a shared file with no mutual exclusion,
// so a group must belong to exactly one user.
func TestPreUploadCreateRefusesAnotherUsersGroup(t *testing.T) {
	s := newTestServer(t)
	if _, _, err := s.preUploadCreate(partHook("alice", testGroup, testBoundaries, testTotal, 10)); err != nil {
		t.Fatalf("alice's first part: %v", err)
	}

	_, _, err := s.preUploadCreate(partHook("bob", testGroup, testBoundaries, testTotal, 20))
	if err == nil {
		t.Fatal("bob must not be able to inject a part into alice's group")
	}
	if got := statusOf(t, err); got != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", got, http.StatusForbidden)
	}

	// bob must not be able to hijack the final upload either.
	if _, _, err := s.preUploadCreate(finalHook("bob", testGroup, testTotal)); err == nil {
		t.Fatal("bob must not be able to finalize alice's group")
	}

	// And alice is unaffected by the attempts.
	if _, _, err := s.preUploadCreate(partHook("alice", testGroup, testBoundaries, testTotal, 20)); err != nil {
		t.Fatalf("alice lost her group: %v", err)
	}
}

// A final POST whose parts do not add up is refused rather than publishing a
// file with holes in it.
func TestPreUploadCreateRejectsFinalWithWrongTotal(t *testing.T) {
	s := newTestServer(t)
	hook := finalHook("alice", testGroup, testTotal)
	hook.Upload.Size = testTotal - 1
	if _, _, err := s.preUploadCreate(hook); err == nil {
		t.Fatal("a final upload smaller than the declared total must be refused")
	} else if got := statusOf(t, err); got != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, http.StatusBadRequest)
	}
}

// An upload with no group metadata must come through untouched, so
// single-stream uploads and clients predating this change keep working.
func TestPreUploadCreateLeavesNonGroupUploadsAlone(t *testing.T) {
	s := newTestServer(t)
	hook := tushandler.HookEvent{
		Context: context.Background(),
		Upload: tushandler.FileInfo{
			Size: 1234,
			MetaData: tushandler.MetaData{
				"userid":   "alice",
				"filename": "clip.mov",
			},
		},
	}
	_, changes, err := s.preUploadCreate(hook)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changes.ID != "" {
		t.Errorf("ID = %q, want it left to the store", changes.ID)
	}
	if changes.MetaData[tus.MetaPartOffset] != "" || changes.MetaData[tus.MetaGroup] != "" {
		t.Errorf("group metadata invented for a plain upload: %#v", changes.MetaData)
	}
	if changes.MetaData["filename"] != "clip.mov" {
		t.Errorf("filename = %q, want it preserved", changes.MetaData["filename"])
	}
}
