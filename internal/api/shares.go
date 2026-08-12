package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// GetShare retrieves a share by ID and serves the underlying file. A share
// carries no policy of its own — expiry, download limits, and verification
// all live on its parent package, since that's the only place the UI ever
// lets someone set them.
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

	// max_downloads is a per-file budget, not a shared pool: it's configured
	// once on the package (the UI only ever exposes one field), but checked
	// against this specific share's own access_count. Otherwise downloading
	// any N files across a multi-file package would permanently lock out
	// every other file once the package-wide total hit the limit, even ones
	// never touched.
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

	// Two counters: the share's own access_count (used for the per-file gate
	// above) and the package's download_count (an aggregate shown to the
	// owner, e.g. "2/10 downloads" — display-only, not used for gating).
	if _, err := h.queries.IncrementShareAccessCount(r.Context(), shareID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}
	if _, err := h.queries.IncrementPackageDownloadCount(r.Context(), share.PackageID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
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
