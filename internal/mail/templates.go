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

// ShareURL builds the recipient-facing link for a package, mirroring the
// frontend's /s/:packageId route. baseURL is MAIL_LINK_BASE_URL, else BASE_URL.
func ShareURL(baseURL, packageID string) (string, error) {
	u, err := linkOrigin(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/s/" + url.PathEscape(packageID)
	return u.String(), nil
}

// ManageURL builds the author-facing link: the Send page's "Sent packages" tab
// focused on one package. Send.vue reads the tab/package query params.
func ManageURL(baseURL, packageID string) (string, error) {
	u, err := linkOrigin(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/send"
	u.RawQuery = url.Values{"tab": {"sent"}, "package": {packageID}}.Encode()
	return u.String(), nil
}

func linkOrigin(baseURL string) (*url.URL, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("no link origin configured — cannot build a link (set MAIL_LINK_BASE_URL or BASE_URL)")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse link origin %q: %w", baseURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("link origin %q must be an absolute URL including scheme and host", baseURL)
	}
	return u, nil
}

// Lives in frontend/public, so it's served unauthenticated from the app origin.
const logoAsset = "logo-email.png"

// LogoURL returns the brand mark's absolute URL, or "" when baseURL is unusable.
// The templates then show the wordmark alone, as they do for blocked images.
func LogoURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + logoAsset
	return u.String()
}

// NotificationFile is one row of the file list, mirroring .public-file rows.
type NotificationFile struct {
	Name string
	Size int64
}

// ShareNotification is the data behind the "someone shared files with you" mail.
// Same fields PackageDownloadScreen.vue renders, so mail and page read alike.
// SenderEmail becomes Reply-To, reaching the sharer, not the service mailbox.
type ShareNotification struct {
	SenderName  string
	SenderEmail string
	PackageName string
	// Message is the sender's own note, may be empty.
	Message  string
	ShareURL string
	// LogoURL is optional; empty renders the wordmark alone.
	LogoURL string
	// Optional: when empty the mail omits the list and falls back to FileCount,
	// so a caller that hasn't loaded the shares still sends.
	Files     []NotificationFile
	FileCount int
	// MaxDownloads mirrors packages.max_downloads; 0 means unlimited.
	MaxDownloads int
	ExpiresAt    time.Time
	// Switches the wording to "the link you asked about works again", keeping the
	// same layout so a recipient sees one familiar mail, not two.
	Renewed bool
}

// Headline is the mail's one-line summary: the text opening and the HTML <title>.
// Branching here, not in two templates, so the wordings can't drift.
func (d ShareNotification) Headline() string {
	if d.SenderName == "" {
		if d.Renewed {
			return "Your download link works again"
		}
		return "You've received files"
	}
	return d.SenderName + " " + d.HeadlineVerb()
}

// HeadlineVerb is Headline after the sender's name, for the HTML body — which
// bolds the name and so can't use Headline whole.
func (d ShareNotification) HeadlineVerb() string {
	if d.Renewed {
		return "renewed your download link"
	}
	return "sent you a package"
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

// TotalLabel is the formatted total, empty when no sizes are known so templates
// can drop the size clause instead of printing "0 B".
func (d ShareNotification) TotalLabel() string {
	if d.TotalSize() == 0 {
		return ""
	}
	return FormatBytes(d.TotalSize())
}

// ExpiresOn is the absolute expiry. The page can say "in 3 days"; a mail may be
// read days after it was sent.
func (d ShareNotification) ExpiresOn() string {
	return d.ExpiresAt.Format("2 January 2006, 15:04 MST")
}

// MessageHTML escapes the sender's note and turns newlines into breaks, since
// white-space: pre-wrap is unreliable in Outlook.
func (d ShareNotification) MessageHTML() htmltemplate.HTML {
	escaped := html.EscapeString(d.Message)
	return htmltemplate.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}

// FormatBytes mirrors fmtBytes in the send components, so sizes read alike.
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

// BuildShareNotification renders both bodies into a Message. One call per
// recipient — recipients must not see each other's addresses.
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

// BuildAccessGrantedNotification answers a granted request: BuildShareNotification
// re-headlined, sent only to the requester. Without it, granting is silent.
func BuildAccessGrantedNotification(to string, d ShareNotification) (Message, error) {
	d.Renewed = true

	subject := "Your download link works again"
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
