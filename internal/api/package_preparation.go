package api

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/objectstore"
)

const (
	packagePreparationPollInterval = 2 * time.Second
	packagePreparationMaxAttempts  = int64(3)
	packageProgressReportInterval  = time.Second
	packageProgressReportBytes     = int64(32 << 20)
)

var errPackageArchiveTooLarge = errors.New("generated ZIP would exceed the 100 GiB limit")

// StartPackagePreparationWorker resumes unfinished package artifacts and then
// keeps processing new ones. Only one worker is started per Handlers instance;
// artifact state in SQLite is the durable queue, while the wake channel merely
// avoids waiting for the next poll after a package is created.
func (h *Handlers) StartPackagePreparationWorker(ctx context.Context) {
	h.packageWorkerOnce.Do(func() {
		if reset, err := h.queries.ResetBuildingPackageArtifacts(ctx); err != nil {
			log.Printf("package preparation: reset interrupted artifacts: %v", err)
		} else if reset > 0 {
			log.Printf("package preparation: resumed %d interrupted artifact(s)", reset)
		}
		go h.packagePreparationLoop(ctx)
		go h.notifyReadyPackagesWithUnsentRecipients()
	})
}

func (h *Handlers) wakePackagePreparation() {
	select {
	case h.packagePreparationWake <- struct{}{}:
	default:
	}
}

func (h *Handlers) packagePreparationLoop(ctx context.Context) {
	ticker := time.NewTicker(packagePreparationPollInterval)
	defer ticker.Stop()

	for {
		if err := h.processPreparingPackages(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("package preparation: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-h.packagePreparationWake:
		case <-ticker.C:
		}
	}
}

func (h *Handlers) processPreparingPackages(ctx context.Context) error {
	packages, err := h.queries.ListProcessingPackages(ctx)
	if err != nil {
		return fmt.Errorf("list processing packages: %w", err)
	}
	for _, pkg := range packages {
		if err := h.processPreparingPackage(ctx, pkg); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			log.Printf("package preparation: package %s: %v", pkg.ID, err)
		}
	}
	return nil
}

func (h *Handlers) processPreparingPackage(ctx context.Context, pkg db.Package) error {
	artifacts, err := h.queries.ListPackageArtifacts(ctx, pkg.ID)
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}
	if len(artifacts) == 0 {
		h.failPackagePreparation(pkg.ID, "package has no download artifacts")
		return nil
	}

	var completedBytes int64
	for _, artifact := range artifacts {
		if artifact.Status == "ready" {
			completedBytes += artifact.SourceSize
		}
	}

	for _, artifact := range artifacts {
		if artifact.Status == "ready" {
			continue
		}
		if artifact.Status == "failed" {
			h.failPackagePreparation(pkg.ID, artifact.Error.String)
			return nil
		}
		if artifact.Status != "pending" {
			continue
		}

		members, err := h.queries.ListPackageArtifactMembersWithUploads(ctx, artifact.ID)
		if err != nil {
			return fmt.Errorf("list members for %s: %w", artifact.ID, err)
		}
		if len(members) == 0 {
			h.failArtifactAndPackage(artifact.ID, pkg.ID, "download artifact has no source files")
			return nil
		}

		waiting := false
		for _, member := range members {
			switch member.UploadStorageStatus {
			case "ready":
			case "failed":
				h.failArtifactAndPackage(artifact.ID, pkg.ID, fmt.Sprintf("source file %q failed to reach storage", member.UploadFilename))
				return nil
			default:
				waiting = true
			}
		}
		if waiting {
			continue
		}

		building, err := h.queries.MarkPackageArtifactBuilding(ctx, artifact.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return fmt.Errorf("claim artifact %s: %w", artifact.ID, err)
		}

		var size int64
		var objectKey sql.NullString
		if building.Kind == string(PackageArchiveArtifactDirect) {
			size = building.SourceSize
		} else {
			size, objectKey, err = h.buildPackageZIP(ctx, pkg, building, members, completedBytes)
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			h.handleArtifactBuildFailure(ctx, pkg.ID, building, err)
			return nil
		}

		if _, err := h.queries.MarkPackageArtifactReady(ctx, db.MarkPackageArtifactReadyParams{
			Size:      size,
			ObjectKey: objectKey,
			ID:        building.ID,
		}); err != nil {
			// The object is safely published but not visible until this state
			// transition succeeds. Put it back on the durable queue so a transient
			// SQLite failure does not strand it in "building" until restart.
			if _, resetErr := h.queries.RequeueBuildingPackageArtifact(context.Background(), building.ID); resetErr != nil {
				log.Printf("package preparation: requeue artifact %s after ready-state failure: %v", building.ID, resetErr)
			}
			return fmt.Errorf("mark artifact %s ready: %w", building.ID, err)
		}
		completedBytes += building.SourceSize
		if _, err := h.queries.UpdatePackagePreparationProgress(ctx, db.UpdatePackagePreparationProgressParams{
			PreparationBytesDone: completedBytes,
			ID:                   pkg.ID,
		}); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("record package progress: %w", err)
		}
	}

	artifacts, err = h.queries.ListPackageArtifacts(ctx, pkg.ID)
	if err != nil {
		return fmt.Errorf("reload artifacts: %w", err)
	}
	for _, artifact := range artifacts {
		if artifact.Status != "ready" {
			return nil
		}
	}

	ready, err := h.queries.FinalizePackagePreparation(ctx, pkg.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("finalize package: %w", err)
	}
	log.Printf("package preparation: package %s ready (%d artifact(s))", pkg.ID, len(artifacts))
	go h.notifyPreparedPackage(ready.ID)
	return nil
}

