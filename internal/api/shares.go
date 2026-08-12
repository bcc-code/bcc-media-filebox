package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
)

const (
	defaultSharesPageSize = 20
	maxSharesPageSize     = 100
	maxShareExpiryDays    = 30
)

type CreateShareRequest struct {
	UploadID       string `json:"uploadId"`
	ExpiresInDays  int    `json:"expiresInDays"`
	RequiresAuth   string `json:"requiresAuth"`
	MaxAccessCount *int   `json:"maxAccessCount"`
}

type CreateShareResponse struct {
	ShareID        string `json:"shareId"`
	ShareURL       string `json:"shareUrl"`
	ExpiresAt      string `json:"expiresAt"`
	MaxAccessCount *int64 `json:"maxAccessCount"`
}

type ShareResponse struct {
	ID             string  `json:"id"`
	UploadID       string  `json:"uploadId"`
	CreatedByID    int64   `json:"createdById"`
	ExpiresAt      *string `json:"expiresAt"`
	AccessCount    int64   `json:"accessCount"`
	MaxAccessCount *int64  `json:"maxAccessCount"`
	RequiresAuth   string  `json:"requiresAuth"`
	CreatedAt      string  `json:"createdAt"`
}

func toShareResponse(s db.Share) ShareResponse {
	var expiresAt *string
	if s.ExpiresAt.Valid {
		expiresAt = new(s.ExpiresAt.Time.Format("2006-01-02T15:04:05Z"))
	}
	var maxAccessCount *int64
	if s.MaxAccessCount.Valid {
		maxAccessCount = new(s.MaxAccessCount.Int64)
	}
	return ShareResponse{
		ID:             s.ID,
		UploadID:       s.UploadID,
		CreatedByID:    s.CreatedByUserID,
		ExpiresAt:      expiresAt,
		AccessCount:    s.AccessCount,
		MaxAccessCount: maxAccessCount,
		RequiresAuth:   s.RequiresAuth,
		CreatedAt:      s.CreatedAt.Format("2006-01-02T15:04:05Z"),
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

	if req.ExpiresInDays <= 0 || req.ExpiresInDays > maxShareExpiryDays {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("expiresInDays is required and must be between 1 and %d", maxShareExpiryDays))
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

	expTime := time.Now().AddDate(0, 0, req.ExpiresInDays)
	expiresAt := sql.NullTime{Time: expTime, Valid: true}
	expiresAtStr := expTime.Format("2006-01-02T15:04:05Z")

	var maxAccessCount sql.NullInt64
	if req.MaxAccessCount != nil && *req.MaxAccessCount > 0 {
		maxAccessCount = sql.NullInt64{Int64: int64(*req.MaxAccessCount), Valid: true}
	}

	params := db.CreateShareParams{
		ID:              shareID,
		CreatedByUserID: caller.UserID,
		UploadID:        req.UploadID,
		ExpiresAt:       expiresAt,
		RequiresAuth:    req.RequiresAuth,
		MaxAccessCount:  maxAccessCount,
	}

	share, err := h.queries.CreateShare(r.Context(), params)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create share")
		return
	}

	var maxAccessCountResp *int64
	if share.MaxAccessCount.Valid {
		maxAccessCountResp = new(share.MaxAccessCount.Int64)
	}

	writeJSON(w, http.StatusCreated, CreateShareResponse{
		ShareID:        share.ID,
		ShareURL:       shareID,
		ExpiresAt:      expiresAtStr,
		MaxAccessCount: maxAccessCountResp,
	})
}

