package api

import (
	"encoding/json"
	"net/http"

	db "file-pusher/internal/db/gen"
)

type Handlers struct {
	queries *db.Queries
}

func NewHandlers(queries *db.Queries) *Handlers {
	return &Handlers{queries: queries}
}

type UploadResponse struct {
	ID          string  `json:"id"`
	Filename    string  `json:"filename"`
	Size        int64   `json:"size"`
	Offset      int64   `json:"offset"`
	ContentType *string `json:"contentType"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"createdAt"`
	CompletedAt *string `json:"completedAt"`
}

func toResponse(u db.Upload) UploadResponse {
	r := UploadResponse{
		ID:        u.ID,
		Filename:  u.Filename,
		Size:      u.Size,
		Offset:    u.Offset,
		Status:    u.Status,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if u.ContentType.Valid {
		r.ContentType = &u.ContentType.String
	}
	if u.CompletedAt.Valid {
		t := u.CompletedAt.Time.Format("2006-01-02T15:04:05Z")
		r.CompletedAt = &t
	}
	return r
}

func (h *Handlers) ListUploads(w http.ResponseWriter, r *http.Request) {
	uploads, err := h.queries.ListUploads(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]UploadResponse, len(uploads))
	for i, u := range uploads {
		result[i] = toResponse(u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handlers) DeleteUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing upload id", http.StatusBadRequest)
		return
	}

	err := h.queries.DeleteUpload(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
