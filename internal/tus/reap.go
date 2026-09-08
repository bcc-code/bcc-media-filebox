package tus

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// uploadStatusCompleted is the `uploads.status` value that means the transfer
// finished. Such a row is finalization's business, never the reaper's.
const uploadStatusCompleted = "completed"

const (
	// defaultTempTTL is how long an untouched temporary upload survives before
	// the reaper removes it. It has to be long enough that a user who closes
	// the laptop mid-upload can still resume — tus resume depends on the
	// client's own fingerprint store, so in practice that happens within hours
	// — and short enough that an abandoned upload does not pin its bytes
	// forever.
	defaultTempTTL = 7 * 24 * time.Hour

	// tempReapInterval paces the sweep. The work is one directory listing plus
	// a row lookup per stale id, so it is cheap; the interval only bounds how
	// long an abandoned upload outlives its TTL.
	tempReapInterval = time.Hour
)

// tempSuffixes are the files tusd's filestore and filelocker create alongside
// an upload's binary. Reaping an id means removing all of them.
var tempSuffixes = []string{"", ".info", ".lock", ".stop"}

// TempTTL overrides defaultTempTTL. Zero means the default.
//
// It is a field rather than a constructor argument because tests build an
// EventProcessor directly, and because the useful value depends on how much
// temp disk the deployment has relative to its upload sizes.
func (ep *EventProcessor) tempTTL() time.Duration {
	if ep.TempTTL > 0 {
		return ep.TempTTL
	}
	return defaultTempTTL
}

// StartTempReaper sweeps the temp directory now and then keeps sweeping on a
// ticker. Nothing else in the process ever scans tempDir: RecoverPending works
// from `uploads` rows filtered to completed non-partial uploads, so an upload
// abandoned before completion is invisible to it and its bytes stay on disk
// indefinitely.
//
// That was a slow leak while a partial only occupied the bytes actually
// uploaded. It stops being slow once parallel uploads preallocate a full-size
// group file: an abandoned 264 GB upload would pin 264 GB from its first PATCH.
func (ep *EventProcessor) StartTempReaper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(tempReapInterval)
		defer ticker.Stop()

		for {
			if removed, err := ep.ReapTemp(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					log.Printf("temp reaper: %v", err)
				}
			} else if removed > 0 {
				log.Printf("temp reaper: removed %d abandoned temporary upload(s)", removed)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// ReapTemp removes temporary uploads that no longer belong to anything. An id
// is abandoned when its newest file has not been touched for tempTTL and its
// database row is either gone or still short of completion.
//
// Completed rows are never touched, whatever their age: those belong to
// RecoverPending, which promotes them into their target on the next restart.
// Deleting them here would turn a recoverable upload into data loss.
//
// Freshness is judged by modification time and never by size. A preallocated
// group file is full length from its first write, so its size says nothing
// about how much has arrived.
func (ep *EventProcessor) ReapTemp(ctx context.Context) (int, error) {
	entries, err := os.ReadDir(ep.tempDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	// Group the flat directory listing by upload id: `<id>`, `<id>.info`,
	// `<id>.lock` and `<id>.stop` are one upload, and the id is stale only if
	// every one of them is.
	newest := make(map[string]time.Time, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		id := tempUploadID(entry.Name())
		if id == "" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			// Vanished mid-listing, or unreadable. Either way, leave it alone:
			// without a modification time there is no evidence it is stale.
			continue
		}
		if mod := info.ModTime(); mod.After(newest[id]) {
			newest[id] = mod
		}
	}

	cutoff := time.Now().Add(-ep.tempTTL())
	removed := 0
	for id, mod := range newest {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		if mod.After(cutoff) {
			continue
		}
		reap, err := ep.shouldReap(ctx, id)
		if err != nil {
			log.Printf("temp reaper: inspect %s: %v", id, err)
			continue
		}
		if !reap {
			continue
		}
		if ep.removeTempFiles(id) {
			removed++
		}
	}
	return removed, nil
}

// tempUploadID maps a file in the temp directory back to its upload id, or ""
// for a name that is not part of an upload.
func tempUploadID(name string) string {
	if name == "" || strings.HasPrefix(name, ".") {
		// Includes crossDeviceMove's ".filebox-upload-*.part" staging files,
		// which that function cleans up itself and which may be mid-copy.
		return ""
	}
	for _, suffix := range tempSuffixes {
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}

// shouldReap decides an id's fate from its database row.
func (ep *EventProcessor) shouldReap(ctx context.Context, id string) (bool, error) {
	row, err := ep.queries.GetUpload(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		// No row: a terminated upload whose files outlived it, or a group file
		// whose final POST never arrived. Nothing will ever claim it.
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if row.Status == uploadStatusCompleted {
		return false, nil
	}
	return true, nil
}

// removeTempFiles deletes every file belonging to an id, and its row. It
// reports whether anything was actually removed.
func (ep *EventProcessor) removeTempFiles(id string) bool {
	removedAny := false
	for _, suffix := range tempSuffixes {
		path := filepath.Join(ep.tempDir, id+suffix)
		if err := os.Remove(path); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("temp reaper: remove %s: %v", path, err)
			}
			continue
		}
		removedAny = true
	}
	if !removedAny {
		return false
	}
	// The row goes last: while it exists, a crash between the two leaves an
	// upload the reaper will simply pick up again.
	if err := ep.queries.DeleteUpload(context.Background(), id); err != nil {
		log.Printf("temp reaper: delete row %s: %v", id, err)
	}
	return true
}
