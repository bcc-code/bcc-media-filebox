package tus

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"file-pusher/internal/config"
	db "file-pusher/internal/db/gen"

	"github.com/tus/tusd/v2/pkg/handler"
)

type EventProcessor struct {
	queries   *db.Queries
	uploadDir string
	tempDir   string
	targets   []config.Target
}

func NewEventProcessor(queries *db.Queries, uploadDir, tempDir string, targets []config.Target) *EventProcessor {
	return &EventProcessor{queries: queries, uploadDir: uploadDir, tempDir: tempDir, targets: targets}
}

// Run processes all tus events in a single goroutine to avoid race conditions.
// With concatenation, CreatedUploads and CompleteUploads for the final upload
// fire within the same HTTP request — separate goroutines can process them
// out of order. A single select loop guarantees create-before-complete.
func (ep *EventProcessor) Run(h *handler.UnroutedHandler) {
	for {
		select {
		case event, ok := <-h.CreatedUploads:
			if !ok {
				return
			}
			ep.handleCreated(event)
		case event, ok := <-h.UploadProgress:
			if !ok {
				return
			}
			ep.handleProgress(event)
		case event, ok := <-h.CompleteUploads:
			if !ok {
				return
			}
			ep.handleComplete(event)
		case event, ok := <-h.TerminatedUploads:
			if !ok {
				return
			}
			ep.handleTerminated(event)
		}
	}
}

func (ep *EventProcessor) handleCreated(event handler.HookEvent) {
	info := event.Upload
	isPartial := int64(0)
	if info.IsPartial {
		isPartial = 1
	}

	filename := info.MetaData["filename"]
	contentType := info.MetaData["filetype"]
	userID := info.MetaData["userid"]
	sha256Hash := info.MetaData["sha256"]

	err := ep.queries.CreateUpload(context.Background(), db.CreateUploadParams{
		ID:     info.ID,
		UserID: userID,
		Filename: filename,
		Size:     info.Size,
		ContentType: sql.NullString{
			String: contentType,
			Valid:  contentType != "",
		},
		IsPartial:     isPartial,
		FinalUploadID: sql.NullString{},
		Sha256: sql.NullString{
			String: sha256Hash,
			Valid:  sha256Hash != "",
		},
	})
	if err != nil {
		log.Printf("error creating upload record: %v", err)
	}
}

func (ep *EventProcessor) handleProgress(event handler.HookEvent) {
	info := event.Upload
	err := ep.queries.UpdateUploadOffset(context.Background(), db.UpdateUploadOffsetParams{
		Offset: info.Offset,
		ID:     info.ID,
	})
	if err != nil {
		log.Printf("error updating upload offset: %v", err)
	}
}

func (ep *EventProcessor) handleComplete(event handler.HookEvent) {
	info := event.Upload

	// Capture completion time before any post-processing (rename, hash
	// verification) so that bandwidth calculations reflect only the
	// transfer, not the assembly overhead.
	completedAt := time.Now()

	// Mark as completed in DB
	err := ep.queries.CompleteUpload(context.Background(), info.ID)
	if err != nil {
		log.Printf("error completing upload: %v", err)
	}

	// Skip file operations for partial uploads — they'll be cleaned up
	// when the final concatenated upload completes.
	if info.IsPartial {
		return
	}

	// Run post-upload work (rename, hash verification, cleanup) in a
	// separate goroutine so we don't block the event loop — hashing a
	// large file can take minutes and would stall all other uploads.
	go ep.finalizeUpload(info, completedAt)
}

