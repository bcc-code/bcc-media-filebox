package mail

import (
	"strings"
	"testing"
	"time"
)

func testSender() *SMTPSender {
	return &SMTPSender{Host: "relay.bcc.no", Port: "587", From: "filebox@bcc.no", FromName: "FileBox"}
}

func testData() ShareNotification {
	return ShareNotification{
		SenderName:  "John Doe",
		SenderEmail: "john.doe@bcc.no",
		PackageName: "Summer conference rushes",
		Message:     "Here are the files we talked about.",
		ShareURL:    "https://filebox.example.com/s/abc123",
		Files: []NotificationFile{
			{Name: "opening-wide.mov", Size: 4823400000},
			{Name: "interview-a.mov", Size: 1200000},
			{Name: "notes.pdf", Size: 820},
		},
		MaxDownloads: 3,
		ExpiresAt:    time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestValidateRejectsHeaderInjection(t *testing.T) {
	cases := map[string]Message{
		"subject":   {To: []string{"a@b.no"}, Subject: "hi\r\nBcc: victim@x.no"},
		"recipient": {To: []string{"a@b.no\nBcc: victim@x.no"}},
		"reply-to":  {To: []string{"a@b.no"}, ReplyTo: "a@b.no\r\nBcc: victim@x.no"},
		"sender":    {To: []string{"a@b.no"}, SenderName: "Jan\nBcc: victim@x.no"},
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			if err := msg.Validate(); err == nil {
				t.Fatal("expected newline in header field to be rejected")
			}
		})
	}
}

func TestValidateRejectsEmptyRecipients(t *testing.T) {
	if err := (Message{Subject: "x"}).Validate(); err == nil {
		t.Fatal("expected a message with no recipients to be rejected")
	}
	if err := (Message{To: []string{"  "}}).Validate(); err == nil {
		t.Fatal("expected an empty recipient address to be rejected")
	}
}

// The envelope/From/Reply-To split is the whole point of the design: the
// service address must own From, and the human must own Reply-To.
func TestBuildHeaderSplit(t *testing.T) {
	msg, err := BuildShareNotification("recipient@gmail.com", testData())
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	raw, err := testSender().build(msg)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	headers, _, _ := strings.Cut(string(raw), "\r\n\r\n")

	for _, want := range []string{
		`From: "John Doe (via FileBox)" <filebox@bcc.no>`,
		`Reply-To: "John Doe" <john.doe@bcc.no>`,
		"To: recipient@gmail.com",
		"Auto-Submitted: auto-generated",
	} {
		if !strings.Contains(headers, want) {
			t.Errorf("headers missing %q\ngot:\n%s", want, headers)
		}
	}
	if strings.Contains(headers, "From: john.doe@bcc.no") {
		t.Error("the human's address must never be the From address")
	}
}

func TestBuildFallsBackToServiceNameWithoutSender(t *testing.T) {
	raw, err := testSender().build(Message{To: []string{"a@b.no"}, Subject: "x", Text: "y"})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	if !strings.Contains(string(raw), `From: "FileBox" <filebox@bcc.no>`) {
		t.Errorf("expected bare service name in From, got:\n%s", raw)
	}
}

func TestBuildIsMultipartWithBothBodies(t *testing.T) {
	msg, err := BuildShareNotification("recipient@gmail.com", testData())
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	raw, err := testSender().build(msg)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	got := string(raw)

	for _, want := range []string{
		"Content-Type: multipart/alternative",
		"text/plain; charset=utf-8",
		"text/html; charset=utf-8",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("message missing %q", want)
		}
	}
	if strings.Contains(got, "\n") && strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Error("message contains a bare LF; SMTP requires CRLF line endings")
	}
}

func TestNotificationBodiesCarryLinkAndExpiry(t *testing.T) {
	msg, err := BuildShareNotification("recipient@gmail.com", testData())
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	for name, body := range map[string]string{"text": msg.Text, "html": msg.HTML} {
		for _, want := range []string{
			"https://filebox.example.com/s/abc123", // the link
			"1 September 2026",                     // absolute expiry
			"opening-wide.mov",                     // file list
			"4.49 GB",                              // per-file size, fmtBytes-compatible
			"3 files",
			"3 downloads allowed",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s body is missing %q:\n%s", name, want, body)
			}
		}
	}
	if msg.Subject != "John Doe sent you a package: Summer conference rushes" {
		t.Errorf("unexpected subject: %q", msg.Subject)
	}
}

// The mail must not drift from the recipient page it links to: both use the
// same wording and the same resolved send.css tokens.
func TestNotificationMatchesRecipientPageDesign(t *testing.T) {
	msg, err := BuildShareNotification("recipient@gmail.com", testData())
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	for _, want := range []string{
		"sent you a package",       // PackageDownloadScreen.vue's .public-from
		"background-color:#0f1c33", // --bg-2, the .public-card surface
		"#2a3d61",                  // --line-2 card border
		"#e7ecf5",                  // --ink
		"#6c7896",                  // --ink-3
		"#73a9e1",                  // --accent, oklch(0.72 0.10 250)
		"color:#0a1426",            // --accent-ink on the primary button
	} {
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("HTML body is missing design token %q", want)
		}
	}
	// oklch() and custom properties do not survive any mail client.
	for _, banned := range []string{"oklch(", "color-mix(", "var(--"} {
		if strings.Contains(msg.HTML, banned) {
			t.Errorf("HTML body contains %q, which no mail client resolves", banned)
		}
	}
}

