package mail

import (
	"fmt"
	"html"
	htmltemplate "html/template"
	"strings"
	"time"
)

// Request reasons, mirroring package_access_requests.reason: the state the
// package was in when asked, since the author may read the mail days later.
const (
	ReasonExpired            = "expired"
	ReasonRevoked            = "revoked"
	ReasonLimitReached       = "limit_reached"
	ReasonPermanentlyExpired = "permanently_expired"
)

// AccessRequestNotification is the data behind the mail telling an author
// someone wants their package back. RequesterEmail becomes Reply-To — the mirror
// of BuildShareNotification, where Reply-To is the author instead.
type AccessRequestNotification struct {
	AuthorName     string
	PackageName    string
	RequesterEmail string
	Reason string
	Message string
	ManageURL string
	LogoURL string
	ExpiresAt time.Time
	MaxDownloads int
	DownloadCount int
	RequestedAt   time.Time
}

// ReasonLabel explains what the recipient hit, in the recipient's terms.
func (d AccessRequestNotification) ReasonLabel() string {
	switch d.Reason {
	case ReasonExpired:
		return "The link had expired"
	case ReasonRevoked:
		return "The package had been revoked"
	case ReasonLimitReached:
		return "Every file had reached its download limit"
	default:
		return "The package was no longer available"
	}
}

// LimitLabel summarises the download budget, empty when unlimited so the
// templates can drop the line instead of printing "0".
func (d AccessRequestNotification) LimitLabel() string {
	if d.MaxDownloads == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d used", d.DownloadCount, d.MaxDownloads)
}

// ExpiredOn is the expiry as an absolute date — see ShareNotification.ExpiresOn.
func (d AccessRequestNotification) ExpiredOn() string {
	return d.ExpiresAt.Format("2 January 2006, 15:04 MST")
}

// MessageHTML escapes the requester's note and turns newlines into breaks.
func (d AccessRequestNotification) MessageHTML() htmltemplate.HTML {
	escaped := html.EscapeString(d.Message)
	return htmltemplate.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}

// BuildAccessRequestNotification renders the mail sent to a package's author.
func BuildAccessRequestNotification(to string, d AccessRequestNotification) (Message, error) {
	subject := fmt.Sprintf("%s asked you to reopen a package", d.RequesterEmail)
	if d.PackageName != "" {
		subject = fmt.Sprintf("%s: %s", subject, d.PackageName)
	}

	text, err := renderText("access_request.txt.tmpl", d)
	if err != nil {
		return Message{}, err
	}
	htmlBody, err := renderHTML("access_request.html.tmpl", d)
	if err != nil {
		return Message{}, err
	}

	return Message{
		To:      []string{to},
		ReplyTo: d.RequesterEmail,
		Subject: subject,
		Text:    text,
		HTML:    htmlBody,
	}, nil
}
