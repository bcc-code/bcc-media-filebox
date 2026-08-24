package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
)

const (
	defaultSharesPageSize  = 50
	maxSharesPageSize      = 100
	maxShareExpiryDays     = 30
	maxPackageLifetimeDays = 90
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
	PackageID         string `json:"packageId"`
	PackageURL        string `json:"packageUrl"`
	ExpiresAt         string `json:"expiresAt"`
	PreparationStatus string `json:"preparationStatus"`
	ArtifactCount     int    `json:"artifactCount"`
}

// CreatePackage bundles already-uploaded files into one shareable package
// ("Send"). Shares retain the original-file manifest; package_artifacts are the
// actual recipient downloads (direct files and/or generated ZIPs).
func (h *Handlers) CreatePackage(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}
	// Send is reserved for fully authenticated identities — a guest session is
	// just a self-asserted name and email, which is not enough to put outbound
	// share links (and email) under someone's name.
	if caller.IsGuest() {
		writeJSONError(w, http.StatusForbidden, "guest accounts cannot send packages")
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

	// Recipients are validated before anything is created: a typo silently
	// dropped here means that person gets no mail and, since only recipients may
	// ask for a dead package back, no way to ask for it either.
	recipientEmails := make([]string, 0, len(req.Recipients))
	for _, raw := range req.Recipients {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		email, ok := canonicalEmail(raw)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("%q is not a valid email address", strings.TrimSpace(raw)))
			return
		}
		if !containsEmailFold(recipientEmails, email) {
			recipientEmails = append(recipientEmails, email)
		}
	}

	// Check every upload first, so a package is never partly created against a
	// file the caller can't share.
	uploads := make([]db.Upload, 0, len(req.UploadIDs))
	sources := make([]PackageArchiveSource, 0, len(req.UploadIDs))
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
		// The TUS completion notification is handled before its HTTP response is
		// returned, so a legitimate client only reaches this endpoint after this
		// state flip. Storage promotion may still be pending; that is deliberately
		// handled by the package worker and exposed through preparation progress.
		if upload.IsPartial != 0 {
			writeJSONError(w, http.StatusConflict, fmt.Sprintf("upload %q is only one part of a file", uploadID))
			return
		}
		if upload.Status != "completed" {
			writeJSONError(w, http.StatusConflict, fmt.Sprintf("upload %q has not finished", uploadID))
			return
		}
		uploads = append(uploads, upload)
		sources = append(sources, PackageArchiveSource{ID: upload.ID, Filename: upload.Filename, Size: upload.Size})
	}

	plan, err := planPackageArchives(sources)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "files cannot be packaged: "+err.Error())
		return
	}

	packageID, err := generateShareID()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to generate package id")
		return
	}

	expTime := time.Now().AddDate(0, 0, req.ExpiresInDays)

	// The capability behind the download report's opt-out link. Minted for every
	// package, not only the ones notifying today, so the link still works if the
	// author turns notifications back on later.
	muteTokenValue, err := generateShareID()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to generate mute token")
		return
	}
	muteToken := sql.NullString{String: muteTokenValue, Valid: true}
	tx, q, err := h.queries.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to start package creation")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	pkg, err := q.CreatePackage(r.Context(), db.CreatePackageParams{
		ID:                 packageID,
		CreatedByUserID:    caller.UserID,
		Name:               req.Name,
		Message:            req.Message,
		VerificationMethod: req.VerificationMethod,
		PasswordHash:       passwordHash,
		ExpiresAt:          expTime,
		MaxDownloads:       maxDownloads,
		NotifyOnDownload:   boolToInt64(req.NotifyOnDownload),
		NotifyMuteToken:    muteToken,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create package")
		return
	}

	shareByUploadID := make(map[string]string, len(uploads))
	for _, upload := range uploads {
		shareID, err := generateShareID()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to generate share id")
			return
		}
		// No expiry or limit here: source-manifest shares carry no policy of their
		// own. Recipient-visible artifacts enforce the package policy.
		if _, err := q.CreateShare(r.Context(), db.CreateShareParams{
			ID:        shareID,
			PackageID: packageID,
			UploadID:  upload.ID,
		}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to attach file to package")
			return
		}
		shareByUploadID[upload.ID] = shareID
	}

	var preparationBytesTotal int64
	for _, artifact := range plan.Artifacts {
		preparationBytesTotal += artifact.PayloadSize
	}
	zipCount := 0
	for _, artifact := range plan.Artifacts {
		if artifact.Kind == PackageArchiveArtifactZIP {
			zipCount++
		}
	}
	zipIndex := 0
	allReady := true
	var initiallyPreparedBytes int64
	for position, planned := range plan.Artifacts {
		artifactID, err := generateShareID()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to generate download id")
			return
		}

		filename := planned.Members[0].Source.Filename
		status := "ready"
		size := planned.PayloadSize
		progress := planned.PayloadSize
		objectKey := sql.NullString{}
		if planned.Kind == PackageArchiveArtifactZIP {
			zipIndex++
			filename = packageArchiveFilename(pkg.Name, zipIndex, zipCount)
			status = "pending"
			size = 0
			progress = 0
			if h.store != nil {
				objectKey = sql.NullString{String: h.store.PackageArtifactKey(pkg.ID, artifactID), Valid: true}
			}
		} else {
			upload, ok := uploadByID(uploads, planned.Members[0].Source.ID)
			if !ok || upload.StorageStatus != "ready" {
				status = "pending"
				progress = 0
			}
		}
		if status != "ready" {
			allReady = false
		} else {
			initiallyPreparedBytes += planned.PayloadSize
		}

		if _, err := q.CreatePackageArtifact(r.Context(), db.CreatePackageArtifactParams{
			ID:            artifactID,
			PackageID:     pkg.ID,
			Kind:          string(planned.Kind),
			Filename:      filename,
			Size:          size,
			SourceSize:    planned.PayloadSize,
			Position:      int64(position),
			Status:        status,
			ProgressBytes: progress,
			ObjectKey:     objectKey,
		}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to create package download")
			return
		}
		for memberPosition, member := range planned.Members {
			shareID := shareByUploadID[member.Source.ID]
			if _, err := q.CreatePackageArtifactMember(r.Context(), db.CreatePackageArtifactMemberParams{
				ArtifactID:      artifactID,
				ShareID:         shareID,
				Position:        int64(memberPosition),
				ArchiveFilename: member.ArchiveName,
			}); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "failed to create package download")
				return
			}
		}
	}

	recipients := make([]db.PackageRecipient, 0, len(recipientEmails))
	for _, email := range recipientEmails {
		rcpt, err := q.CreatePackageRecipient(r.Context(), db.CreatePackageRecipientParams{
			PackageID: packageID,
			Email:     email,
		})
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to add recipient")
			return
		}
		recipients = append(recipients, rcpt)
	}

	// Publish the durable job only after its complete plan and recipient list
	// exist. The worker scans processing packages, so this ordering prevents it
	// from claiming a half-created package between individual SQLite writes.
	pkg, err = q.SetPackagePreparationProcessing(r.Context(), db.SetPackagePreparationProcessingParams{
		PreparationBytesTotal: preparationBytesTotal,
		ID:                    pkg.ID,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to initialise package preparation")
		return
	}
	if allReady {
		pkg, err = q.FinalizePackagePreparation(r.Context(), pkg.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to finish package preparation")
			return
		}
	} else if initiallyPreparedBytes > 0 {
		pkg, err = q.UpdatePackagePreparationProgress(r.Context(), db.UpdatePackagePreparationProgressParams{
			PreparationBytesDone: initiallyPreparedBytes,
			ID:                   pkg.ID,
		})
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to record package preparation progress")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to commit package creation")
		return
	}
	committed = true

	// A package that only contains direct, storage-ready files can be delivered
	// immediately. ZIP packages are mailed by the preparation worker only after
	// every artifact is atomically ready, so recipients never land on a partial
	// download set.
	if pkg.PreparationStatus == "ready" {
		go h.notifyRecipients(pkg, recipients, uploads, caller)
	} else {
		h.wakePackagePreparation()
	}

	writeJSON(w, http.StatusCreated, CreatePackageResponse{
		PackageID:         pkg.ID,
		PackageURL:        packageID,
		ExpiresAt:         expTime.UTC().Format("2006-01-02T15:04:05Z"),
		PreparationStatus: pkg.PreparationStatus,
		ArtifactCount:     len(plan.Artifacts),
	})
}

