package mail

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// sampleOrigin is where the sample's link and logo point, defaulting to the Vite
// dev server so both work locally. MAIL_LINK_BASE_URL overrides it.
func sampleOrigin() string {
	if v := os.Getenv("MAIL_LINK_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:8091"
}

// sampleNotification is realistic data for the manual preview and Mailpit send.
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

// sampleAccessRequest is realistic data for the author-facing request mail.
func sampleAccessRequest(t *testing.T) AccessRequestNotification {
	t.Helper()

	manageURL, err := ManageURL(sampleOrigin(), "9f3ac21b7d")
	if err != nil {
		t.Fatalf("build sample manage URL: %v", err)
	}

	return AccessRequestNotification{
		AuthorName:     "John Doe",
		PackageName:    "Summer conference rushes",
		RequesterEmail: "anna.berg@example.com",
		Reason:         ReasonExpired,
		Message:        "Our editor was off sick last week and the link ran out.\n\nCould we get a few more days?",
		ManageURL:      manageURL,
		LogoURL:        LogoURL(sampleOrigin()),
		ExpiresAt:      time.Now().Add(-2 * 24 * time.Hour),
		MaxDownloads:   3,
		DownloadCount:  1,
		RequestedAt:    time.Now(),
	}
}

// TestWritePreview dumps the rendered bodies to disk so the design can be
// eyeballed in a browser. Skipped unless MAIL_PREVIEW_DIR is set:
//
//	MAIL_PREVIEW_DIR=/tmp/fb go test ./internal/mail/ -run Preview && open /tmp/fb/share.html
//
// Checks layout and colour only — use Mailpit for how clients treat it.
func TestWritePreview(t *testing.T) {
	dir := os.Getenv("MAIL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set MAIL_PREVIEW_DIR=/tmp/fb to dump the rendered mail for review")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create preview dir: %v", err)
	}

	share, err := BuildShareNotification("recipient@example.com", sampleNotification(t))
	if err != nil {
		t.Fatalf("build share notification: %v", err)
	}
	// The renewed variant: only its headline and closing note differ.
	granted, err := BuildAccessGrantedNotification("recipient@example.com", sampleNotification(t))
	if err != nil {
		t.Fatalf("build granted notification: %v", err)
	}
	request, err := BuildAccessRequestNotification("john.doe@bcc.no", sampleAccessRequest(t))
	if err != nil {
		t.Fatalf("build access request notification: %v", err)
	}

	for prefix, msg := range map[string]Message{"share": share, "granted": granted, "request": request} {
		for name, body := range map[string]string{prefix + ".html": msg.HTML, prefix + ".txt": msg.Text} {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
			t.Logf("wrote %s", path)
		}
		t.Logf("%s subject: %s", prefix, msg.Subject)
	}
	t.Logf("links point at %s", sampleOrigin())
}
