package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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

// CreatePackage bundles already-uploaded files into one shareable package
// ("Send"). Each file becomes its own shares row; verification is enforced at
// the package level, before any individual file is reached.
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

	// Reject a non-positive budget rather than reading it as unlimited — that
	// would silently do the opposite of what the sender asked for.
	if req.MaxDownloads != nil && *req.MaxDownloads < 1 {
		writeJSONError(w, http.StatusBadRequest, "maxDownloads must be at least 1, or omitted for unlimited")
		return
	}

	var maxDownloads sql.NullInt64
	if req.MaxDownloads != nil {
		maxDownloads = sql.NullInt64{Int64: int64(*req.MaxDownloads), Valid: true}
	}

	// Check every upload first, so a package is never partly created against a
	// file the caller can't share.
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
		// No expiry or limit here: a share in a package carries no policy of its
		// own, and GetShare enforces the package's instead.
		if _, err := h.queries.CreateShare(r.Context(), db.CreateShareParams{
			ID:        shareID,
			PackageID: packageID,
			UploadID:  upload.ID,
		}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to attach file to package")
			return
		}
	}

	recipients := make([]db.PackageRecipient, 0, len(req.Recipients))
	for _, email := range req.Recipients {
		if email == "" {
			continue
		}
		rcpt, err := h.queries.CreatePackageRecipient(r.Context(), db.CreatePackageRecipientParams{
			PackageID: packageID,
			Email:     email,
		})
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to add recipient")
			return
		}
		recipients = append(recipients, rcpt)
	}

	// Detached: the package exists either way, and delivery state lands on
	// package_recipients.
	go h.notifyRecipients(pkg, recipients, uploads, caller)

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
	// Carried here so the author sees asks on the card itself, not only in the
	// email that may be lost in an inbox.
	PendingRequests []AccessRequestView `json:"pendingRequests"`
}

// AccessRequestView is one row of package_access_requests as the author sees it.
type AccessRequestView struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
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

	packages, err := h.buildPackageListItems(r.Context(), rows)
	if err != nil {
		// Logged, not returned: the cause is a DB error, and writeJSONError's text
		// is shown to the user as-is.
		log.Printf("packages: list for user %d: %v", caller.UserID, err)
		writeJSONError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}

	writeJSON(w, http.StatusOK, ListPackagesResponse{
		Packages: packages,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

// buildPackageListItem is the one-package form, used by ExtendPackage.
func (h *Handlers) buildPackageListItem(ctx context.Context, pkg db.Package) (PackageListItem, error) {
	items, err := h.buildPackageListItems(ctx, []db.Package{pkg})
	if err != nil {
		return PackageListItem{}, err
	}
	return items[0], nil
}

// buildPackageListItems assembles the author-facing view of a page of packages.
// Four batch queries, not five per package: they all queue on the one connection.
func (h *Handlers) buildPackageListItems(ctx context.Context, pkgs []db.Package) ([]PackageListItem, error) {
	ids := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		ids[i] = pkg.ID
	}

	fileRows, err := h.queries.ListSharesByPackageIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list package files: %w", err)
	}
	fileCount := make(map[string]int, len(pkgs))
	totalSize := make(map[string]int64, len(pkgs))
	for _, f := range fileRows {
		fileCount[f.PackageID]++
		totalSize[f.PackageID] += f.Size
	}

	// A package with no shares gets no row, which is the COALESCE(...,0) the
	// single-package queries apply.
	countRows, err := h.queries.GetPackageAccessCountsByPackageIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("compute package downloads: %w", err)
	}
	maxAccess := make(map[string]int64, len(pkgs))
	minAccess := make(map[string]int64, len(pkgs))
	for _, c := range countRows {
		maxAccess[c.PackageID] = c.MaxAccessCount
		minAccess[c.PackageID] = c.MinAccessCount
	}

	recipientRows, err := h.queries.ListPackageRecipientsByPackageIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list package recipients: %w", err)
	}
	recipients := make(map[string][]string, len(pkgs))
	for _, rec := range recipientRows {
		recipients[rec.PackageID] = append(recipients[rec.PackageID], rec.Email)
	}

	requestRows, err := h.queries.ListPendingPackageAccessRequestsByPackageIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list access requests: %w", err)
	}
	requests := make(map[string][]AccessRequestView, len(pkgs))
	for _, req := range requestRows {
		requests[req.PackageID] = append(requests[req.PackageID], AccessRequestView{
			ID:        req.ID,
			Email:     req.Email,
			Reason:    req.Reason,
			Message:   req.Message,
			CreatedAt: req.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	items := make([]PackageListItem, len(pkgs))
	for i, pkg := range pkgs {
		var maxDownloads *int64
		if pkg.MaxDownloads.Valid {
			maxDownloads = &pkg.MaxDownloads.Int64
		}
		// Empty, not nil, so the JSON keeps [] as before.
		recs := recipients[pkg.ID]
		if recs == nil {
			recs = []string{}
		}
		asks := requests[pkg.ID]
		if asks == nil {
			asks = []AccessRequestView{}
		}
		items[i] = PackageListItem{
			PackageID:          pkg.ID,
			Name:               pkg.Name,
			Message:            pkg.Message,
			VerificationMethod: pkg.VerificationMethod,
			Recipients:         recs,
			FileCount:          fileCount[pkg.ID],
			TotalSize:          totalSize[pkg.ID],
			DownloadCount:      maxAccess[pkg.ID],
			MaxDownloads:       maxDownloads,
			ExpiresAt:          pkg.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
			IsExpired:          pkg.Status == "active" && pkg.ExpiresAt.Before(time.Now()),
			IsDownloadLimitHit: pkg.MaxDownloads.Valid && minAccess[pkg.ID] >= pkg.MaxDownloads.Int64,
			Status:             pkg.Status,
			CreatedAt:          pkg.CreatedAt.Format("2006-01-02T15:04:05Z"),
			PendingRequests:    asks,
		}
	}
	return items, nil
}

// RevokePackage revokes a package; creator only. A soft status flip, not a
// delete, so it still shows as "revoked" in the creator's list.
func (h *Handlers) RevokePackage(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	packageID := r.PathValue("id")

	pkg, err := h.queries.GetPackageByID(r.Context(), packageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, errPackageNotFound)
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
