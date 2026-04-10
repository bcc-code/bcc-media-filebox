package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	db "file-pusher/internal/db/gen"
	"file-pusher/internal/server"

	_ "modernc.org/sqlite"
)

func main() {
	port := envOr("PORT", "8080")
	uploadDir := envOr("UPLOAD_DIR", "uploads")
	dbPath := envOr("DB_PATH", "file-pusher.db")

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("failed to create upload directory: %v", err)
	}

	database, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if err := runMigrations(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	queries := db.New(database)

	var frontendFS fs.FS
	if ef := embeddedFrontend(); ef != nil {
		if sub, err := fs.Sub(ef, "frontend_dist"); err == nil {
			frontendFS = sub
		}
	}

	srv, err := server.New(queries, uploadDir, frontendFS)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting server on %s", addr)
	log.Printf("Upload directory: %s", uploadDir)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func runMigrations(database *sql.DB) error {
	migration, err := os.ReadFile("internal/db/migrations/001_init.sql")
	if err != nil {
		// Try embedded path for production binary
		migration = []byte(`CREATE TABLE IF NOT EXISTS uploads (
			id TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			size INTEGER NOT NULL,
			offset INTEGER NOT NULL DEFAULT 0,
			content_type TEXT,
			status TEXT NOT NULL DEFAULT 'uploading',
			is_partial INTEGER NOT NULL DEFAULT 0,
			final_upload_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		);`)
	}
	_, err = database.Exec(string(migration))
	return err
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
