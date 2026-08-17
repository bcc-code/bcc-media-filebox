package mail

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	htmltemplate "html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
	"time"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var (
	htmlTemplates = htmltemplate.Must(htmltemplate.ParseFS(templateFS, "templates/*.html.tmpl"))
	textTemplates = texttemplate.Must(texttemplate.ParseFS(templateFS, "templates/*.txt.tmpl"))
)

// ShareURL builds the recipient-facing link for a package. baseURL is the
// configured link origin (MAIL_LINK_BASE_URL, else BASE_URL); the path mirrors
// the frontend's /s/:packageId route in frontend/src/router.ts.
func ShareURL(baseURL, packageID string) (string, error) {
	if baseURL == "" {
		return "", fmt.Errorf("no link origin configured — cannot build a share link (set MAIL_LINK_BASE_URL or BASE_URL)")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse link origin %q: %w", baseURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("link origin %q must be an absolute URL including scheme and host", baseURL)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/s/" + url.PathEscape(packageID)
	return u.String(), nil
}

// logoAsset lives in frontend/public, so it is served unauthenticated from the
// app's origin in production and from the Vite dev server in development.
const logoAsset = "logo-email.png"

// LogoURL returns the absolute URL of the brand mark, or "" when baseURL is
// unusable — the templates then fall back to the wordmark on its own, which is
// also what a recipient sees when their client blocks images.
func LogoURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + logoAsset
	return u.String()
}

// NotificationFile is one row of the file list, mirroring the .public-file rows
// on the recipient page.
type NotificationFile struct {
	Name string
	Size int64
}

// ShareNotification is the data behind the "someone shared files with you"
// mail. It carries the same fields PackageDownloadScreen.vue renders, so the
// mail and the page it links to read as one thing.
//
// SenderEmail becomes Reply-To, so a recipient's reply reaches the person who
// actually shared rather than the unattended service mailbox.
type ShareNotification struct {
	SenderName  string
	SenderEmail string
	PackageName string
	// Message is the sender's own note, may be empty.
	Message  string
	ShareURL string
	// LogoURL is optional; empty renders the wordmark alone.
	LogoURL string
	// Files is optional. When empty the mail falls back to FileCount and omits
	// the file list, so a caller that hasn't loaded the shares still sends.
	Files     []NotificationFile
	FileCount int
	// MaxDownloads mirrors packages.max_downloads; 0 means unlimited.
	MaxDownloads int
	ExpiresAt    time.Time
}

// Count is the number of files, from the list when present.
func (d ShareNotification) Count() int {
	if len(d.Files) > 0 {
		return len(d.Files)
	}
	return d.FileCount
}

// FileLabel renders the file count with correct pluralisation.
func (d ShareNotification) FileLabel() string {
	if d.Count() == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", d.Count())
}

// TotalSize sums the file list; 0 when sizes are unknown.
func (d ShareNotification) TotalSize() int64 {
	var total int64
	for _, f := range d.Files {
		total += f.Size
	}
	return total
}

// TotalLabel is the formatted total, empty when no sizes are known — templates
// use emptiness to drop the size clause rather than print "0 B".
func (d ShareNotification) TotalLabel() string {
	if d.TotalSize() == 0 {
		return ""
	}
	return FormatBytes(d.TotalSize())
}

// ExpiresOn is the absolute expiry. The recipient page shows a relative
// "expires in 3 days", but a mail may be read days after it was sent, so this
// one is anchored.
func (d ShareNotification) ExpiresOn() string {
	return d.ExpiresAt.Format("2 January 2006, 15:04 MST")
}

// MessageHTML escapes the sender's note and turns newlines into breaks. The
// CSS equivalent (white-space: pre-wrap) is unreliable in Outlook.
func (d ShareNotification) MessageHTML() htmltemplate.HTML {
	escaped := html.EscapeString(d.Message)
	return htmltemplate.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}

// FormatBytes mirrors fmtBytes in the frontend's send components so a size
// reads identically in the mail and on the page.
func FormatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v, i := float64(b), -1
	for {
		v /= 1024
		i++
		if v < 1024 || i == len(units)-1 {
			break
		}
	}
	if v < 10 {
		return fmt.Sprintf("%.2f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// Size renders one file's size for the list.
func (f NotificationFile) SizeLabel() string { return FormatBytes(f.Size) }

// BuildShareNotification renders both bodies and returns a Message ready to
// hand to a Sender. One call per recipient — recipients must not be able to
// see each other's addresses.
func BuildShareNotification(to string, d ShareNotification) (Message, error) {
	// Mirrors PackageDownloadScreen's "<name> sent you a package".
	subject := fmt.Sprintf("%s sent you a package", d.SenderName)
	if d.SenderName == "" {
		subject = fmt.Sprintf("You've received %s", d.FileLabel())
	}
	if d.PackageName != "" {
		subject = fmt.Sprintf("%s: %s", subject, d.PackageName)
	}

	text, err := renderText("share_notification.txt.tmpl", d)
	if err != nil {
		return Message{}, err
	}
	htmlBody, err := renderHTML("share_notification.html.tmpl", d)
	if err != nil {
		return Message{}, err
	}

	return Message{
		To:          []string{to},
		ReplyTo:     d.SenderEmail,
		ReplyToName: d.SenderName,
		SenderName:  d.SenderName,
		Subject:     subject,
		Text:        text,
		HTML:        htmlBody,
	}, nil
}

func renderText(name string, data any) (string, error) {
	var b bytes.Buffer
	if err := textTemplates.ExecuteTemplate(&b, name, data); err != nil {
		return "", fmt.Errorf("render %s: %w", name, err)
	}
	return b.String(), nil
}

func renderHTML(name string, data any) (string, error) {
	var b bytes.Buffer
	if err := htmlTemplates.ExecuteTemplate(&b, name, data); err != nil {
		return "", fmt.Errorf("render %s: %w", name, err)
	}
	return b.String(), nil
}
