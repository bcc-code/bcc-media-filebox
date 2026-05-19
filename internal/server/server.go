package server

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"filebox/internal/api"
	"filebox/internal/auth"
	"filebox/internal/config"
	db "filebox/internal/db/gen"
	"filebox/internal/tus"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tushandler "github.com/tus/tusd/v2/pkg/handler"
)

type Server struct {
	mux      *http.ServeMux
	queries  *db.Queries
	targets  []config.Target
	manager  *auth.Manager
	sessions *auth.SessionStore
	baseURL  string
}

// New constructs the HTTP server. The manager and sessions arguments may be
// nil — in that case all auth routes return guest responses and uploads are
// tagged with "guest:<ulid>" user_ids.
func New(queries *db.Queries, uploadDir string, baseURL string, frontendFS fs.FS, targets []config.Target, manager *auth.Manager, sessions *auth.SessionStore) (*Server, error) {
	s := &Server{
		mux:      http.NewServeMux(),
		queries:  queries,
		targets:  targets,
		manager:  manager,
		sessions: sessions,
		baseURL:  baseURL,
	}

	if err := s.setupTus(uploadDir, baseURL); err != nil {
		return nil, err
	}
	s.setupAPI()
	s.setupAuth()
	s.setupFrontend(frontendFS)

	return s, nil
}

func (s *Server) setupTus(uploadDir string, baseURL string) error {
	tempDir := filepath.Join(uploadDir, ".tmp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("create temp upload dir: %w", err)
	}
	store := filestore.New(tempDir)
	locker := filelocker.New(tempDir)

	composer := tushandler.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	tusConfig := tushandler.Config{
		BasePath:                "/files/",
		StoreComposer:           composer,
		NotifyCompleteUploads:   true,
		NotifyCreatedUploads:    true,
		NotifyTerminatedUploads: true,
		NotifyUploadProgress:    true,
		PreUploadCreateCallback: s.preUploadCreate,
	}

	if baseURL != "" {
		tusConfig.BasePath = baseURL + "/files/"
	}

	h, err := tushandler.NewHandler(tusConfig)
	if err != nil {
		return err
	}

	ep := tus.NewEventProcessor(s.queries, uploadDir, tempDir, s.targets)
	go ep.Run(h.UnroutedHandler)

	s.mux.Handle("/files/", http.StripPrefix("/files/", h))
	return nil
}

// preUploadCreate validates the filename and enforces the canonical user_id
// metadata before the upload row is created. The client's "userid" value is
// always overwritten: authenticated callers get "<provider>:<subject>"; guests
// get "guest:<sanitised-token>". This is the chokepoint that prevents a guest
// from impersonating an authenticated user via spoofed metadata.
func (s *Server) preUploadCreate(hook tushandler.HookEvent) (tushandler.HTTPResponse, tushandler.FileInfoChanges, error) {
	name := hook.Upload.MetaData["filename"]
	if name != "" {
		if _, err := tus.SanitizeFilename(name); err != nil {
			return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, tushandler.NewError("ERR_INVALID_FILENAME", err.Error(), http.StatusBadRequest)
		}
	}

	newMeta := make(tushandler.MetaData, len(hook.Upload.MetaData)+1)
	for k, v := range hook.Upload.MetaData {
		newMeta[k] = v
	}

	canonical, err := s.resolveUploadUserID(hook)
	if err != nil {
		return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, tushandler.NewError("ERR_INVALID_USERID", err.Error(), http.StatusBadRequest)
	}
	newMeta["userid"] = canonical

	return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{MetaData: newMeta}, nil
}

func (s *Server) resolveUploadUserID(hook tushandler.HookEvent) (string, error) {
	if s.sessions != nil {
		sid := cookieValue(hook.HTTPRequest.Header, auth.SessionCookieName)
		if sid != "" {
			if caller, _ := s.sessions.LookupByID(hook.Context, sid); caller != nil {
				return caller.CanonicalUserID(), nil
			}
		}
	}
	// Guest path: accept either a raw ULID (legacy clients) or an already-
	// prefixed "guest:<token>" (current client) — but never a provider-style
	// "<provider>:<subject>" id that could collide with an authenticated user.
	raw := strings.TrimPrefix(hook.Upload.MetaData["userid"], "guest:")
	if raw == "" || strings.Contains(raw, ":") {
		// Empty or suspicious — substitute a server-generated token. The
		// client loses correlation with its own history, but cannot impersonate.
		var err error
		raw, err = randomGuestID()
		if err != nil {
			return "", err
		}
	}
	return "guest:" + raw, nil
}

func (s *Server) setupAPI() {
	h := api.NewHandlers(s.queries, s.targets)
	s.mux.HandleFunc("GET /api/targets", h.ListTargets)
	s.mux.HandleFunc("GET /api/uploads", h.ListUploads)
}

func (s *Server) setupAuth() {
	h := auth.NewHandlers(s.manager, s.sessions, s.queries, s.baseURL)
	h.Register(s.mux)
}

func (s *Server) setupFrontend(frontendFS fs.FS) {
	if frontendFS == nil {
		log.Println("No embedded frontend, serving API only")
		return
	}
	s.mux.Handle("/", http.FileServerFS(frontendFS))
}

func (s *Server) Handler() http.Handler {
	if s.sessions != nil {
		return s.sessions.Middleware(s.mux)
	}
	return s.mux
}

// cookieValue extracts a named cookie from a raw header set. It uses the
// stdlib parser via a synthetic request so we get the same semantics as
// http.Request.Cookie elsewhere in the codebase.
func cookieValue(h http.Header, name string) string {
	req := http.Request{Header: h}
	c, err := req.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