// Files is optional so a caller that hasn't loaded the shares can still send.
func TestNotificationWithoutFileListFallsBackToCount(t *testing.T) {
	d := testData()
	d.Files = nil
	d.FileCount = 2
	msg, err := BuildShareNotification("recipient@gmail.com", d)
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	for name, body := range map[string]string{"text": msg.Text, "html": msg.HTML} {
		if !strings.Contains(body, "Download 2 files") {
			t.Errorf("%s body should fall back to the file count:\n%s", name, body)
		}
		if strings.Contains(body, "0 B") {
			t.Errorf("%s body prints a bogus zero total when sizes are unknown", name)
		}
	}
}

func TestFormatBytesMatchesFrontend(t *testing.T) {
	cases := map[int64]string{
		0:          "0 B",
		820:        "820 B",
		1024:       "1.00 KB",
		1200000:    "1.14 MB",
		4823400000: "4.49 GB",
	}
	for in, want := range cases {
		if got := FormatBytes(in); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

// A sender's free-text note reaches the HTML body, so it must be escaped.
func TestNotificationEscapesSenderMessage(t *testing.T) {
	d := testData()
	d.Message = `<script>alert("xss")</script>`
	msg, err := BuildShareNotification("recipient@gmail.com", d)
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	if strings.Contains(msg.HTML, "<script>") {
		t.Errorf("sender message was not escaped in the HTML body:\n%s", msg.HTML)
	}
}

func TestShareURL(t *testing.T) {
	cases := []struct{ base, want string }{
		{"https://filebox.example.com", "https://filebox.example.com/s/abc123"},
		{"https://filebox.example.com/", "https://filebox.example.com/s/abc123"},
		{"http://localhost:8080", "http://localhost:8080/s/abc123"},
	}
	for _, c := range cases {
		got, err := ShareURL(c.base, "abc123")
		if err != nil {
			t.Fatalf("ShareURL(%q): %v", c.base, err)
		}
		if got != c.want {
			t.Errorf("ShareURL(%q) = %q, want %q", c.base, got, c.want)
		}
	}
	for _, bad := range []string{"", "filebox.example.com", "/relative"} {
		if _, err := ShareURL(bad, "abc123"); err == nil {
			t.Errorf("ShareURL(%q): expected an error for a non-absolute base URL", bad)
		}
	}
}

func TestNoopSenderStillValidates(t *testing.T) {
	if err := (NoopSender{}).Send(t.Context(), Message{}); err == nil {
		t.Fatal("expected NoopSender to reject an invalid message")
	}
	if err := (NoopSender{}).Send(t.Context(), Message{To: []string{"a@b.no"}}); err != nil {
		t.Fatalf("expected NoopSender to accept a valid message: %v", err)
	}
}

func TestLogoURL(t *testing.T) {
	cases := map[string]string{
		"https://filebox.example.com":  "https://filebox.example.com/logo-email.png",
		"https://filebox.example.com/": "https://filebox.example.com/logo-email.png",
		"http://localhost:8091":        "http://localhost:8091/logo-email.png",
		"":                             "", // falls back to the wordmark
		"filebox.example.com":          "",
	}
	for base, want := range cases {
		if got := LogoURL(base); got != want {
			t.Errorf("LogoURL(%q) = %q, want %q", base, got, want)
		}
	}
}

// The mark is decorative: blocked images must leave the wordmark, not a broken
// image with duplicate branding.
func TestNotificationLogoIsOptionalAndDecorative(t *testing.T) {
	d := testData()
	d.LogoURL = "https://filebox.example.com/logo-email.png"
	withLogo, err := BuildShareNotification("recipient@gmail.com", d)
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	if !strings.Contains(withLogo.HTML, `src="https://filebox.example.com/logo-email.png"`) {
		t.Error("HTML body is missing the logo image")
	}
	if !strings.Contains(withLogo.HTML, `alt=""`) {
		t.Error("the logo must have empty alt text — the wordmark already names the product")
	}
	if !strings.Contains(withLogo.HTML, `width="23" height="26"`) {
		t.Error("the logo needs width/height attributes; Outlook ignores CSS sizing")
	}

	d.LogoURL = ""
	noLogo, err := BuildShareNotification("recipient@gmail.com", d)
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	if strings.Contains(noLogo.HTML, "<img") {
		t.Error("without a logo URL the mail must contain no images at all")
	}
	if !strings.Contains(noLogo.HTML, ">FileBox<") {
		t.Error("the wordmark must survive as the fallback branding")
	}
}
