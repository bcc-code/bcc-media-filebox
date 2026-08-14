package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
)

const (
	defaultSharesPageSize = 20
	maxSharesPageSize     = 100
	maxShareExpiryDays    = 30
)

var validVerificationMethods = map[string]bool{
	"none":       true,
	"email_otp":  true,
	"magic_link": true,
	"password":   true,
	"bcc_login":  true,
}

type CreatePackageRequest struct {
	Name               string   `json:"name"`
	Message            string   `json:"message"`
	UploadIDs          []string `json:"uploadIds"`
	Recipients         []string `json:"recipients"`
	ExpiresInDays      int      `json:"expiresInDays"`
	MaxDownloads       *int     `json:"maxDownloads"`
	VerificationMethod string   `json:"verificationMethod"`
	Password           string   `json:"password"`
	NotifyOnDownload   bool     `json:"notifyOnDownload"`
}

type CreatePackageResponse struct {
	PackageID  string `json:"packageId"`
	PackageURL string `json:"packageUrl"`
	ExpiresAt  string `json:"expiresAt"`
}

// CreatePackage bundles one or more already-uploaded files into a single
// shareable package ("Send"). Each file becomes its own shares row
// (package_id set, requires_auth "none") since package-level verification
// gates access before any individual file is reached.
func (h *Handlers) CreatePackage(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	var req CreatePackageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Name == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	if len(req.UploadIDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "at least one uploadId is required")
		return
	}

	if req.ExpiresInDays <= 0 || req.ExpiresInDays > maxShareExpiryDays {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("expiresInDays is required and must be between 1 and %d", maxShareExpiryDays))
		return
	}

	if req.VerificationMethod == "" {
		req.VerificationMethod = "none"
	}
	if !validVerificationMethods[req.VerificationMethod] {
		writeJSONError(w, http.StatusBadRequest, "invalid verificationMethod")
		return
	}

	var passwordHash sql.NullString
	if req.VerificationMethod == "password" {
		if req.Password == "" {
			writeJSONError(w, http.StatusBadRequest, "password is required for verificationMethod=password")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		passwordHash = sql.NullString{String: string(hash), Valid: true}
	}

	var maxDownloads sql.NullInt64
	if req.MaxDownloads != nil && *req.MaxDownloads > 0 {
		maxDownloads = sql.NullInt64{Int64: int64(*req.MaxDownloads), Valid: true}
	}

	// Verify ownership of every upload before creating anything, so a
	// package is never partially created against a file the caller can't share.
	uploads := make([]db.Upload, 0, len(req.UploadIDs))
	for _, uploadID := range req.UploadIDs {
		upload, err := h.queries.GetUpload(r.Context(), uploadID)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("upload %q not found", uploadID))
			return
		}
		if upload.UserID != caller.CanonicalUserID() {
			writeJSONError(w, http.StatusForbidden, fmt.Sprintf("not authorized to share upload %q", uploadID))
			return
		}
		uploads = append(uploads, upload)
	}

	packageID, err := generateShareID()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to generate package id")
		return
	}

	expTime := time.Now().AddDate(0, 0, req.ExpiresInDays)

	pkg, err := h.queries.CreatePackage(r.Context(), db.CreatePackageParams{
		ID:                 packageID,
		CreatedByUserID:    caller.UserID,
		Name:               req.Name,
		Message:            req.Message,
		VerificationMethod: req.VerificationMethod,
		PasswordHash:       passwordHash,
		ExpiresAt:          expTime,
		MaxDownloads:       maxDownloads,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create package")
		return
	}

	for _, upload := range uploads {
		shareID, err := generateShareID()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to generate share id")
			return
		}
		// No expiry/access-limit set here: a share belonging to a package
		// carries no policy of its own, since the UI only ever lets someone
		// set expiry/limits on the package as a whole. GetShare enforces
		// those against the package, not this row.
		if _, err := h.queries.CreateShare(r.Context(), db.CreateShareParams{
			ID:        shareID,
			PackageID: packageID,
			UploadID:  upload.ID,
		}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to attach file to package")
			return
		}
	}

	for _, email := range req.Recipients {
		if email == "" {
			continue
		}
		if _, err := h.queries.CreatePackageRecipient(r.Context(), db.CreatePackageRecipientParams{
			PackageID: packageID,
			Email:     email,
		}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to add recipient")
			return
		}
	}

	writeJSON(w, http.StatusCreated, CreatePackageResponse{
		PackageID:  pkg.ID,
		PackageURL: packageID,
		ExpiresAt:  expTime.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

type PackageListItem struct {
	PackageID          string   `json:"packageId"`
	Name               string   `json:"name"`
	Message            string   `json:"message"`
	VerificationMethod string   `json:"verificationMethod"`
	Recipients         []string `json:"recipients"`
	FileCount          int      `json:"fileCount"`
	TotalSize          int64    `json:"totalSize"`
	DownloadCount      int64    `json:"downloadCount"`
	MaxDownloads       *int64   `json:"maxDownloads"`
	ExpiresAt          string   `json:"expiresAt"`
	IsExpired          bool     `json:"isExpired"`
	IsDownloadLimitHit bool     `json:"isDownloadLimitHit"`
	Status             string   `json:"status"`
	CreatedAt          string   `json:"createdAt"`
}

type ListPackagesResponse struct {
	Packages []PackageListItem `json:"packages"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
	Total    int64             `json:"total"`
}

// ListPackagesByUser lists the packages created by the authenticated caller,
// paginated ("Sent packages").
func (h *Handlers) ListPackagesByUser(w http.ResponseWriter, r *http.Request) {
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

	total, err := h.queries.CountPackagesByUser(r.Context(), caller.UserID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to count packages")
		return
	}

	rows, err := h.queries.ListPackagesByUser(r.Context(), db.ListPackagesByUserParams{
		CreatedByUserID: caller.UserID,
		Limit:           int64(pageSize),
		Offset:          int64((page - 1) * pageSize),
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}

	now := time.Now()
	packages := make([]PackageListItem, len(rows))
	for i, row := range rows {
		files, err := h.queries.ListSharesByPackageID(r.Context(), row.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to list package files")
			return
		}
		var totalSize int64
		for _, f := range files {
			totalSize += f.Size
		}

		maxAccessCount, err := h.queries.GetPackageMaxAccessCount(r.Context(), row.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to compute package downloads")
			return
		}

		recipientRows, err := h.queries.ListPackageRecipientsByPackageID(r.Context(), row.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to list package recipients")
			return
		}
		recipients := make([]string, len(recipientRows))
		for i, rec := range recipientRows {
			recipients[i] = rec.Email
		}

		var maxDownloads *int64
		if row.MaxDownloads.Valid {
			maxDownloads = new(row.MaxDownloads.Int64)
		}

		packages[i] = PackageListItem{
			PackageID:          row.ID,
			Name:               row.Name,
			Message:            row.Message,
			VerificationMethod: row.VerificationMethod,
			Recipients:         recipients,
			FileCount:          len(files),
			TotalSize:          totalSize,
			DownloadCount:      maxAccessCount,
			MaxDownloads:       maxDownloads,
			ExpiresAt:          row.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
			IsExpired:          row.Status == "active" && row.ExpiresAt.Before(now),
			IsDownloadLimitHit: row.MaxDownloads.Valid && row.DownloadCount >= row.MaxDownloads.Int64,
			Status:             row.Status,
			CreatedAt:          row.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, ListPackagesResponse{
		Packages: packages,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

// RevokePackage revokes a package. Only the package's creator may revoke it.
// Revocation is a soft status flip, not a delete, so the package still shows
// up (as "revoked") in the creator's package list.
func (h *Handlers) RevokePackage(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	packageID := r.PathValue("id")

	pkg, err := h.queries.GetPackageByID(r.Context(), packageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "package not found")
		return
	}

	if pkg.CreatedByUserID != caller.UserID {
		writeJSONError(w, http.StatusForbidden, "not authorized to revoke this package")
		return
	}

	if err := h.queries.RevokePackage(r.Context(), packageID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to revoke package")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
