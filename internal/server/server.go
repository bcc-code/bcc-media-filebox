package server

import (
	"io/fs"
	"log"
	"net/http"

	db "file-pusher/internal/db/gen"
	"file-pusher/internal/api"
	"file-pusher/internal/tus"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tushandler "github.com/tus/tusd/v2/pkg/handler"
)

type Server struct {
	mux     *http.ServeMux
	queries *db.Queries
}

func New(queries *db.Queries, uploadDir string, frontendFS fs.FS) (*Server, error) {
	s := &Server{
		mux:     http.NewServeMux(),
		queries: queries,
	}

	if err := s.setupTus(uploadDir); err != nil {
		return nil, err
	}
	s.setupAPI()
	s.setupFrontend(frontendFS)

	return s, nil
}

func (s *Server) setupTus(uploadDir string) error {
	store := filestore.New(uploadDir)
	locker := filelocker.New(uploadDir)

	composer := tushandler.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	h, err := tushandler.NewHandler(tushandler.Config{
		BasePath:                "/files/",
		StoreComposer:           composer,
		NotifyCompleteUploads:   true,
		NotifyCreatedUploads:    true,
		NotifyTerminatedUploads: true,
		NotifyUploadProgress:    true,
	})
	if err != nil {
		return err
	}

	ep := tus.NewEventProcessor(s.queries, uploadDir)
	go ep.Run(h.UnroutedHandler)

	s.mux.Handle("/files/", http.StripPrefix("/files/", h))
	return nil
}

func (s *Server) setupAPI() {
	h := api.NewHandlers(s.queries)
	s.mux.HandleFunc("GET /api/uploads", h.ListUploads)
	s.mux.HandleFunc("DELETE /api/uploads/{id}", h.DeleteUpload)
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
