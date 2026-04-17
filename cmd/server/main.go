package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"filebox/internal/config"
	dbpkg "filebox/internal/db"
	db "filebox/internal/db/gen"
	"filebox/internal/server"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func main() {
	port := envOr("PORT", "8080")
	uploadDir := envOr("UPLOAD_DIR", "uploads")
	dbPath := envOr("DB_PATH", "filebox.db")
	baseURL := os.Getenv("BASE_URL") // e.g. "https://upload.example.com"

	targets, err := config.LoadTargets()
	if err != nil {
		log.Fatalf("failed to load targets: %v", err)
	}

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("failed to create upload directory: %v", err)
	}

	database, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	database.SetMaxOpenConns(1)
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

	srv, err := server.New(queries, uploadDir, baseURL, frontendFS, targets)
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
	goose.SetBaseFS(dbpkg.Migrations)
	goose.SetDialect("sqlite3")
	return goose.Up(database, "migrations")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