func (h *Handlers) handleArtifactBuildFailure(ctx context.Context, packageID string, artifact db.PackageArtifact, buildErr error) {
	message := truncatePreparationError(buildErr.Error())
	failed, err := h.queries.RequeuePackageArtifactAfterFailure(ctx, db.RequeuePackageArtifactAfterFailureParams{
		MaxAttempts: packagePreparationMaxAttempts,
		Error:       sql.NullString{String: message, Valid: true},
		ID:          artifact.ID,
	})
	if err != nil {
		log.Printf("package preparation: record/requeue artifact %s failure: %v", artifact.ID, err)
		h.failPackagePreparation(packageID, message)
		return
	}
	if failed.Status == "pending" {
		_, _ = h.queries.UpdatePackagePreparationProgress(ctx, db.UpdatePackagePreparationProgressParams{
			PreparationBytesDone: h.readyArtifactBytes(ctx, packageID),
			ID:                   packageID,
		})
		log.Printf("package preparation: artifact %s attempt %d/%d failed, will retry: %v",
			failed.ID, failed.Attempts, packagePreparationMaxAttempts, buildErr)
		return
	}
	h.failPackagePreparation(packageID, message)
}

func (h *Handlers) failArtifactAndPackage(artifactID, packageID, message string) {
	message = truncatePreparationError(message)
	_, _ = h.queries.FailPackageArtifact(context.Background(), db.FailPackageArtifactParams{
		Error: sql.NullString{String: message, Valid: true},
		ID:    artifactID,
	})
	h.failPackagePreparation(packageID, message)
}

func (h *Handlers) failPackagePreparation(packageID, message string) {
	message = truncatePreparationError(message)
	if message == "" {
		message = "Package preparation failed"
	}
	if _, err := h.queries.FailPackagePreparation(context.Background(), db.FailPackagePreparationParams{
		PreparationError: sql.NullString{String: message, Valid: true},
		ID:               packageID,
	}); err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("package preparation: mark package %s failed: %v", packageID, err)
	}
}

func truncatePreparationError(message string) string {
	const max = 1000
	message = strings.TrimSpace(message)
	if len(message) > max {
		return message[:max]
	}
	return message
}

