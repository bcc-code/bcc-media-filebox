package mail

import (
	"fmt"
	"time"
)

// DownloadedFile is one file's activity inside a coalescing window. Count is the
// number of fetches, so a recipient who took the same file twice — or two
// recipients who each took it once — is one row, not two.
type DownloadedFile struct {
	Name  string
	Size  int64
	Count int
}

// SizeLabel renders the file's size for the list, as NotificationFile does.
func (f DownloadedFile) SizeLabel() string { return FormatBytes(f.Size) }

// CountLabel is the "×3" suffix, empty for a single fetch so the common row
// stays clean.
func (f DownloadedFile) CountLabel() string {
	if f.Count <= 1 {
		return ""
	}
	return fmt.Sprintf("×%d", f.Count)
}

// DownloadNotification is the data behind the mail telling an author their
// package is being picked up. One mail covers a whole window of activity on one
// package — see downloadNotifier in internal/api — so everything here is plural.
type DownloadNotification struct {
	AuthorName  string
	PackageName string
	// ManageURL points the author at the package's card in the Send UI.
	ManageURL string
	// MuteURL switches these reports off without a login; empty drops the offer.
	MuteURL string
	// LogoURL is optional; empty renders the wordmark alone.
	LogoURL string
	Files   []DownloadedFile
	// Downloaders are the people the server could name, deduped, in first-seen
	// order. Empty when the package needs no login — then nobody is identifiable
	// and the mail says so rather than implying a single anonymous visitor.
	Downloaders []string
	// Truncated counts fetches dropped from Files because the window held more
	// activity than one mail should list.
	Truncated int
	// MaxDownloads mirrors packages.max_downloads; 0 means unlimited.
	MaxDownloads int
	// DownloadCount is the most-downloaded file's total, matching what the access
	// request mail reports — the number that decides when the package runs dry.
	DownloadCount int
	ExpiresAt     time.Time
	// The window this mail covers. Equal when only one download happened.
	FirstAt time.Time
	LastAt  time.Time
}

// Count is the total number of fetches in the window.
func (d DownloadNotification) Count() int {
	total := d.Truncated
	for _, f := range d.Files {
		total += f.Count
	}
	return total
}

// DownloadLabel renders the fetch count with correct pluralisation.
func (d DownloadNotification) DownloadLabel() string {
	if d.Count() == 1 {
		return "1 download"
	}
	return fmt.Sprintf("%d downloads", d.Count())
}

// FileLabel renders how many distinct files were touched.
func (d DownloadNotification) FileLabel() string {
	if len(d.Files) == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", len(d.Files))
}

// WhoLabel names the downloaders, or admits the package can't identify them.
// Only verification methods that authenticate a person yield names.
func (d DownloadNotification) WhoLabel() string {
	switch len(d.Downloaders) {
	case 0:
		return "Someone with the link"
	case 1:
		return d.Downloaders[0]
	case 2:
		return d.Downloaders[0] + " and " + d.Downloaders[1]
	default:
		return fmt.Sprintf("%s and %d others", d.Downloaders[0], len(d.Downloaders)-1)
	}
}

// TotalSize sums the distinct files, counting a file fetched twice once: it's
// the size of what was taken, not the bytes off the wire.
func (d DownloadNotification) TotalSize() int64 {
	var total int64
	for _, f := range d.Files {
		total += f.Size
	}
	return total
}

// TotalLabel is the formatted total, empty when no sizes are known so templates
// can drop the size clause instead of printing "0 B".
func (d DownloadNotification) TotalLabel() string {
	if d.TotalSize() == 0 {
		return ""
	}
	return FormatBytes(d.TotalSize())
}

// WhenLabel is the window as absolute times — the mail lands minutes after the
// activity, and may be read much later than that.
func (d DownloadNotification) WhenLabel() string {
	const clock = "15:04 MST"
	first := d.FirstAt.Format("2 January 2006, " + clock)
	if d.LastAt.Sub(d.FirstAt) < time.Minute {
		return first
	}
	return first + "–" + d.LastAt.Format(clock)
}

// LimitLabel summarises the download budget, empty when unlimited so the
// templates can drop the line instead of printing "0".
func (d DownloadNotification) LimitLabel() string {
	if d.MaxDownloads == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d used", d.DownloadCount, d.MaxDownloads)
}

// Exhausted reports whether the busiest file has used up its budget, which is
// what turns this mail into a heads-up that recipients may start asking for it
// back.
func (d DownloadNotification) Exhausted() bool {
	return d.MaxDownloads > 0 && d.DownloadCount >= d.MaxDownloads
}

// ExpiresOn is the absolute expiry — see ShareNotification.ExpiresOn.
func (d DownloadNotification) ExpiresOn() string {
	return d.ExpiresAt.Format("2 January 2006, 15:04 MST")
}

// BuildDownloadNotification renders the mail sent to a package's author. No
// Reply-To: the downloader may be anonymous, and even when named, replying to a
// notification isn't the point.
func BuildDownloadNotification(to string, d DownloadNotification) (Message, error) {
	subject := fmt.Sprintf("%s downloaded from your package", d.FileLabel())
	if d.PackageName != "" {
		subject = fmt.Sprintf("%s: %s", subject, d.PackageName)
	}

	text, err := renderText("download_notification.txt.tmpl", d)
	if err != nil {
		return Message{}, err
	}
	htmlBody, err := renderHTML("download_notification.html.tmpl", d)
	if err != nil {
		return Message{}, err
	}

	return Message{
		To:      []string{to},
		Subject: subject,
		Text:    text,
		HTML:    htmlBody,
	}, nil
}
