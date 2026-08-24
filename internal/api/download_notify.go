package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/mail"
)

// Coalescing windows for download notifications. The quiet window restarts on
// every download, so a recipient working through a package — or several
// recipients opening the link around the same time — produces one mail instead
// of one per file. maxWait caps the wait from the first download, so a steady
// trickle still gets reported. vars, not consts, so tests needn't sleep.
var (
	downloadQuietWindow = 5 * time.Minute
	downloadMaxWait     = 15 * time.Minute
)

// Bounds on one batch, so a package under sustained download can't grow an
// unbounded mail. Overflow files are reported as a count instead.
const (
	maxBatchFiles       = 50
	maxBatchDownloaders = 50
)

// downloadHit is one file fetch, as recorded while serving it.
type downloadHit struct {
	filename string
	size     int64
	// who names the downloader, "" when the package's verification method
	// identifies nobody.
	who string
	at  time.Time
}

type fileTally struct {
	size  int64
	count int
}

// downloadBatch is the open coalescing window for one package. Batches are keyed
// by package alone: the author wants one "your package is being picked up" mail,
// not one per recipient who happened to click at the same time.
type downloadBatch struct {
	pkg db.Package
	// files tallied by name, with order preserving first-seen for the mail.
	files map[string]*fileTally
	order []string
	// who, deduped and in first-seen order; seen is its index.
	who  []string
	seen map[string]bool
	// dropped counts fetches left out of files once maxBatchFiles was reached.
	dropped     int
	first, last time.Time
	timer       *time.Timer
}

func (b *downloadBatch) add(hit downloadHit) {
	b.last = hit.at

	if t := b.files[hit.filename]; t != nil {
		t.count++
	} else if len(b.order) < maxBatchFiles {
		b.files[hit.filename] = &fileTally{size: hit.size, count: 1}
		b.order = append(b.order, hit.filename)
	} else {
		b.dropped++
	}

	if hit.who != "" && !b.seen[hit.who] && len(b.who) < maxBatchDownloaders {
		b.seen[hit.who] = true
		b.who = append(b.who, hit.who)
	}
}

// downloadNotifier coalesces download notifications in memory. Pending batches
// don't survive a restart — the download and its counters are already committed,
// so all that's lost is the ping.
type downloadNotifier struct {
	mu   sync.Mutex
	open map[string]*downloadBatch
	// send delivers a closed batch. Injected so tests don't need a mailer.
	send       func(*downloadBatch)
	quiet, max time.Duration
}

func newDownloadNotifier(send func(*downloadBatch)) *downloadNotifier {
	return &downloadNotifier{
		open:  map[string]*downloadBatch{},
		send:  send,
		quiet: downloadQuietWindow,
		max:   downloadMaxWait,
	}
}

func (n *downloadNotifier) record(pkg db.Package, hit downloadHit) {
	n.mu.Lock()
	defer n.mu.Unlock()

	b := n.open[pkg.ID]
	if b == nil {
		b = &downloadBatch{
			pkg:   pkg,
			files: map[string]*fileTally{},
			seen:  map[string]bool{},
			first: hit.at,
		}
		n.open[pkg.ID] = b
		b.timer = time.AfterFunc(n.quiet, func() { n.flush(pkg.ID) })
	}
	b.add(hit)

	// Restart the quiet window, clamped so the send never slips past maxWait from
	// the first download.
	wait := n.quiet
	if deadline := b.first.Add(n.max); hit.at.Add(wait).After(deadline) {
		wait = deadline.Sub(hit.at)
	}
	if wait < 0 {
		wait = 0
	}
	b.timer.Reset(wait)
}

// flush closes a batch and hands it to send. Removing it from the map under the
// lock is what makes Reset in record safe: a timer that has already fired blocks
// on the lock, so it still sees the late hit, and the Reset it races with
// re-arms a timer whose batch is gone — that firing finds nothing and returns.
func (n *downloadNotifier) flush(packageID string) {
	n.mu.Lock()
	b := n.open[packageID]
	delete(n.open, packageID)
	n.mu.Unlock()

	if b == nil {
		return
	}
	n.send(b)
}

// drop discards an open window, so switching notifications off doesn't let one
// last mail through minutes later.
func (n *downloadNotifier) drop(packageID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if b := n.open[packageID]; b != nil {
		b.timer.Stop()
		delete(n.open, packageID)
	}
}

type setNotifyRequest struct {
	NotifyOnDownload bool `json:"notifyOnDownload"`
}

// SetPackageNotify turns download reports on or off for a package already sent;
// creator only, like RevokePackage.
func (h *Handlers) SetPackageNotify(w http.ResponseWriter, r *http.Request) {
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
		writeJSONError(w, http.StatusForbidden, "not authorized to change this package")
		return
	}

	var req setNotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.queries.SetPackageNotifyOnDownload(r.Context(), db.SetPackageNotifyOnDownloadParams{
		NotifyOnDownload: boolToInt64(req.NotifyOnDownload),
		ID:               packageID,
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to update notifications")
		return
	}
	if !req.NotifyOnDownload {
		h.downloads.drop(packageID)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"notifyOnDownload": req.NotifyOnDownload})
}

// MutePackageNotifications is the report mail's opt-out, authorised by the token
// in the link rather than a session. Off only, so a leaked link can't make a
// package start reporting. POST because mail scanners prefetch GETs.
func (h *Handlers) MutePackageNotifications(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeJSONError(w, http.StatusNotFound, "Notification link not found")
		return
	}

	pkg, err := h.queries.MutePackageNotificationsByToken(r.Context(), sql.NullString{String: token, Valid: true})
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Notification link not found")
		return
	}
	h.downloads.drop(pkg.ID)

	writeJSON(w, http.StatusOK, map[string]string{"packageName": pkg.Name})
}

// recordDownload notes a served file for the package author's notification mail,
// if they asked for one. Called after the access counters are committed, so a
// download that was refused is never reported.
func (h *Handlers) recordDownload(r *http.Request, pkg db.Package, upload db.Upload) {
	h.recordDownloadItem(r, pkg, upload.Filename, upload.Size)
}

// recordDownloadItem is the artifact-aware form of recordDownload. Generated
// ZIPs do not have an uploads row, but download reports should still name and
// size exactly what the recipient fetched.
func (h *Handlers) recordDownloadItem(r *http.Request, pkg db.Package, filename string, size int64) {
	if pkg.NotifyOnDownload == 0 || !mail.IsEnabled(h.mailer) {
		return
	}

	// Only bcc_login identifies a person; password and none do not, and the mail
	// says "someone with the link" rather than guessing.
	who := ""
	if caller := auth.CallerFrom(r.Context()); caller != nil {
		who = caller.Name
		if who == "" {
			who = caller.Email
		}
	}

	h.downloads.record(pkg, downloadHit{
		filename: filename,
		size:     size,
		who:      who,
		at:       time.Now(),
	})
}
