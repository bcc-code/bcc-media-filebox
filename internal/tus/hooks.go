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
	"filebox/internal/objectstore"
	"filebox/internal/webhook"

	"github.com/tus/tusd/v2/pkg/handler"
)

type EventProcessor struct {
	queries   *db.Queries
	uploadDir string
	tempDir   string
	store     *objectstore.Client
}

// NewEventProcessor wires the upload event loop. A nil store means every upload
// finalizes locally; otherwise uploads tagged objectstore.TargetName go to S3.
func NewEventProcessor(queries *db.Queries, uploadDir, tempDir string, store *objectstore.Client) *EventProcessor {
	return &EventProcessor{queries: queries, uploadDir: uploadDir, tempDir: tempDir, store: store}
}

// RecoverPending resumes completed TUS uploads whose asynchronous promotion to
// local storage or S3 was interrupted by a process restart. If the temporary
// bytes still exist, normal finalization is replayed. If they do not, recovery
// checks whether the final object was already published before the process
// stopped and completes only the missing database bookkeeping.
func (ep *EventProcessor) RecoverPending(ctx context.Context) error {
	uploads, err := ep.queries.ListPendingStorageUploads(ctx)
	if err != nil {
		return fmt.Errorf("list pending storage uploads: %w", err)
	}

	var recoveryErrs []error
	for _, upload := range uploads {
		if err := ctx.Err(); err != nil {
			recoveryErrs = append(recoveryErrs, err)
			break
		}
		if err := ep.recoverPendingUpload(ctx, upload); err != nil {
			wrapped := fmt.Errorf("upload %s: %w", upload.ID, err)
			log.Printf("storage recovery: %v", wrapped)
			recoveryErrs = append(recoveryErrs, wrapped)
		}
	}
	return errors.Join(recoveryErrs...)
}

func (ep *EventProcessor) recoverPendingUpload(ctx context.Context, upload db.Upload) error {
	info := ep.recoveryFileInfo(upload)
	completedAt := upload.CreatedAt
	if upload.CompletedAt.Valid {
		completedAt = upload.CompletedAt.Time
	}

	tempPath := filepath.Join(ep.tempDir, upload.ID)
	stat, err := os.Stat(tempPath)
	switch {
	case err == nil && !stat.Mode().IsRegular():
		return ep.failRecovery(ctx, upload.ID, "temporary upload is not a regular file")
	case err == nil:
		// Run synchronously so startup recovery can verify the resulting state.
		ep.finalizeUpload(info, completedAt)
		updated, getErr := ep.queries.GetUpload(ctx, upload.ID)
		if getErr != nil {
			return fmt.Errorf("read state after finalization: %w", getErr)
		}
		if updated.StorageStatus != "ready" {
			return fmt.Errorf("finalization ended with storage status %q", updated.StorageStatus)
		}
		return nil
	case !os.IsNotExist(err):
		return fmt.Errorf("inspect temporary upload: %w", err)
	}

	// The source disappeared, which can be the normal crash window after the
	// final rename/upload but before MarkUploadStorageReady committed.
	if upload.TargetName.Valid && upload.TargetName.String == objectstore.TargetName {
		if ep.store == nil {
			// This can be fixed by restoring S3 configuration, so leave the row
			// pending for the next restart rather than turning it into data loss.
			return errors.New("S3 destination configured but object store is unavailable")
		}
		key := ep.store.Key(upload.ID, upload.Filename)
		exists, existsErr := ep.store.Exists(ctx, key)
		if existsErr != nil {
			return fmt.Errorf("check final S3 object: %w", existsErr)
		}
		if !exists {
			return ep.failRecovery(ctx, upload.ID, "temporary upload and final S3 object are missing")
		}
		if err := ep.queries.MarkUploadStorageReady(ctx, upload.ID); err != nil {
			return fmt.Errorf("mark recovered S3 upload ready: %w", err)
		}
		ep.cleanupFinalization(info, completedAt)
		log.Printf("storage recovery: upload %s was already present at s3://%s/%s", upload.ID, ep.store.Bucket(), key)
		return nil
	}

	finalPath, found, err := ep.findRecoveredLocalFile(ctx, info, upload)
	if err != nil {
		return err
	}
	if !found {
		return ep.failRecovery(ctx, upload.ID, "temporary upload and final local file are missing")
	}
	finalName := filepath.Base(finalPath)
	if _, err := ep.queries.FinalizeUploadStorage(ctx, db.FinalizeUploadStorageParams{
		Filename: finalName,
		ID:       upload.ID,
	}); err != nil {
		return fmt.Errorf("record recovered local upload: %w", err)
	}
	ep.cleanupFinalization(info, completedAt)
	log.Printf("storage recovery: upload %s was already present at %s", upload.ID, finalPath)
	return nil
}

