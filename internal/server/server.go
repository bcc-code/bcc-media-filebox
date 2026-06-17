package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"filebox/internal/api"
	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/forms"
	"filebox/internal/tus"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tushandler "github.com/tus/tusd/v2/pkg/handler"
)

type Server struct {
	mux      *http.ServeMux
	queries  *db.Queries
	manager  *auth.Manager
	sessions *auth.SessionStore
	baseURL  string
}

// New constructs the HTTP server. The manager and sessions arguments may be
// nil — in that case all auth routes return guest responses and uploads are
// tagged with "guest:<ulid>" user_ids.
func New(queries *db.Queries, uploadDir string, baseURL string, frontendFS fs.FS, manager *auth.Manager, sessions *auth.SessionStore) (*Server, error) {
	s := &Server{
		mux:      http.NewServeMux(),
		queries:  queries,
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

	ep := tus.NewEventProcessor(s.queries, uploadDir, tempDir)
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
	if name := hook.Upload.MetaData["filename"]; name != "" {
		sanitized, err := tus.SanitizeFilename(name)
		if err != nil {
			return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, tushandler.NewError("ERR_INVALID_FILENAME", err.Error(), http.StatusBadRequest)
		}
		hook.Upload.MetaData["filename"] = sanitized
	}

	// When the chosen target is bound to a hardcoded form, the submitted form
	// data must satisfy the form's required fields. Reject early so a bad upload
	// never starts; finalizeUpload re-derives the authoritative filename later.
	if targetName := hook.Upload.MetaData["target"]; targetName != "" {
		if t, err := s.queries.GetTargetByName(hook.Context, targetName); err == nil && t.FormKey.Valid && t.FormKey.String != "" {
			form, ok := forms.Get(t.FormKey.String)
			if !ok {
				return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, tushandler.NewError("ERR_INVALID_FORM", "target form is not available", http.StatusBadRequest)
			}
			values := map[string]string{}
			if raw := hook.Upload.MetaData["formdata"]; raw != "" {
				if err := json.Unmarshal([]byte(raw), &values); err != nil {
					return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, tushandler.NewError("ERR_INVALID_FORM", "invalid form data", http.StatusBadRequest)
				}
			}
			if err := forms.Validate(form, values); err != nil {
				return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, tushandler.NewError("ERR_INVALID_FORM", err.Error(), http.StatusBadRequest)
			}
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
	h := api.NewHandlers(s.queries)
	s.mux.HandleFunc("GET /api/targets", h.ListTargets)
	s.mux.HandleFunc("GET /api/projects", h.ListProjects)
	s.mux.HandleFunc("GET /api/projects/{code}/suggestions", h.ProjectSuggestions)
	s.mux.HandleFunc("GET /api/uploads", h.ListUploads)

	admin := api.NewAdminHandlers(s.queries)
	admin.Register(s.mux)
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
	fileServer := http.FileServerFS(frontendFS)
	indexBytes, _ := fs.ReadFile(frontendFS, "index.html")

	s.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Existing asset? serve it. Otherwise fall through to index.html so
		// the SPA router (/admin, /admin/foo, etc.) can take over.
		if r.URL.Path != "/" {
			if _, err := fs.Stat(frontendFS, strings.TrimPrefix(r.URL.Path, "/")); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		if r.URL.Path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html so vue-router resolves the route
		// client-side. Cache the bytes to avoid stat'ing the FS every time.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexBytes)
	}))
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
