package api

import (
	"encoding/json"
	"net/http"

	"filebox/internal/config"
	db "filebox/internal/db/gen"
)

type Handlers struct {
	queries *db.Queries
	targets []config.Target
}

func NewHandlers(queries *db.Queries, targets []config.Target) *Handlers {
	return &Handlers{queries: queries, targets: targets}
}

type UploadResponse struct {
	ID           string   `json:"id"`
	Filename     string   `json:"filename"`
	Size         int64    `json:"size"`
	Offset       int64    `json:"offset"`
	ContentType  *string  `json:"contentType"`
	Status       string   `json:"status"`
	DurationMs   *int64   `json:"durationMs"`
	AvgBandwidth *float64 `json:"avgBandwidth"`
	SHA256       *string  `json:"sha256"`
	CreatedAt    string   `json:"createdAt"`
	CompletedAt  *string  `json:"completedAt"`
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
	if u.Sha256.Valid {
		r.SHA256 = &u.Sha256.String
	}
	if u.CompletedAt.Valid {
		t := u.CompletedAt.Time.Format("2006-01-02T15:04:05Z")
		r.CompletedAt = &t
	}
	if u.DurationMs.Valid && u.DurationMs.Int64 > 0 {
		r.DurationMs = &u.DurationMs.Int64
		// Average bandwidth in bytes/sec
		bw := float64(u.Size) / (float64(u.DurationMs.Int64) / 1000.0)
		r.AvgBandwidth = &bw
	}
	return r
}

func (h *Handlers) ListTargets(w http.ResponseWriter, r *http.Request) {
	names := make([]string, len(h.targets))
	for i, t := range h.targets {
		names[i] = t.Name
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(names)
}

func (h *Handlers) ListUploads(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "missing user_id parameter", http.StatusBadRequest)
		return
	}

	uploads, err := h.queries.ListUploads(r.Context(), userID)
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