// recoveryFileInfo prefers tusd's sidecar because it retains concatenation
// metadata, then overwrites the fields used for storage routing with the
// database row. The fallback makes recovery work even if tusd removed or did
// not finish writing the .info file.
func (ep *EventProcessor) recoveryFileInfo(upload db.Upload) handler.FileInfo {
	info := handler.FileInfo{}
	data, err := os.ReadFile(filepath.Join(ep.tempDir, upload.ID+".info"))
	if err == nil {
		if unmarshalErr := json.Unmarshal(data, &info); unmarshalErr != nil {
			log.Printf("storage recovery: ignoring invalid info file for %s: %v", upload.ID, unmarshalErr)
			info = handler.FileInfo{}
		} else if info.ID != "" && info.ID != upload.ID {
			log.Printf("storage recovery: ignoring mismatched info file for %s (contains %s)", upload.ID, info.ID)
			info = handler.FileInfo{}
		}
	} else if !os.IsNotExist(err) {
		log.Printf("storage recovery: cannot read info file for %s, using database metadata: %v", upload.ID, err)
	}

	if info.MetaData == nil {
		info.MetaData = handler.MetaData{}
	}
	info.ID = upload.ID
	info.Size = upload.Size
	info.Offset = upload.Size
	info.IsPartial = false
	info.MetaData["filename"] = upload.Filename
	info.MetaData["userid"] = upload.UserID
	info.MetaData["filetype"] = upload.ContentType.String
	info.MetaData["sha256"] = upload.Sha256.String
	info.MetaData["target"] = upload.TargetName.String
	info.MetaData["formdata"] = upload.FormData.String
	return info
}

func (ep *EventProcessor) findRecoveredLocalFile(ctx context.Context, info handler.FileInfo, upload db.Upload) (string, bool, error) {
	targetDir := filepath.Join(ep.uploadDir, "RawMaterial")
	desiredName := info.MetaData["filename"]
	configuredTarget := false
	if upload.TargetName.Valid && upload.TargetName.String != "" {
		target, err := ep.queries.GetTargetByName(ctx, upload.TargetName.String)
		if err == nil {
			configuredTarget = true
			targetDir = target.Path
			if target.FormKey.Valid && target.FormKey.String != "" {
				if form, ok := forms.Get(target.FormKey.String); ok {
					desiredName = forms.BuildFilename(form, parseFormData(info.MetaData["formdata"]), filepath.Ext(desiredName))
				}
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return "", false, fmt.Errorf("resolve local target: %w", err)
		}
	}
	if stat, err := os.Stat(targetDir); err != nil {
		if configuredTarget || !os.IsNotExist(err) {
			return "", false, fmt.Errorf("storage target %s is unavailable: %w", targetDir, err)
		}
		return "", false, nil
	} else if !stat.IsDir() {
		return "", false, fmt.Errorf("storage target %s is not a directory", targetDir)
	}

	candidates := []string{upload.Filename}
	if sanitized, err := SanitizeFilename(desiredName); err == nil && sanitized != upload.Filename {
		candidates = append(candidates, sanitized)
	}
	var match string
	for _, name := range candidates {
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			continue
		}
		candidate := filepath.Join(targetDir, name)
		stat, err := os.Stat(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("inspect possible final file %s: %w", candidate, err)
		}
		if !stat.Mode().IsRegular() || stat.Size() != upload.Size {
			continue
		}
		if upload.Sha256.Valid && upload.Sha256.String != "" {
			actual, err := computeFileSHA256(candidate)
			if err != nil {
				return "", false, fmt.Errorf("verify possible final file %s: %w", candidate, err)
			}
			if actual != upload.Sha256.String {
				continue
			}
		} else if upload.CompletedAt.Valid && stat.ModTime().Before(upload.CompletedAt.Time.Add(-5*time.Second)) {
			// With no content hash, do not mistake an older same-name file for
			// the upload that vanished from the temporary directory.
			continue
		}
		if match != "" && match != candidate {
			return "", false, errors.New("multiple possible final local files found")
		}
		match = candidate
	}
	return match, match != "", nil
}