func (h *Handlers) readyArtifactBytes(ctx context.Context, packageID string) int64 {
	artifacts, err := h.queries.ListPackageArtifacts(ctx, packageID)
	if err != nil {
		return 0
	}
	var total int64
	for _, artifact := range artifacts {
		if artifact.Status == "ready" {
			total += artifact.SourceSize
		}
	}
	return total
}

func (h *Handlers) buildPackageZIP(
	ctx context.Context,
	pkg db.Package,
	artifact db.PackageArtifact,
	members []db.ListPackageArtifactMembersWithUploadsRow,
	completedBytes int64,
) (int64, sql.NullString, error) {
	report := newPackageProgressReporter(h, pkg.ID, artifact.ID, completedBytes)

	if h.store != nil {
		key := artifact.ObjectKey.String
		if key == "" {
			key = h.store.PackageArtifactKey(pkg.ID, artifact.ID)
		}
		pr, pw := io.Pipe()
		type zipResult struct {
			size int64
			err  error
		}
		result := make(chan zipResult, 1)
		go func() {
			size, err := h.writePackageZIP(ctx, pw, members, report.add)
			_ = pw.CloseWithError(err)
			result <- zipResult{size: size, err: err}
		}()

		uploadErr := h.store.UploadReader(ctx, key, pr, "application/zip")
		if uploadErr != nil {
			_ = pr.CloseWithError(uploadErr)
		} else {
			_ = pr.Close()
		}
		written := <-result
		report.flush()
		if written.err != nil {
			return 0, sql.NullString{}, written.err
		}
		if uploadErr != nil {
			return 0, sql.NullString{}, uploadErr
		}
		return written.size, sql.NullString{String: key, Valid: true}, nil
	}

	dir := filepath.Join(h.uploadDir, ".archives", pkg.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, sql.NullString{}, fmt.Errorf("create archive directory: %w", err)
	}
	finalPath := filepath.Join(dir, artifact.ID+".zip")
	partPath := finalPath + ".part"
	out, err := os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return 0, sql.NullString{}, fmt.Errorf("create archive: %w", err)
	}
	cleanup := func() { _ = os.Remove(partPath) }

	size, writeErr := h.writePackageZIP(ctx, out, members, report.add)
	report.flush()
	if writeErr == nil {
		writeErr = out.Sync()
	}
	if closeErr := out.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		cleanup()
		return 0, sql.NullString{}, writeErr
	}
	if err := os.Rename(partPath, finalPath); err != nil {
		cleanup()
		return 0, sql.NullString{}, fmt.Errorf("publish archive: %w", err)
	}
	// Rename is only crash-durable once the containing directory has reached
	// disk. Do this before the database can advertise the artifact as ready.
	archiveDir, err := os.Open(dir)
	if err != nil {
		return 0, sql.NullString{}, fmt.Errorf("open archive directory for sync: %w", err)
	}
	syncErr := archiveDir.Sync()
	closeErr := archiveDir.Close()
	if syncErr != nil {
		return 0, sql.NullString{}, fmt.Errorf("sync archive directory: %w", syncErr)
	}
	if closeErr != nil {
		return 0, sql.NullString{}, fmt.Errorf("close archive directory: %w", closeErr)
	}
	return size, sql.NullString{}, nil
}

