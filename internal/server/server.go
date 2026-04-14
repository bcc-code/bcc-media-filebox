package server

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"file-pusher/internal/api"
	"file-pusher/internal/config"
	db "file-pusher/internal/db/gen"
	"file-pusher/internal/tus"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tushandler "github.com/tus/tusd/v2/pkg/handler"
)

type Server struct {
	mux     *http.ServeMux
	queries *db.Queries
	targets []config.Target
}

func New(queries *db.Queries, uploadDir string, baseURL string, frontendFS fs.FS, targets []config.Target) (*Server, error) {
	s := &Server{
		mux:     http.NewServeMux(),
		queries: queries,
		targets: targets,
	}

	if err := s.setupTus(uploadDir, baseURL); err != nil {
		return nil, err
	}
	s.setupAPI()
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

func (s *Server) setupAPI() {
	h := api.NewHandlers(s.queries, s.targets)
	s.mux.HandleFunc("GET /api/targets", h.ListTargets)
	s.mux.HandleFunc("GET /api/uploads", h.ListUploads)
}

func (s *Server) setupFrontend(frontendFS fs.FS) {
	if frontendFS == nil {
		log.Println("No embedded frontend, serving API only")
		return
	}
	s.mux.Handle("/", http.FileServerFS(frontendFS))
}

func (s *Server) Handler() http.Handler {
	return s.mux
}
