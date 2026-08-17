package mail

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// sampleOrigin is where the sample's link and logo point. It defaults to the
// Vite dev server (the port vite.config.ts pins) so a preview opened locally
// has a clickable link and a logo that loads; MAIL_LINK_BASE_URL overrides it,
// matching what the running app would use.
func sampleOrigin() string {
	if v := os.Getenv("MAIL_LINK_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:8091"
}

// sampleNotification is realistic data for the two manual helpers below: the
// browser preview and the Mailpit send.
func sampleNotification(t *testing.T) ShareNotification {
	t.Helper()

	shareURL, err := ShareURL(sampleOrigin(), "9f3ac21b7d")
	if err != nil {
		t.Fatalf("build sample share URL: %v", err)
	}

	return ShareNotification{
		SenderName:  "John Doe",
		SenderEmail: "john.doe@bcc.no",
		PackageName: "Summer conference rushes",
		Message:     "Here are the files we talked about.\n\nShout if anything is missing.",
		ShareURL:    shareURL,
		LogoURL:     LogoURL(sampleOrigin()),
		Files: []NotificationFile{
			{Name: "opening-wide-4k.mov", Size: 4823400000},
			{Name: "interview-anna-cam-a.mov", Size: 1288490188},
			{Name: "shot-notes.pdf", Size: 82043},
		},
		MaxDownloads: 3,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}
}

// TestWritePreview dumps the rendered bodies to disk so the mail design can be
// eyeballed in a browser without a mail server. Skipped unless MAIL_PREVIEW_DIR
// is set:
//
//	MAIL_PREVIEW_DIR=/tmp/fb go test ./internal/mail/ -run Preview && open /tmp/fb/share.html
//
// A browser is a friendlier renderer than any mail client, so this checks
// layout and colour only — use Mailpit for how clients actually treat it.
func TestWritePreview(t *testing.T) {
	dir := os.Getenv("MAIL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set MAIL_PREVIEW_DIR=/tmp/fb to dump the rendered mail for review")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create preview dir: %v", err)
	}

	msg, err := BuildShareNotification("recipient@example.com", sampleNotification(t))
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	for name, body := range map[string]string{"share.html": msg.HTML, "share.txt": msg.Text} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
	}
	t.Logf("subject: %s", msg.Subject)
	t.Logf("links point at %s", sampleOrigin())
}