func (h *Handlers) writePackageZIP(
	ctx context.Context,
	dst io.Writer,
	members []db.ListPackageArtifactMembersWithUploadsRow,
	onProgress func(int64),
) (int64, error) {
	capped := &packageArchiveCappedWriter{dst: dst, limit: packageArchiveMaxSize}
	zw := zip.NewWriter(capped)
	for _, member := range members {
		if err := ctx.Err(); err != nil {
			_ = zw.Close()
			return capped.written, err
		}
		header := &zip.FileHeader{Name: member.ArchiveFilename, Method: zip.Store}
		header.UncompressedSize64 = uint64(member.UploadSize)
		entry, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return capped.written, fmt.Errorf("create ZIP entry %q: %w", member.ArchiveFilename, err)
		}

		source, err := h.openPackageUpload(ctx, member)
		if err != nil {
			_ = zw.Close()
			return capped.written, fmt.Errorf("open source %q: %w", member.UploadFilename, err)
		}
		copied, copyErr := io.Copy(entry, &packageProgressReader{ctx: ctx, src: source, add: onProgress})
		closeErr := source.Close()
		if copyErr != nil {
			_ = zw.Close()
			return capped.written, fmt.Errorf("copy source %q: %w", member.UploadFilename, copyErr)
		}
		if closeErr != nil {
			_ = zw.Close()
			return capped.written, fmt.Errorf("close source %q: %w", member.UploadFilename, closeErr)
		}
		if copied != member.UploadSize {
			_ = zw.Close()
			return capped.written, fmt.Errorf("source %q changed size: expected %d bytes, read %d", member.UploadFilename, member.UploadSize, copied)
		}
	}
	if err := zw.Close(); err != nil {
		return capped.written, fmt.Errorf("finish ZIP: %w", err)
	}
	return capped.written, nil
}

func (h *Handlers) openPackageUpload(ctx context.Context, member db.ListPackageArtifactMembersWithUploadsRow) (io.ReadCloser, error) {
	if member.UploadTargetName.Valid && member.UploadTargetName.String == objectstore.TargetName {
		if h.store == nil {
			return nil, errors.New("S3 source found but S3 is not configured")
		}
		return h.store.Open(ctx, h.store.Key(member.UploadID, member.UploadFilename))
	}

	targetDir := filepath.Join(h.uploadDir, "RawMaterial")
	if member.UploadTargetName.Valid && member.UploadTargetName.String != "" {
		if target, err := h.queries.GetTargetByName(ctx, member.UploadTargetName.String); err == nil {
			targetDir = target.Path
		}
	}
	path, err := safeLocalUploadPath(targetDir, member.UploadFilename)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

type packageArchiveCappedWriter struct {
	dst     io.Writer
	limit   int64
	written int64
}

func (w *packageArchiveCappedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.written
	if remaining < int64(len(p)) {
		return 0, errPackageArchiveTooLarge
	}
	n, err := w.dst.Write(p)
	w.written += int64(n)
	return n, err
}

type packageProgressReader struct {
	ctx context.Context
	src io.Reader
	add func(int64)
}

func (r *packageProgressReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.src.Read(p)
	if n > 0 {
		r.add(int64(n))
	}
	return n, err
}

type packageProgressReporter struct {
	h             *Handlers
	packageID     string
	artifactID    string
	base          int64
	copied        int64
	lastBytes     int64
	lastPublished time.Time
}

func newPackageProgressReporter(h *Handlers, packageID, artifactID string, base int64) *packageProgressReporter {
	return &packageProgressReporter{
		h:             h,
		packageID:     packageID,
		artifactID:    artifactID,
		base:          base,
		lastPublished: time.Now(),
	}
}

func (p *packageProgressReporter) add(delta int64) {
	p.copied += delta
	if p.copied-p.lastBytes >= packageProgressReportBytes || time.Since(p.lastPublished) >= packageProgressReportInterval {
		p.publish()
	}
}

func (p *packageProgressReporter) flush() { p.publish() }

func (p *packageProgressReporter) publish() {
	if p.copied == p.lastBytes && !p.lastPublished.IsZero() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, artifactErr := p.h.queries.UpdatePackageArtifactProgress(ctx, db.UpdatePackageArtifactProgressParams{
		ProgressBytes: p.copied,
		ID:            p.artifactID,
	})
	_, packageErr := p.h.queries.UpdatePackagePreparationProgress(ctx, db.UpdatePackagePreparationProgressParams{
		PreparationBytesDone: p.base + p.copied,
		ID:                   p.packageID,
	})
	if artifactErr != nil && !errors.Is(artifactErr, sql.ErrNoRows) {
		log.Printf("package preparation: artifact %s progress: %v", p.artifactID, artifactErr)
	}
	if packageErr != nil && !errors.Is(packageErr, sql.ErrNoRows) {
		log.Printf("package preparation: package %s progress: %v", p.packageID, packageErr)
	}
	p.lastBytes = p.copied
	p.lastPublished = time.Now()
}

func packageArchiveFilename(packageName string, index, count int) string {
	base := strings.TrimSpace(packageName)
	if base == "" {
		base = "package"
	}
	var clean strings.Builder
	for _, r := range base {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			clean.WriteRune(r)
		case r == ' ':
			clean.WriteByte('-')
		default:
			clean.WriteByte('_')
		}
		if clean.Len() >= 120 {
			break
		}
	}
	base = strings.Trim(clean.String(), "-_.")
	if base == "" {
		base = "package"
	}
	if count <= 1 {
		return base + ".zip"
	}
	return fmt.Sprintf("%s-part-%03d-of-%03d.zip", base, index, count)
}

