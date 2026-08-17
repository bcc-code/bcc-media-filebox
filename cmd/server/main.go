package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"filebox/internal/auth"
	"filebox/internal/config"
	dbpkg "filebox/internal/db"
	db "filebox/internal/db/gen"
	"filebox/internal/mail"
	"filebox/internal/objectstore"
	"filebox/internal/server"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func main() {
	// Load .env if present (development convenience). Production deploys use
	// real env vars, so a missing file is fine; anything else is fatal so
	// typos don't silently lose configuration.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("failed to load .env: %v", err)
	}

	port := envOr("PORT", "8080")
	uploadDir := envOr("UPLOAD_DIR", "uploads")
	dbPath := envOr("DB_PATH", "filebox.db")
	baseURL := os.Getenv("BASE_URL") // e.g. "https://upload.example.com"

	envTargets, err := config.LoadTargetsFromEnv()
	if err != nil {
		log.Fatalf("failed to read TARGET_N_* env vars: %v", err)
	}

	authConfig, err := auth.LoadConfig(baseURL)
	if err != nil {
		log.Fatalf("failed to load auth config: %v", err)
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

	if err := config.BootstrapTargets(context.Background(), queries, envTargets); err != nil {
		log.Fatalf("failed to bootstrap targets: %v", err)
	}

	if err := auth.BootstrapAdminFromEnv(context.Background(), queries, os.Getenv("BOOTSTRAP_ADMIN_EMAIL")); err != nil {
		log.Fatalf("failed to bootstrap admin: %v", err)
	}

	var (
		authManager  *auth.Manager
		sessionStore *auth.SessionStore
	)
	if authConfig.Enabled() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		authManager, err = auth.NewManager(ctx, authConfig)
		if err != nil {
			log.Fatalf("failed to initialise OAuth providers: %v", err)
		}
		sessionStore = auth.NewSessionStore(queries, authConfig.CookieSecure)
		ids := make([]string, 0, len(authConfig.Providers))
		for _, p := range authConfig.Providers {
			ids = append(ids, p.ID)
		}
		log.Printf("OAuth enabled (providers: %v)", ids)
	} else {
		log.Println("OAuth disabled (no OIDC_* env vars set) — running in guest-only mode")
	}

	// Send uploads go to S3 when a bucket is configured; without one they fall
	// back to a local target directory like every other upload.
	objectStore, err := objectstore.NewFromEnv(context.Background())
	if err != nil {
		log.Fatalf("failed to initialise S3 object store: %v", err)
	}
	if objectStore != nil {
		log.Printf("S3 enabled for Send uploads (bucket: %s)", objectStore.Bucket())
	} else {
		log.Println("S3 disabled (no S3_BUCKET set) — Send uploads use local targets")
	}

	// Email delivery. Unconfigured is a valid state (NoopSender logs instead of
	// sending), but a configured relay without BASE_URL would mail out links
	// that go nowhere — so that combination is fatal rather than silent.
	mailer, err := mail.NewFromEnv()
	if err != nil {
		log.Fatalf("failed to initialise mail: %v", err)
	}
	// Recipient links default to BASE_URL, but the two differ in dev: BASE_URL
	// is this server's origin (and the OAuth redirect), while a recipient opens
	// the Vite dev server. MAIL_LINK_BASE_URL overrides just the link, and dev
	// builds fall back to Vite so local testing needs no configuration.
	mailBaseURL := envOr("MAIL_LINK_BASE_URL", baseURL)
	if mailBaseURL == "" {
		mailBaseURL = devLinkOrigin // set only in dev builds
	}
	if mail.IsEnabled(mailer) && mailBaseURL == "" {
		log.Fatalf("mail is configured but neither MAIL_LINK_BASE_URL nor BASE_URL is set — recipient links would point at the wrong host (dev builds fall back to the Vite dev server; production must set one explicitly)")
	}
	if mail.IsEnabled(mailer) {
		log.Printf("mail: recipient links point at %s", mailBaseURL)
	}

	var frontendFS fs.FS
	if ef := embeddedFrontend(); ef != nil {
		if sub, err := fs.Sub(ef, "frontend_dist"); err == nil {
			frontendFS = sub
		}
	}

	srv, err := server.New(queries, uploadDir, baseURL, mailBaseURL, frontendFS, authManager, sessionStore, objectStore, mailer)
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