func generateShareID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GetShare retrieves a share by ID and serves the underlying file, provided
// the share hasn't expired and, for shares requiring BCC auth, the caller is
// logged in via the "bcc" provider.
func (h *Handlers) GetShare(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("id")

	share, err := h.queries.GetShareByID(r.Context(), shareID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "share not found")
		return
	}

	if share.ExpiresAt.Valid && !share.ExpiresAt.Time.After(time.Now()) {
		writeJSONError(w, http.StatusGone, "share has expired")
		return
	}

	if share.MaxAccessCount.Valid && share.AccessCount >= share.MaxAccessCount.Int64 {
		writeJSONError(w, http.StatusGone, "share access limit reached")
		return
	}

	if share.RequiresAuth == "bcc" {
		caller := auth.CallerFrom(r.Context())
		if caller == nil || caller.Provider != "bcc" {
			writeJSONError(w, http.StatusForbidden, "authentication required")
			return
		}
	}

	upload, err := h.queries.GetUpload(r.Context(), share.UploadID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "file not found")
		return
	}

	// Mirrors the directory resolution in internal/tus's finalizeUpload: files
	// land under the target's configured path, or uploadDir/RawMaterial when
	// the upload has no target.
	targetDir := filepath.Join(h.uploadDir, "RawMaterial")
	if upload.TargetName.Valid && upload.TargetName.String != "" {
		if target, err := h.queries.GetTargetByName(r.Context(), upload.TargetName.String); err == nil {
			targetDir = target.Path
		}
	}
	filePath := filepath.Join(targetDir, upload.Filename)

	if _, err := os.Stat(filePath); err != nil {
		writeJSONError(w, http.StatusNotFound, "file not found on disk")
		return
	}

	if _, err := h.queries.UpdateShareAccessCount(r.Context(), shareID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", upload.Filename))
	http.ServeFile(w, r, filePath)
}

// ListSharesByUpload lists all shares for an upload. Only the upload's owner may view them.
func (h *Handlers) ListSharesByUpload(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	uploadID := r.PathValue("id")

	upload, err := h.queries.GetUpload(r.Context(), uploadID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "upload not found")
		return
	}

	if upload.UserID != caller.CanonicalUserID() {
		writeJSONError(w, http.StatusForbidden, "not authorized to view shares for this upload")
		return
	}

	shares, err := h.queries.GetSharesByUploadID(r.Context(), uploadID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list shares")
		return
	}

	result := make([]ShareResponse, len(shares))
	for i, s := range shares {
		result[i] = toShareResponse(s)
	}

	writeJSON(w, http.StatusOK, result)
}

type ShareListItem struct {
	ShareID        string  `json:"shareId"`
	UploadID       string  `json:"uploadId"`
	Filename       string  `json:"filename"`
	CreatedAt      string  `json:"createdAt"`
	ExpiresAt      *string `json:"expiresAt"`
	IsExpired      bool    `json:"isExpired"`
	AccessCount    int64   `json:"accessCount"`
	MaxAccessCount *int64  `json:"maxAccessCount"`
}

type ListSharesResponse struct {
	Shares   []ShareListItem `json:"shares"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}

// ListSharesByUser lists the shares created by the authenticated caller, paginated.
func (h *Handlers) ListSharesByUser(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}

	pageSize := defaultSharesPageSize
	if raw := r.URL.Query().Get("pageSize"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			pageSize = v
		}
	}
	if pageSize > maxSharesPageSize {
		pageSize = maxSharesPageSize
	}

	total, err := h.queries.CountSharesByUserID(r.Context(), caller.UserID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to count shares")
		return
	}

	rows, err := h.queries.ListSharesByUserPaginated(r.Context(), db.ListSharesByUserPaginatedParams{
		CreatedByUserID: caller.UserID,
		Limit:           int64(pageSize),
		Offset:          int64((page - 1) * pageSize),
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list shares")
		return
	}

	now := time.Now()
	shares := make([]ShareListItem, len(rows))
	for i, row := range rows {
		var expiresAt *string
		isExpired := false
		if row.ExpiresAt.Valid {
			expiresAt = new(row.ExpiresAt.Time.Format("2006-01-02T15:04:05Z"))
			isExpired = row.ExpiresAt.Time.Before(now)
		}
		var maxAccessCount *int64
		if row.MaxAccessCount.Valid {
			maxAccessCount = new(row.MaxAccessCount.Int64)
		}
		shares[i] = ShareListItem{
			ShareID:        row.ShareID,
			UploadID:       row.UploadID,
			Filename:       row.Filename,
			CreatedAt:      row.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ExpiresAt:      expiresAt,
			IsExpired:      isExpired,
			AccessCount:    row.AccessCount,
			MaxAccessCount: maxAccessCount,
		}
	}

	writeJSON(w, http.StatusOK, ListSharesResponse{
		Shares:   shares,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

// DeleteShare revokes a share. Only the share's creator may delete it.
func (h *Handlers) DeleteShare(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	shareID := r.PathValue("id")

	share, err := h.queries.GetShareByID(r.Context(), shareID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "share not found")
		return
	}

	if share.CreatedByUserID != caller.UserID {
		writeJSONError(w, http.StatusForbidden, "not authorized to delete this share")
		return
	}

	if err := h.queries.DeleteShare(r.Context(), shareID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to delete share")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
