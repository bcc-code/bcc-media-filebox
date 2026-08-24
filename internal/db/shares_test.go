package db_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	dbpkg "filebox/internal/db"
	db "filebox/internal/db/gen"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// newTestDB opens a throwaway SQLite with migrations applied, using the same
// connection settings as cmd/server so concurrency behaves as in production.
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

func TestSQLiteDSNEnablesForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pragma.db")
	conn, err := sql.Open("sqlite", dbpkg.SQLiteDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var enabled int
	if err := conn.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("read PRAGMA foreign_keys: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", enabled)
	}
}

// Regression test: the limit used to be a separate read-then-increment that two
// overlapping requests could both pass. "Download all" fires every file at once,
// so that interleaving is routine, not rare.
func TestIncrementShareAccessCountIfUnderLimitIsAtomic(t *testing.T) {
	ctx := context.Background()
	queries := newTestDB(t)

	const limit = 3
	const attempts = 25
	user, err := queries.UpsertUser(ctx, db.UpsertUserParams{Provider: "bcc", Subject: "counter-author"})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}
	if err := queries.CreateUpload(ctx, db.CreateUploadParams{
		ID: "upload1", UserID: "bcc:counter-author", Filename: "one.mov", Size: 1,
	}); err != nil {
		t.Fatalf("create upload: %v", err)
	}

	if _, err := queries.CreatePackage(ctx, db.CreatePackageParams{
		ID:                 "pkg1",
		CreatedByUserID:    user.ID,
		Name:               "pkg",
		VerificationMethod: "none",
		ExpiresAt:          time.Now().Add(time.Hour),
		MaxDownloads:       sql.NullInt64{Int64: limit, Valid: true},
	}); err != nil {
		t.Fatalf("create package: %v", err)
	}
	if _, err := queries.CreateShare(ctx, db.CreateShareParams{
		ID:        "share1",
		PackageID: "pkg1",
		UploadID:  "upload1",
	}); err != nil {
		t.Fatalf("create share: %v", err)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		granted  int
		rejected int
	)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := queries.IncrementShareAccessCountIfUnderLimit(ctx, db.IncrementShareAccessCountIfUnderLimitParams{
				ID:             "share1",
				MaxAccessCount: limit,
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				granted++
			case errors.Is(err, sql.ErrNoRows):
				rejected++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if granted != limit {
		t.Errorf("granted %d downloads, want exactly %d", granted, limit)
	}
	if rejected != attempts-limit {
		t.Errorf("rejected %d downloads, want %d", rejected, attempts-limit)
	}

	share, err := queries.GetShareByID(ctx, "share1")
	if err != nil {
		t.Fatalf("get share: %v", err)
	}
	if share.AccessCount != limit {
		t.Errorf("access_count = %d, want %d (limit must never be exceeded)", share.AccessCount, limit)
	}
}

// Pins the boundary: the claim must fail once access_count has reached the
// limit, not after it has passed it.
func TestIncrementShareAccessCountIfUnderLimitRejectsAtLimit(t *testing.T) {
	ctx := context.Background()
	queries := newTestDB(t)
	user, err := queries.UpsertUser(ctx, db.UpsertUserParams{Provider: "bcc", Subject: "limit-author"})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}
	if err := queries.CreateUpload(ctx, db.CreateUploadParams{
		ID: "upload1", UserID: "bcc:limit-author", Filename: "one.mov", Size: 1,
	}); err != nil {
		t.Fatalf("create upload: %v", err)
	}

	if _, err := queries.CreatePackage(ctx, db.CreatePackageParams{
		ID:                 "pkg1",
		CreatedByUserID:    user.ID,
		Name:               "pkg",
		VerificationMethod: "none",
		ExpiresAt:          time.Now().Add(time.Hour),
		MaxDownloads:       sql.NullInt64{Int64: 1, Valid: true},
	}); err != nil {
		t.Fatalf("create package: %v", err)
	}
	if _, err := queries.CreateShare(ctx, db.CreateShareParams{
		ID:        "share1",
		PackageID: "pkg1",
		UploadID:  "upload1",
	}); err != nil {
		t.Fatalf("create share: %v", err)
	}

	params := db.IncrementShareAccessCountIfUnderLimitParams{ID: "share1", MaxAccessCount: 1}

	got, err := queries.IncrementShareAccessCountIfUnderLimit(ctx, params)
	if err != nil {
		t.Fatalf("first claim should succeed: %v", err)
	}
	if got.AccessCount != 1 {
		t.Errorf("access_count after first claim = %d, want 1", got.AccessCount)
	}

	if _, err := queries.IncrementShareAccessCountIfUnderLimit(ctx, params); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("second claim error = %v, want sql.ErrNoRows", err)
	}
}
