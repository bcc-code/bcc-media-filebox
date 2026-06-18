package tus

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	db "filebox/internal/db/gen"
	"filebox/internal/forms"
	"filebox/internal/webhook"

	"github.com/tus/tusd/v2/pkg/handler"
)

type EventProcessor struct {
	queries   *db.Queries
	uploadDir string
	tempDir   string
}

func NewEventProcessor(queries *db.Queries, uploadDir, tempDir string) *EventProcessor {
	return &EventProcessor{queries: queries, uploadDir: uploadDir, tempDir: tempDir}
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
	targetName := info.MetaData["target"]
	formData := info.MetaData["formdata"]

	err := ep.queries.CreateUpload(context.Background(), db.CreateUploadParams{
		ID:       info.ID,
		UserID:   userID,
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
		TargetName: sql.NullString{
			String: targetName,
			Valid:  targetName != "",
		},
		FormData: sql.NullString{
			String: formData,
			Valid:  formData != "",
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
	// Resolve the target row from the DB — targets can be added/edited by admins
	// at runtime, so this can't be cached at startup. When the target is bound to
	// a hardcoded form, the final filename is derived from the submitted form
	// data rather than the client-supplied name.
	targetName := info.MetaData["target"]
	targetDir := filepath.Join(ep.uploadDir, "RawMaterial")
	var form forms.Form
	hasForm := false
	var formValues map[string]string
	var webhookURL string
	if t, err := ep.queries.GetTargetByName(context.Background(), targetName); err == nil {
		targetDir = t.Path
		webhookURL = t.WebhookUrl.String
		if t.FormKey.Valid && t.FormKey.String != "" {
			if f, ok := forms.Get(t.FormKey.String); ok {
				form = f
				hasForm = true
				formValues = parseFormData(info.MetaData["formdata"])
			}
		}
	}

	// Rename the file from hash ID to its final name in the target directory.
	// For form targets the name comes from forms.BuildFilename; otherwise the
	// client-supplied filename is used. Defense in depth: re-sanitize here so any
	// future code path that bypasses the create callback still can't escape
	// targetDir.
	rawFilename := info.MetaData["filename"]
	var dstPath string
	desiredName := rawFilename
	if hasForm {
		desiredName = forms.BuildFilename(form, formValues, filepath.Ext(rawFilename))
	}
	if desiredName != "" {
		filename, err := SanitizeFilename(desiredName)
		if err != nil {
			log.Printf("rejecting upload %s: %v", info.ID, err)
			ep.queries.FailUpload(context.Background(), info.ID)
		} else {
			dstPath = ep.renameUpload(info.ID, filename, targetDir)
		}
	}

	// Record the final on-disk name (form-derived and/or de-duped) so the upload
	// history reflects what actually landed in the target dir, not the original
	// client filename captured at create time.
	if dstPath != "" {
		if err := ep.queries.UpdateUploadFilename(context.Background(), db.UpdateUploadFilenameParams{
			Filename: filepath.Base(dstPath),
			ID:       info.ID,
		}); err != nil {
			log.Printf("warning: failed to update stored filename for %s: %v", info.ID, err)
		}
	}

	// Verify file integrity against the client-provided SHA-256 hash
	integrityFailed := false
	expectedHash := info.MetaData["sha256"]
	if expectedHash != "" && dstPath != "" {
		actualHash, err := computeFileSHA256(dstPath)
		if err != nil {
			log.Printf("error computing SHA-256 for %s: %v", dstPath, err)
		} else if actualHash != expectedHash {
			log.Printf("integrity check FAILED for upload %s (%s): expected %s, got %s", info.ID, dstPath, expectedHash, actualHash)
			ep.queries.FailUpload(context.Background(), info.ID)
			integrityFailed = true
		} else {
			log.Printf("integrity verified for %s (SHA-256: %s)", dstPath, actualHash)
		}
	}

	// Write the JSON sidecar next to the file for form uploads. Derived from the
	// final (possibly de-duped) destination name. Skipped on integrity failure so
	// we never leave metadata describing a rejected file; a sidecar write failure
	// is logged but does not fail the upload — the file is the primary artifact.
	if hasForm && dstPath != "" && !integrityFailed {
		uploaderID := info.MetaData["userid"]
		payload := sidecarPayload{
			OriginalFilename: rawFilename,
			Filename:         filepath.Base(dstPath),
			Target:           targetName,
			FormKey:          form.Key,
			Fields:           formValues,
			UploaderID:       uploaderID,
			UploaderEmail:    ep.uploaderEmail(uploaderID),
			SHA256:           expectedHash,
			UploadedAt:       completedAt.UTC().Format(time.RFC3339),
		}
		if err := ep.writeSidecar(dstPath, targetDir, payload); err != nil {
			log.Printf("warning: failed to write sidecar for %s: %v", dstPath, err)
		} else if webhookURL != "" {
			// Notify the target's webhook that a new file (and sidecar) landed.
			// Fire-and-forget: like the sidecar write, this is best-effort — the
			// file is the primary artifact and a slow or failing receiver must
			// not stall finalization.
			sidecarPath := dstPath + ".json"
			ep.fireWebhook(webhookURL, filepath.Base(sidecarPath), ep.relPath(sidecarPath))
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
		if !errors.Is(err, syscall.EXDEV) {
			log.Printf("error renaming upload %s to %s: %v", id, dst, err)
			return ""
		}
		log.Printf("cross-device copy: %s -> %s", src, dst)
		if err := crossDeviceMove(src, dst); err != nil {
			log.Printf("error moving upload %s to %s across filesystems: %v", id, dst, err)
			return ""
		}
	}
	log.Printf("upload saved: %s", dst)
	return dst
}

// crossDeviceMove copies src to dst when os.Rename fails with EXDEV. To keep
// the destination atomically visible, bytes are written to a sibling
// "<dst>.part" file (same filesystem as dst, so the closing rename is atomic),
// then renamed into place. The source is removed only after the destination is
// safely published; on any error the partial sibling is cleaned up and src is
// left in place so the upload can be retried.
func crossDeviceMove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	part := dst + ".part"
	out, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("create partial: %w", err)
	}
	cleanup := func() { _ = os.Remove(part) }

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		cleanup()
		return fmt.Errorf("copy: %w", err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		cleanup()
		return fmt.Errorf("sync: %w", err)
	}
	if err := out.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close partial: %w", err)
	}
	if err := os.Rename(part, dst); err != nil {
		cleanup()
		return fmt.Errorf("rename partial: %w", err)
	}
	if err := os.Remove(src); err != nil {
		log.Printf("warning: failed to remove source %s after cross-device move: %v", src, err)
	}
	return nil
}

// sidecarPayload is the JSON written next to a form upload, recording the
// submitted field values and provenance.
type sidecarPayload struct {
	OriginalFilename string            `json:"originalFilename"`
	Filename         string            `json:"filename"`
	Target           string            `json:"target"`
	FormKey          string            `json:"formKey"`
	Fields           map[string]string `json:"fields"`
	UploaderID       string            `json:"uploaderId"`
	UploaderEmail    string            `json:"uploaderEmail,omitempty"`
	SHA256           string            `json:"sha256,omitempty"`
	UploadedAt       string            `json:"uploadedAt"`
}

// uploaderEmail resolves the email for a canonical user_id ("<provider>:<subject>",
// e.g. "guest:<ulid>") by looking up the user record. Returns "" when the id is
// malformed, the user is unknown, or the email is unset — the sidecar omits an
// empty email rather than failing the upload.
func (ep *EventProcessor) uploaderEmail(userID string) string {
	provider, subject, ok := strings.Cut(userID, ":")
	if !ok {
		return ""
	}
	u, err := ep.queries.GetUserByProviderSubject(context.Background(), db.GetUserByProviderSubjectParams{
		Provider: provider,
		Subject:  subject,
	})
	if err != nil {
		return ""
	}
	return u.Email.String
}

// parseFormData decodes the TUS "formdata" metadata JSON into a value map,
// returning an empty (non-nil) map when absent or malformed.
func parseFormData(raw string) map[string]string {
	values := map[string]string{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			log.Printf("warning: ignoring malformed form data: %v", err)
		}
	}
	return values
}

