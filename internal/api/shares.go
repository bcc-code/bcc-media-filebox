package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
)

type CreateShareRequest struct {
	UploadID      string `json:"uploadId"`
	ExpiresInDays *int   `json:"expiresInDays"`
	RequiresAuth  string `json:"requiresAuth"`
}

type CreateShareResponse struct {
	ShareID   string  `json:"shareId"`
	ShareURL  string  `json:"shareUrl"`
	ExpiresAt *string `json:"expiresAt"`
}

type ShareResponse struct {
	ID           string  `json:"id"`
	UploadID     string  `json:"uploadId"`
	CreatedByID  int64   `json:"createdById"`
	ExpiresAt    *string `json:"expiresAt"`
	AccessCount  int64   `json:"accessCount"`
	RequiresAuth string  `json:"requiresAuth"`
	CreatedAt    string  `json:"createdAt"`
}

func toShareResponse(s db.Share) ShareResponse {
	var expiresAt *string
	if s.ExpiresAt.Valid {
		expiresAt = new(s.ExpiresAt.Time.Format("2006-01-02T15:04:05Z"))
	}
	return ShareResponse{
		ID:           s.ID,
		UploadID:     s.UploadID,
		CreatedByID:  s.CreatedByUserID,
		ExpiresAt:    expiresAt,
		AccessCount:  s.AccessCount,
		RequiresAuth: s.RequiresAuth,
		CreatedAt:    s.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// CreateShare creates a new share
func (h *Handlers) CreateShare(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	var req CreateShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.UploadID == "" {
		writeJSONError(w, http.StatusBadRequest, "uploadId is required")
		return
	}

	if req.RequiresAuth == "" {
		req.RequiresAuth = "none"
	}

	shareID, err := generateShareID()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to generate share id")
		return
	}

	var expiresAt sql.NullTime
	var expiresAtStr *string
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		expTime := time.Now().AddDate(0, 0, *req.ExpiresInDays)
		expiresAt = sql.NullTime{Time: expTime, Valid: true}
		expiresAtStr = new(expTime.Format("2006-01-02T15:04:05Z"))
	}

	params := db.CreateShareParams{
		ID:               shareID,
		CreatedByUserID:  caller.UserID,
		UploadID:         req.UploadID,
		ExpiresAt:        expiresAt,
		RequiresAuth:     req.RequiresAuth,
	}

	share, err := h.queries.CreateShare(r.Context(), params)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create share")
		return
	}

	writeJSON(w, http.StatusCreated, CreateShareResponse{
		ShareID:   share.ID,
		ShareURL:  shareID,
		ExpiresAt: expiresAtStr,
	})
}

func generateShareID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GetShare retrieves a share by ID
func (h *Handlers) GetShare(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
}

// ListSharesByUpload lists all shares for an upload
func (h *Handlers) ListSharesByUpload(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
}

// ListSharesByUser lists all shares created by a user
func (h *Handlers) ListSharesByUser(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
}

// DeleteShare deletes a share
func (h *Handlers) DeleteShare(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
}
