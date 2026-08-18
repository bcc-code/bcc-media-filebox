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

// notifyTimeout bounds one notification run so a relay that stops answering
// can't leak a goroutine. var, not const, so tests can exhaust it.
var notifyTimeout = 2 * time.Minute

// markTimeout bounds one delivery-status write. Separate from the send context
// on purpose: a run that exhausts notifyTimeout still has to record why.
const markTimeout = 5 * time.Second

func markContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), markTimeout)
}

// notifyRecipients mails every recipient of a new package, detached from the
// request: a relay takes seconds per recipient, and an unsent mail is
// recoverable (sent_at NULL, send_error says why) while a failed create isn't.
func (h *Handlers) notifyRecipients(pkg db.Package, recipients []db.PackageRecipient, uploads []db.Upload, caller *auth.Caller) {
	// Own goroutine, so http's per-request recover can't see it — an unguarded
	// panic would take down the server, not one send.
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

// notifyAccessRequest tells a package's author someone wants the link working
// again. Detached like notifyRecipients — the DB row is the durable record.
func (h *Handlers) notifyAccessRequest(pkg db.Package, req db.PackageAccessRequest, minAccessCount int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("mail: panic notifying author of access request %s: %v\n%s", req.ID, r, debug.Stack())
		}
	}()

	if !mail.IsEnabled(h.mailer) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()

	author, err := h.queries.GetUser(ctx, pkg.CreatedByUserID)
	if err != nil {
		log.Printf("mail: cannot notify author of package %s: %v", pkg.ID, err)
		return
	}
	if !author.Email.Valid || author.Email.String == "" {
		// No address from the provider. The request still shows on their card.
		log.Printf("mail: author of package %s has no email address, request %s not sent", pkg.ID, req.ID)
		return
	}

	manageURL, err := mail.ManageURL(h.mailBaseURL, pkg.ID)
	if err != nil {
		log.Printf("mail: cannot notify author of package %s: %v", pkg.ID, err)
		return
	}

	msg, err := mail.BuildAccessRequestNotification(author.Email.String, mail.AccessRequestNotification{
		AuthorName:     author.Name.String,
		PackageName:    pkg.Name,
		RequesterEmail: req.Email,
		Reason:         req.Reason,
		Message:        req.Message,
		ManageURL:      manageURL,
		LogoURL:        mail.LogoURL(h.mailBaseURL),
		ExpiresAt:      pkg.ExpiresAt,
		MaxDownloads:   int(pkg.MaxDownloads.Int64),
		DownloadCount:  int(minAccessCount),
		RequestedAt:    req.CreatedAt,
	})
	if err == nil {
		err = h.mailer.Send(ctx, msg)
	}
	if err != nil {
		log.Printf("mail: access request %s for package %s to %s failed: %v", req.ID, pkg.ID, author.Email.String, err)
	}
}

// notifyAccessGranted mails everyone whose request an extend just answered.
// Without it, granting is silent to the person waiting on the link.
func (h *Handlers) notifyAccessGranted(pkg db.Package, granted []db.PackageAccessRequest, caller *auth.Caller) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("mail: panic notifying granted requesters of package %s: %v\n%s", pkg.ID, r, debug.Stack())
		}
	}()

	if len(granted) == 0 || !mail.IsEnabled(h.mailer) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()

	shareURL, err := mail.ShareURL(h.mailBaseURL, pkg.ID)
	if err != nil {
		log.Printf("mail: cannot notify granted requesters of package %s: %v", pkg.ID, err)
		return
	}

	// Best effort: a missing file list only costs the mail its file rows.
	var files []mail.NotificationFile
	if rows, err := h.queries.ListSharesByPackageID(ctx, pkg.ID); err == nil {
		files = make([]mail.NotificationFile, len(rows))
		for i, row := range rows {
			files[i] = mail.NotificationFile{Name: row.Filename, Size: row.Size}
		}
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

	for _, req := range granted {
		// One message each: requesters must not see each other.
		msg, err := mail.BuildAccessGrantedNotification(req.Email, data)
		if err == nil {
			err = h.mailer.Send(ctx, msg)
		}
		if err != nil {
			log.Printf("mail: access granted for package %s to %s failed: %v", pkg.ID, req.Email, err)
		}
	}
}
