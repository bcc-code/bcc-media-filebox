package api

import (
	"context"
	"database/sql"
	"log"
	"runtime/debug"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/mail"
)

// notifyTimeout bounds one package's whole notification run. A corporate relay
// that stops answering must not leak a goroutine.
// var, not const, so tests can exhaust it without waiting two minutes.
var notifyTimeout = 2 * time.Minute

// markTimeout bounds one delivery-status write. These get their own context on
// purpose: a run that exhausts notifyTimeout still has to record why, and
// reusing the expired send context would drop the reason silently.
const markTimeout = 5 * time.Second

func markContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), markTimeout)
}

// notifyRecipients mails every recipient of a freshly created package.
//
// It runs detached from the request: SMTP against a real relay takes seconds
// per recipient, and a created package with an unsent mail is recoverable
// (sent_at stays NULL, send_error says why) while a package creation that
// failed because a relay was slow is not.
func (h *Handlers) notifyRecipients(pkg db.Package, recipients []db.PackageRecipient, uploads []db.Upload, caller *auth.Caller) {
	// Runs on its own goroutine, so http's per-request recover cannot see it:
	// an unguarded panic here would take down the whole server, not one send.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("mail: panic notifying recipients of package %s: %v\n%s", pkg.ID, r, debug.Stack())
		}
	}()

	if len(recipients) == 0 || !mail.IsEnabled(h.mailer) {
		return
	}

	shareURL, err := mail.ShareURL(h.mailBaseURL, pkg.ID)
	if err != nil {
		// Startup already refuses this combination, so reaching here means
		// BASE_URL changed under a running process.
		log.Printf("mail: cannot notify recipients of package %s: %v", pkg.ID, err)
		return
	}

	files := make([]mail.NotificationFile, 0, len(uploads))
	for _, u := range uploads {
		files = append(files, mail.NotificationFile{Name: u.Filename, Size: u.Size})
	}

	data := mail.ShareNotification{
		PackageName:  pkg.Name,
		Message:      pkg.Message,
		ShareURL:     shareURL,
		LogoURL:      mail.LogoURL(h.mailBaseURL),
		Files:        files,
		ExpiresAt:    pkg.ExpiresAt,
		MaxDownloads: int(pkg.MaxDownloads.Int64),
	}
	if caller != nil {
		data.SenderName = caller.Name
		data.SenderEmail = caller.Email
	}

	// sendCtx bounds the SMTP work only — status writes below use markContext.
	sendCtx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()

	for _, rcpt := range recipients {
		// One message per recipient: recipients must not see each other.
		msg, err := mail.BuildShareNotification(rcpt.Email, data)
		if err == nil {
			err = h.mailer.Send(sendCtx, msg)
		}

		markCtx, cancelMark := markContext()
		if err != nil {
			log.Printf("mail: package %s to %s failed: %v", pkg.ID, rcpt.Email, err)
			if dbErr := h.queries.MarkPackageRecipientSendFailed(markCtx, db.MarkPackageRecipientSendFailedParams{
				SendError: sql.NullString{String: err.Error(), Valid: true},
				ID:        rcpt.ID,
			}); dbErr != nil {
				log.Printf("mail: recording failure for recipient %d: %v", rcpt.ID, dbErr)
			}
		} else if dbErr := h.queries.MarkPackageRecipientSent(markCtx, rcpt.ID); dbErr != nil {
			log.Printf("mail: recording delivery for recipient %d: %v", rcpt.ID, dbErr)
		}
		cancelMark()
	}
}