func (ep *EventProcessor) finalizeUpload(info handler.FileInfo, completedAt time.Time) {
	// Resolve target directory
	targetName := info.MetaData["target"]
	targetDir, ok := config.TargetDir(ep.targets, targetName)
	if !ok {
		targetDir = filepath.Join(ep.uploadDir, "RawMaterial")
	}

	// Rename the file from hash ID to original filename in the target directory.
	// Defense in depth: the PreUploadCreateCallback already rejects hostile
	// filenames at create time, but re-sanitize here so any future code path
	// that bypasses the callback still can't escape targetDir.
	rawFilename := info.MetaData["filename"]
	var dstPath string
	if rawFilename != "" {
		filename, err := SanitizeFilename(rawFilename)
		if err != nil {
			log.Printf("rejecting upload %s: %v", info.ID, err)
			ep.queries.FailUpload(context.Background(), info.ID)
		} else {
			dstPath = ep.renameUpload(info.ID, filename, targetDir)
		}
	}

	// Verify file integrity against the client-provided SHA-256 hash
	expectedHash := info.MetaData["sha256"]
	if expectedHash != "" && dstPath != "" {
		actualHash, err := computeFileSHA256(dstPath)
		if err != nil {
			log.Printf("error computing SHA-256 for %s: %v", dstPath, err)
		} else if actualHash != expectedHash {
			log.Printf("integrity check FAILED for upload %s (%s): expected %s, got %s", info.ID, dstPath, expectedHash, actualHash)
			ep.queries.FailUpload(context.Background(), info.ID)
		} else {
			log.Printf("integrity verified for %s (SHA-256: %s)", dstPath, actualHash)
		}
	}

	// For concatenated uploads, fix the duration to measure from the earliest
	// partial upload's creation time (the final upload is created and completed
	// in the same request, so its created_at == completed_at).
	if info.PartialUploads != nil {
		var earliest time.Time
		for _, partialID := range info.PartialUploads {
			p, err := ep.queries.GetUpload(context.Background(), partialID)
			if err != nil {
				continue
			}
			if earliest.IsZero() || p.CreatedAt.Before(earliest) {
				earliest = p.CreatedAt
			}
		}
		if !earliest.IsZero() {
			durationMs := completedAt.Sub(earliest).Milliseconds()
			ep.queries.UpdateDurationMs(context.Background(), db.UpdateDurationMsParams{
				DurationMs: sql.NullInt64{Int64: durationMs, Valid: true},
				ID:         info.ID,
			})
		}

		// Clean up partial files and .info files
		for _, partialID := range info.PartialUploads {
			os.Remove(filepath.Join(ep.tempDir, partialID))
			os.Remove(filepath.Join(ep.tempDir, partialID+".info"))
		}
		// Delete partial DB records
		for _, partialID := range info.PartialUploads {
			ep.queries.DeleteUpload(context.Background(), partialID)
		}
	}

	// Remove the .info file for the completed upload
	os.Remove(filepath.Join(ep.tempDir, info.ID+".info"))
}

func (ep *EventProcessor) handleTerminated(event handler.HookEvent) {
	info := event.Upload
	err := ep.queries.DeleteUpload(context.Background(), info.ID)
	if err != nil {
		log.Printf("error deleting upload record: %v", err)
	}
}

// renameUpload moves the uploaded file from its hash-based ID to the original filename
// inside targetDir. If a file with the same name exists, a numeric suffix is added.
// Returns the destination path on success, or empty string on failure.
func (ep *EventProcessor) renameUpload(id, filename, targetDir string) string {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Printf("error creating target directory %s: %v", targetDir, err)
		return ""
	}

	src := filepath.Join(ep.tempDir, id)
	dst := ep.uniquePath(targetDir, filename)

	// Defense in depth: verify the resolved destination is inside targetDir.
	// SanitizeFilename should already guarantee this, but a containment check
	// here catches future refactors and any path-sensitive edge cases.
	// Note: this does not follow symlinks — if targetDir itself ever contains
	// untrusted symlinks, add filepath.EvalSymlinks.
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		log.Printf("error resolving target dir %s: %v", targetDir, err)
		return ""
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		log.Printf("error resolving destination %s: %v", dst, err)
		return ""
	}
	rel, err := filepath.Rel(absTarget, absDst)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		log.Printf("refusing to write outside target dir: target=%s dst=%s", absTarget, absDst)
		return ""
	}

	if err := os.Rename(src, dst); err != nil {
		log.Printf("error renaming upload %s to %s: %v", id, dst, err)
		return ""
	}
	log.Printf("upload saved: %s", dst)
	return dst
}

func (ep *EventProcessor) uniquePath(dir, filename string) string {
	dst := filepath.Join(dir, filename)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return dst
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	for i := 1; ; i++ {
		dst = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return dst
		}
	}
}

// SanitizeFilename rejects filenames that could escape the target directory
// or otherwise confuse filesystem operations. It deliberately rejects rather
// than silently stripping path components — a name like "../../etc/passwd"
// is never legitimate, and reducing it to "passwd" would save the file under
// a name the user never chose.
func SanitizeFilename(name string) (string, error) {
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid filename %q", name)
	}
	if strings.ContainsRune(name, 0) {
		return "", errors.New("filename contains NUL byte")
	}
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\\') || strings.ContainsRune(name, filepath.Separator) {
		return "", fmt.Errorf("filename %q contains path separator", name)
	}
	return name, nil
}

func computeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
