package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"filebox/internal/auth"
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
		r.CompletedAt = new(u.CompletedAt.Time.Format("2006-01-02T15:04:05Z"))
	}
	if u.DurationMs.Valid && u.DurationMs.Int64 > 0 {
		r.DurationMs = &u.DurationMs.Int64
		// Average bandwidth in bytes/sec
		r.AvgBandwidth = new(float64(u.Size) / (float64(u.DurationMs.Int64) / 1000.0))
	}
	return r
}

func (h *Handlers) ListTargets(w http.ResponseWriter, r *http.Request) {
	// Caller is plumbed through here so future per-target ACLs (group/email
	// rules) have an identity to consult. No rules are applied today.
	_ = auth.CallerFrom(r.Context())

	names := make([]string, len(h.targets))
	for i, t := range h.targets {
		names[i] = t.Name
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(names)
}

// ListUploads returns the history for the calling user. When the request is
// authenticated, the session's canonical user_id is authoritative and the
// user_id query parameter is ignored. Guests must supply user_id, but cannot
// peek at authenticated users' histories: any value containing a ":" that
// isn't a "guest:" id is rejected. Legacy raw-ULID ids (no colon) remain
// queryable so pre-OAuth uploads stay accessible.
func (h *Handlers) ListUploads(w http.ResponseWriter, r *http.Request) {
	var userID string
	if caller := auth.CallerFrom(r.Context()); caller != nil {
		userID = caller.CanonicalUserID()
	} else {
		userID = r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "missing user_id parameter", http.StatusBadRequest)
			return
		}
		if strings.Contains(userID, ":") && !strings.HasPrefix(userID, "guest:") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
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
