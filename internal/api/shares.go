package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	db "filebox/internal/db/gen"
	"filebox/internal/objectstore"
)

// presignTTL bounds how long an S3 download URL stays valid, and so how long it
// can be forwarded or re-fetched uncounted. It needn't cover the transfer (S3
// checks the signature only at request start), but lower breaks paused resumes.
const presignTTL = 5 * time.Minute

// GetShare serves a share's underlying file. Expiry, download limits, and
// verification all live on the parent package, not here.
func (h *Handlers) GetShare(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("id")

	share, err := h.queries.GetShareByID(r.Context(), shareID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "share not found")
		return
	}

	pkg, err := h.queries.GetPackageByID(r.Context(), share.PackageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "share not found")
		return
	}

	if pkg.Status != "active" {
		writeJSONError(w, http.StatusGone, "package has been revoked")
		return
	}

	if !pkg.ExpiresAt.After(time.Now()) {
		writeJSONError(w, http.StatusGone, "package has expired")
		return
	}

	// max_downloads is a per-file budget, not a shared pool: set once on the
	// package but checked against this share's own count. Summing instead would
	// let a few files lock out every other file in the package.
	if pkg.MaxDownloads.Valid && share.AccessCount >= pkg.MaxDownloads.Int64 {
		writeJSONError(w, http.StatusGone, "file download limit reached")
		return
	}

	if !h.packageVerified(r, pkg) {
		writeJSONError(w, http.StatusForbidden, "verification required")
		return
	}

	upload, err := h.queries.GetUpload(r.Context(), share.UploadID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "file not found")
		return
	}

	// Locate the bytes before recording the access, so a missing file doesn't
	// burn a download.
	var signedURL, filePath string
	if h.store != nil && upload.TargetName.String == objectstore.TargetName {
		signedURL, err = h.store.PresignDownload(
			r.Context(),
			h.store.Key(upload.ID, upload.Filename),
			upload.Filename,
			presignTTL,
		)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to prepare download")
			return
		}
	} else {
		// Mirrors storeToDisk in internal/tus: the target's configured path, or
		// uploadDir/RawMaterial when there's no target.
		targetDir := filepath.Join(h.uploadDir, "RawMaterial")
		if upload.TargetName.Valid && upload.TargetName.String != "" {
			if target, err := h.queries.GetTargetByName(r.Context(), upload.TargetName.String); err == nil {
				targetDir = target.Path
			}
		}
		filePath = filepath.Join(targetDir, upload.Filename)

		if _, err := os.Stat(filePath); err != nil {
			writeJSONError(w, http.StatusNotFound, "file not found on disk")
			return
		}
	}

	// Two counters: the share's access_count (the gate) and the package's
	// download_count (display only). Re-testing the limit inside the UPDATE is
	// the authoritative check — the read above is just a fast path, and
	// "Download all" fires every file at once, so races are routine.
	if pkg.MaxDownloads.Valid {
		_, err := h.queries.IncrementShareAccessCountIfUnderLimit(r.Context(), db.IncrementShareAccessCountIfUnderLimitParams{
			ID:          shareID,
			AccessCount: pkg.MaxDownloads.Int64,
		})
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusGone, "file download limit reached")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to record access")
			return
		}
	} else if _, err := h.queries.IncrementShareAccessCount(r.Context(), shareID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}
	if _, err := h.queries.IncrementPackageDownloadCount(r.Context(), share.PackageID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}

	h.recordDownload(r, pkg, upload)

	// Handed off to S3, so the traffic never transits this server. The signed URL
	// carries its own Content-Disposition.
	if signedURL != "" {
		http.Redirect(w, r, signedURL, http.StatusFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", upload.Filename))
	http.ServeFile(w, r, filePath)
}

func generateShareID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