type PackageListItem struct {
	PackageID             string              `json:"packageId"`
	Name                  string              `json:"name"`
	Message               string              `json:"message"`
	VerificationMethod    string              `json:"verificationMethod"`
	Recipients            []string            `json:"recipients"`
	FileCount             int                 `json:"fileCount"`
	TotalSize             int64               `json:"totalSize"`
	DownloadCount         int64               `json:"downloadCount"`
	MaxDownloads          *int64              `json:"maxDownloads"`
	ExpiresAt             string              `json:"expiresAt"`
	IsExpired             bool                `json:"isExpired"`
	IsDownloadLimitHit    bool                `json:"isDownloadLimitHit"`
	Status                string              `json:"status"`
	CreatedAt             string              `json:"createdAt"`
	PermanentlyExpired    bool                `json:"permanentlyExpired"`
	FilesDeletedAt        string              `json:"filesDeletedAt"`
	NotifyOnDownload      bool                `json:"notifyOnDownload"`
	PreparationStatus     string              `json:"preparationStatus"`
	PreparationBytesDone  int64               `json:"preparationBytesDone"`
	PreparationBytesTotal int64               `json:"preparationBytesTotal"`
	PreparationProgress   int                 `json:"preparationProgress"`
	PreparationError      string              `json:"preparationError,omitempty"`
	ArtifactCount         int                 `json:"artifactCount"`
	PendingRequests       []AccessRequestView `json:"pendingRequests"`
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
	artifactCount := make(map[string]int64, len(pkgs))
	artifactCounts, err := h.queries.GetPackageArtifactAccessCountsByPackageIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("compute package artifact downloads: %w", err)
	}
	for _, c := range artifactCounts {
		artifactCount[c.PackageID] = c.ArtifactCount
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
		filesDeletedAt := pkg.CreatedAt.AddDate(0, 0, maxPackageLifetimeDays)
		items[i] = PackageListItem{
			PackageID:             pkg.ID,
			Name:                  pkg.Name,
			Message:               pkg.Message,
			VerificationMethod:    pkg.VerificationMethod,
			Recipients:            recs,
			FileCount:             fileCount[pkg.ID],
			TotalSize:             totalSize[pkg.ID],
			DownloadCount:         maxAccess[pkg.ID],
			MaxDownloads:          maxDownloads,
			ExpiresAt:             pkg.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
			IsExpired:             pkg.Status == "active" && pkg.ExpiresAt.Before(time.Now()),
			IsDownloadLimitHit:    pkg.MaxDownloads.Valid && minAccess[pkg.ID] >= pkg.MaxDownloads.Int64,
			NotifyOnDownload:      pkg.NotifyOnDownload != 0,
			PreparationStatus:     pkg.PreparationStatus,
			PreparationBytesDone:  pkg.PreparationBytesDone,
			PreparationBytesTotal: pkg.PreparationBytesTotal,
			PreparationProgress:   preparationProgress(pkg),
			PreparationError:      pkg.PreparationError.String,
			ArtifactCount:         int(artifactCount[pkg.ID]),
			Status:                pkg.Status,
			CreatedAt:             pkg.CreatedAt.Format("2006-01-02T15:04:05Z"),
			PendingRequests:       asks,
			PermanentlyExpired:    isPermanentlyExpired(pkg),
			FilesDeletedAt:        filesDeletedAt.UTC().Format("2006-01-02T15:04:05Z"),
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