// writeSidecar writes payload as "<dstPath>.json". Reuses the same containment
// check as renameUpload so the sidecar can never escape targetDir.
func (ep *EventProcessor) writeSidecar(dstPath, targetDir string, payload sidecarPayload) error {
	sidecarPath := dstPath + ".json"

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolve target dir: %w", err)
	}
	absSidecar, err := filepath.Abs(sidecarPath)
	if err != nil {
		return fmt.Errorf("resolve sidecar path: %w", err)
	}
	rel, err := filepath.Rel(absTarget, absSidecar)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("refusing to write sidecar outside target dir: %s", absSidecar)
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sidecar: %w", err)
	}
	if err := os.WriteFile(sidecarPath, data, 0644); err != nil {
		return fmt.Errorf("write sidecar: %w", err)
	}
	log.Printf("sidecar written: %s", sidecarPath)
	return nil
}

// relPath returns path relative to the upload root, used as the webhook "path"
// field (e.g. "RawMaterial/ARR_SUB_NAME.mov.json"). Falls back to the absolute
// path when the target dir lives outside the upload root.
func (ep *EventProcessor) relPath(path string) string {
	rel, err := filepath.Rel(ep.uploadDir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if abs, aerr := filepath.Abs(path); aerr == nil {
			return abs
		}
		return path
	}
	return rel
}

// fireWebhook sends the upload notification in a background goroutine so a slow
// receiver never blocks finalization. Success and failure are logged.
func (ep *EventProcessor) fireWebhook(url, sidecarName, relPath string) {
	go func() {
		if err := webhook.Send(context.Background(), url, webhook.Payload{Sidecar: sidecarName, Path: relPath}); err != nil {
			log.Printf("warning: webhook to %s failed for %s: %v", url, sidecarName, err)
			return
		}
		log.Printf("webhook delivered to %s for %s", url, sidecarName)
	}()
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

// SanitizeFilename returns a filename containing only [A-Za-z0-9_-] plus an
// optional single '.' separating the extension. Every other rune (including
// internal dots) is replaced with '_'. A leading '.' is never treated as an
// extension separator — it is also replaced with '_' so files don't end up
// hidden on POSIX filesystems. Returns an error for "", ".", and ".." since
// those can't be safely represented even after substitution.
func SanitizeFilename(name string) (string, error) {
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid filename %q", name)
	}
	lastDot := strings.LastIndex(name, ".")
	if lastDot == 0 {
		// The only dot is the leading one; not an extension separator.
		lastDot = -1
	}
	var b strings.Builder
	b.Grow(len(name))
	for i, r := range name {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
			b.WriteRune(r)
		case r == '.' && i == lastDot:
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String(), nil
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