func uploadByID(uploads []db.Upload, id string) (db.Upload, bool) {
	for _, upload := range uploads {
		if upload.ID == id {
			return upload, true
		}
	}
	return db.Upload{}, false
}

func preparationProgress(pkg db.Package) int {
	if pkg.PreparationStatus == "ready" {
		return 100
	}
	if pkg.PreparationBytesTotal <= 0 || pkg.PreparationBytesDone <= 0 {
		return 0
	}
	progress := int(float64(pkg.PreparationBytesDone) / float64(pkg.PreparationBytesTotal) * 100)
	if progress > 99 && pkg.PreparationStatus == "processing" {
		return 99
	}
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

// notifyPreparedPackage reconstructs the request-time mail inputs after an
// asynchronous build. Delivery status on each recipient remains authoritative.
func (h *Handlers) notifyPreparedPackage(packageID string) {
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()

	pkg, err := h.queries.GetPackageByID(ctx, packageID)
	if err != nil || pkg.PreparationStatus != "ready" || pkg.Status != "active" || !pkg.ExpiresAt.After(time.Now()) || isPermanentlyExpired(pkg) {
		return
	}
	recipients, err := h.queries.ListPackageRecipientsByPackageID(ctx, packageID)
	if err != nil {
		log.Printf("mail: load recipients for prepared package %s: %v", packageID, err)
		return
	}
	unsent := recipients[:0]
	for _, recipient := range recipients {
		if !recipient.SentAt.Valid {
			unsent = append(unsent, recipient)
		}
	}
	if len(unsent) == 0 {
		return
	}

	rows, err := h.queries.ListSharesByPackageID(ctx, packageID)
	if err != nil {
		log.Printf("mail: load files for prepared package %s: %v", packageID, err)
		return
	}
	uploads := make([]db.Upload, len(rows))
	for i, row := range rows {
		uploads[i] = db.Upload{ID: row.UploadID, Filename: row.Filename, Size: row.Size}
	}
	author, err := h.queries.GetUser(ctx, pkg.CreatedByUserID)
	if err != nil {
		log.Printf("mail: load author for prepared package %s: %v", packageID, err)
		return
	}
	caller := &auth.Caller{
		UserID:   author.ID,
		Provider: author.Provider,
		Subject:  author.Subject,
		Email:    author.Email.String,
		Name:     author.Name.String,
		Role:     author.Role,
	}
	h.notifyRecipients(pkg, unsent, uploads, caller)
}

// notifyReadyPackagesWithUnsentRecipients closes the small crash window between
// making a package ready and starting its delivery goroutine. Per-recipient
// sent_at values make this safe to run once at every process start.
func (h *Handlers) notifyReadyPackagesWithUnsentRecipients() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	packages, err := h.queries.ListReadyPackagesWithUnsentRecipients(ctx)
	cancel()
	if err != nil {
		log.Printf("mail: recover prepared package deliveries: %v", err)
		return
	}
	for _, pkg := range packages {
		h.notifyPreparedPackage(pkg.ID)
	}
}
