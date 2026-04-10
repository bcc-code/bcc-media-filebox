package tus

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	db "file-pusher/internal/db/gen"

	"github.com/tus/tusd/v2/pkg/handler"
)

type EventProcessor struct {
	queries   *db.Queries
	uploadDir string
}

func NewEventProcessor(queries *db.Queries, uploadDir string) *EventProcessor {
	return &EventProcessor{queries: queries, uploadDir: uploadDir}
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

	// Rename the file from hash ID to original filename
	filename := info.MetaData["filename"]
	if filename != "" {
		ep.renameUpload(info.ID, filename)
	}

	// Clean up partial files and .info files for concatenated uploads
	if info.PartialUploads != nil {
		for _, partialID := range info.PartialUploads {
			os.Remove(filepath.Join(ep.uploadDir, partialID))
			os.Remove(filepath.Join(ep.uploadDir, partialID+".info"))
		}
		// Delete partial DB records
		for _, partialID := range info.PartialUploads {
			ep.queries.DeleteUpload(context.Background(), partialID)
		}
	}

	// Remove the .info file for the completed upload
	os.Remove(filepath.Join(ep.uploadDir, info.ID+".info"))
}

func (ep *EventProcessor) handleTerminated(event handler.HookEvent) {
	info := event.Upload
	err := ep.queries.DeleteUpload(context.Background(), info.ID)
	if err != nil {
		log.Printf("error deleting upload record: %v", err)
	}
}

// renameUpload moves the uploaded file from its hash-based ID to the original filename.
// If a file with the same name exists, a numeric suffix is added.
func (ep *EventProcessor) renameUpload(id, filename string) {
	src := filepath.Join(ep.uploadDir, id)
	dst := ep.uniquePath(filename)

	if err := os.Rename(src, dst); err != nil {
		log.Printf("error renaming upload %s to %s: %v", id, dst, err)
	} else {
		log.Printf("upload saved: %s", dst)
	}
}

func (ep *EventProcessor) uniquePath(filename string) string {
	dst := filepath.Join(ep.uploadDir, filename)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return dst
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	for i := 1; ; i++ {
		dst = filepath.Join(ep.uploadDir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return dst
		}
	}
}