func (ep *EventProcessor) failRecovery(ctx context.Context, uploadID, reason string) error {
	if err := ep.queries.FailUpload(ctx, uploadID); err != nil {
		return fmt.Errorf("%s; mark storage failed: %w", reason, err)
	}
	_ = os.Remove(filepath.Join(ep.tempDir, uploadID+".info"))
	return errors.New(reason)
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

	err := ep.queries.CreatePendingUpload(context.Background(), db.CreatePendingUploadParams{
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

// finalizeUpload moves a completed upload out of the temp area into its final
// home, then does the bookkeeping common to every destination.
func (ep *EventProcessor) finalizeUpload(info handler.FileInfo, completedAt time.Time) {
	var storedFilename string
	stored := false
	if ep.store != nil && info.MetaData["target"] == objectstore.TargetName {
		storedFilename, stored = ep.storeToS3(info)
	} else {
		storedFilename, stored = ep.storeToDisk(info, completedAt)
	}
	if stored {
		if _, err := ep.queries.FinalizeUploadStorage(context.Background(), db.FinalizeUploadStorageParams{
			Filename: storedFilename,
			ID:       info.ID,
		}); err != nil {
			// Keep tusd's sidecar for restart reconciliation. The published bytes
			// remain inaccessible to package preparation until this atomic state
			// transition succeeds.
			log.Printf("warning: failed to record final storage for upload %s: %v", info.ID, err)
			return
		}
	}
	ep.cleanupFinalization(info, completedAt)
}

// cleanupFinalization removes tusd's temporary bookkeeping after either normal
// finalization or restart recovery, including concatenation partials.
func (ep *EventProcessor) cleanupFinalization(info handler.FileInfo, completedAt time.Time) {
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

// storeToS3 promotes a Send upload into the object store, verifying the SHA-256
// before transferring (unlike the local path) so a corrupt file costs no
// bandwidth. No form handling: an S3-bound upload can't have one.
func (ep *EventProcessor) storeToS3(info handler.FileInfo) (string, bool) {
	srcPath := filepath.Join(ep.tempDir, info.ID)

	filename, err := SanitizeFilename(info.MetaData["filename"])
	if err != nil {
		log.Printf("rejecting upload %s: %v", info.ID, err)
		ep.queries.FailUpload(context.Background(), info.ID)
		os.Remove(srcPath)
		return "", false
	}

	if expected := info.MetaData["sha256"]; expected != "" {
		actual, err := computeFileSHA256(srcPath)
		if err != nil {
			log.Printf("error computing SHA-256 for %s: %v", srcPath, err)
		} else if actual != expected {
			log.Printf("integrity check FAILED for upload %s: expected %s, got %s", info.ID, expected, actual)
			ep.queries.FailUpload(context.Background(), info.ID)
			os.Remove(srcPath)
			return "", false
		} else {
			log.Printf("integrity verified for %s (SHA-256: %s)", info.ID, actual)
		}
	}

	// The row keeps the sanitized name so downloads recompute this exact key —
	// see objectstore.Client.Key.
	key := ep.store.Key(info.ID, filename)
	if err := ep.store.Upload(context.Background(), key, srcPath); err != nil {
		// Leave the temp file: the bytes are intact, so a retry has something to
		// work with.
		log.Printf("error uploading %s to S3: %v", info.ID, err)
		ep.queries.FailUpload(context.Background(), info.ID)
		return "", false
	}

	if err := os.Remove(srcPath); err != nil {
		log.Printf("warning: failed to remove temp file %s after S3 upload: %v", srcPath, err)
	}
	log.Printf("upload saved: s3://%s/%s", ep.store.Bucket(), key)
	return filename, true
}

func (ep *EventProcessor) storeToDisk(info handler.FileInfo, completedAt time.Time) (string, bool) {
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
	if dstPath == "" {
		ep.queries.FailUpload(context.Background(), info.ID)
		return "", false
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

	if integrityFailed {
		return "", false
	}
	return filepath.Base(dstPath), true
}

func (ep *EventProcessor) handleTerminated(event handler.HookEvent) {
	info := event.Upload
	err := ep.queries.DeleteUpload(context.Background(), info.ID)
	if err != nil {
		log.Printf("error deleting upload record: %v", err)
	}
}

// renameUpload moves the uploaded file from its hash-based ID to the original
// filename inside targetDir. Destination selection and publication are one
// atomic, no-replace operation: two processes finishing the same filename can
// never both choose it and silently replace one another. If a name is already
// occupied, a numeric suffix is tried.
//
// Returns the destination path on success, or empty string on failure.
func (ep *EventProcessor) renameUpload(id, filename, targetDir string) string {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Printf("error creating target directory %s: %v", targetDir, err)
		return ""
	}

	src := filepath.Join(ep.tempDir, id)

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
	absDst, err := filepath.Abs(filepath.Join(targetDir, filename))
	if err != nil {
		log.Printf("error resolving destination %s: %v", filename, err)
		return ""
	}
	rel, err := filepath.Rel(absTarget, absDst)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		log.Printf("refusing to write outside target dir: target=%s dst=%s", absTarget, absDst)
		return ""
	}

	dst, err := moveUploadNoReplace(src, targetDir, filename)
	if errors.Is(err, syscall.EXDEV) {
		log.Printf("cross-device copy: %s -> %s", src, targetDir)
		dst, err = crossDeviceMove(src, targetDir, filename)
	}
	if err != nil {
		log.Printf("error moving upload %s into %s: %v", id, targetDir, err)
		return ""
	}
	log.Printf("upload saved: %s", dst)
	return dst
}

// moveUploadNoReplace atomically moves src to the first unoccupied destination
// name. renameNoReplace is implemented with the host OS's exclusive-rename
// primitive, so checking a candidate and claiming it cannot race with another
// process.
func moveUploadNoReplace(src, targetDir, filename string) (string, error) {
	for suffix := 0; ; suffix++ {
		dst := uploadCollisionPath(targetDir, filename, suffix)
		err := renameNoReplace(src, dst)
		if err == nil {
			return dst, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return "", err
	}
}

// crossDeviceMove copies src into a private staging file on the destination
// filesystem, fsyncs it, and then uses the same atomic no-replace publication
// as the same-device path. The source is removed only after publication. On an
// error, the staging file is removed and src remains available for recovery.
func crossDeviceMove(src, targetDir, filename string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.CreateTemp(targetDir, ".filebox-upload-*.part")
	if err != nil {
		return "", fmt.Errorf("create staging file: %w", err)
	}
	part := out.Name()
	cleanup := func() { _ = os.Remove(part) }
	if err := out.Chmod(0644); err != nil {
		out.Close()
		cleanup()
		return "", fmt.Errorf("set staging permissions: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		cleanup()
		return "", fmt.Errorf("copy: %w", err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		cleanup()
		return "", fmt.Errorf("sync: %w", err)
	}
	if err := out.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf("close staging file: %w", err)
	}
	dst, err := moveUploadNoReplace(part, targetDir, filename)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("publish staging file: %w", err)
	}
	if err := os.Remove(src); err != nil {
		log.Printf("warning: failed to remove source %s after cross-device move: %v", src, err)
	}
	return dst, nil
}

func uploadCollisionPath(dir, filename string, suffix int) string {
	if suffix == 0 {
		return filepath.Join(dir, filename)
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	return filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, suffix, ext))
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
