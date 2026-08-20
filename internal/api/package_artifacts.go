package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	db "filebox/internal/db/gen"
	"filebox/internal/objectstore"
)

// GetPackageArtifact downloads one recipient-visible item: either an original
// direct file or a generated ZIP. The package must be completely ready before
// any artifact is exposed, preventing a hybrid package from being downloaded
// only partially while its archives are still building.
func (h *Handlers) GetPackageArtifact(w http.ResponseWriter, r *http.Request) {
	h.servePackageArtifact(w, r, r.PathValue("id"))
}

// servePackageArtifact is also used by mapped legacy share URLs. Keeping the
// access claim in this one path ensures one request consumes exactly one
// artifact download, regardless of which URL form the recipient followed.
func (h *Handlers) servePackageArtifact(w http.ResponseWriter, r *http.Request, artifactID string) {
	// net/http's "GET /..." patterns also match HEAD. A metadata probe must not
	// consume a download allowance, and a presigned S3 GET cannot safely be
	// followed as HEAD, so make the supported method explicit.
	if r.Method == http.MethodHead {
		w.Header().Set("Allow", http.MethodGet)
		writeJSONError(w, http.StatusMethodNotAllowed, "use GET to download this item")
		return
	}
	artifact, err := h.queries.GetPackageArtifact(r.Context(), artifactID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "download not found")
		return
	}
	pkg, err := h.queries.GetPackageByID(r.Context(), artifact.PackageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "download not found")
		return
	}
	if pkg.PreparationStatus == "processing" {
		writeJSONError(w, http.StatusConflict, "package downloads are still being prepared")
		return
	}
	if pkg.PreparationStatus == "failed" || artifact.Status != "ready" {
		writeJSONError(w, http.StatusGone, "package downloads could not be prepared")
		return
	}
	if reason := unavailableReason(pkg, &artifact.AccessCount); reason != "" {
		writeJSONError(w, http.StatusGone, unavailableMessage(reason))
		return
	}
	if !h.packageVerified(r, pkg) {
		writeJSONError(w, http.StatusForbidden, "verification required")
		return
	}

	// Resolve the bytes before consuming the artifact's download budget.
	var signedURL, filePath string
	if artifact.Kind == string(PackageArchiveArtifactZIP) {
		if h.store != nil && artifact.ObjectKey.Valid {
			signedURL, err = h.store.PresignDownload(r.Context(), artifact.ObjectKey.String, artifact.Filename, presignTTL)
		} else {
			filePath = filepath.Join(h.uploadDir, ".archives", pkg.ID, artifact.ID+".zip")
		}
	} else {
		members, memberErr := h.queries.ListPackageArtifactMembersWithUploads(r.Context(), artifact.ID)
		if memberErr != nil || len(members) != 1 {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}
		member := members[0]
		if h.store != nil && member.UploadTargetName.Valid && member.UploadTargetName.String == objectstore.TargetName {
			signedURL, err = h.store.PresignDownload(
				r.Context(),
				h.store.Key(member.UploadID, member.UploadFilename),
				artifact.Filename,
				presignTTL,
			)
		} else {
			targetDir := filepath.Join(h.uploadDir, "RawMaterial")
			if member.UploadTargetName.Valid && member.UploadTargetName.String != "" {
				if target, targetErr := h.queries.GetTargetByName(r.Context(), member.UploadTargetName.String); targetErr == nil {
					targetDir = target.Path
				}
			}
			filePath, err = safeLocalUploadPath(targetDir, member.UploadFilename)
		}
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to prepare download")
		return
	}
	if filePath != "" {
		if _, err := os.Stat(filePath); err != nil {
			writeJSONError(w, http.StatusNotFound, "file not found on disk")
			return
		}
	}

	tx, q, err := h.queries.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}
	defer tx.Rollback()
	if pkg.MaxDownloads.Valid {
		_, err = q.IncrementPackageArtifactAccessCountIfUnderLimit(r.Context(), db.IncrementPackageArtifactAccessCountIfUnderLimitParams{
			ID:          artifact.ID,
			AccessCount: pkg.MaxDownloads.Int64,
		})
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusGone, "download limit reached")
			return
		}
	} else {
		_, err = q.IncrementPackageArtifactAccessCount(r.Context(), artifact.ID)
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}
	if _, err := q.IncrementArtifactMemberShareAccessCounts(r.Context(), artifact.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}
	if _, err := q.IncrementPackageDownloadCount(r.Context(), pkg.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to record access")
		return
	}

	h.recordDownloadItem(r, pkg, artifact.Filename, artifact.Size)
	if signedURL != "" {
		http.Redirect(w, r, signedURL, http.StatusFound)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifact.Filename))
	http.ServeFile(w, r, filePath)
}

// safeLocalUploadPath protects old database rows created before filename
// sanitisation existed. Current uploads are basenames already, but no persisted
// value should ever be able to turn a download/archive read into path traversal.
func safeLocalUploadPath(targetDir, filename string) (string, error) {
	if filename == "" || filename == "." || filename == ".." ||
		strings.ContainsAny(filename, "/\\\x00") || filepath.IsAbs(filename) || filepath.VolumeName(filename) != "" {
		return "", errors.New("unsafe stored filename")
	}
	return filepath.Join(targetDir, filename), nil
}
