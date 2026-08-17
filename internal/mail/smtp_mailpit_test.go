package mail

import (
	"net"
	"os"
	"strconv"
	"testing"
)

// TestSendAgainstMailpit is a manual smoke test against a local Mailpit
// (see development.md). Skipped unless MAILPIT_SMTP is set:
//
//	MAILPIT_SMTP=localhost:1025 go test ./internal/mail/ -run Mailpit -v
//
// Then open http://localhost:8025 and check both the HTML and text bodies.
func TestSendAgainstMailpit(t *testing.T) {
	addr := os.Getenv("MAILPIT_SMTP")
	if addr == "" {
		t.Skip("set MAILPIT_SMTP=localhost:1025 to run this against a local Mailpit")
	}
	// IPv6 literals need the bracket form, e.g. MAILPIT_SMTP="[::1]:1025".
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("MAILPIT_SMTP must be host:port, got %q: %v", addr, err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatalf("MAILPIT_SMTP port must be numeric, got %q", port)
	}

	s := &SMTPSender{Host: host, Port: port, TLS: tlsNone, From: "filebox@bcc.no", FromName: "FileBox"}
	msg, err := BuildShareNotification("recipient@example.com", sampleNotification(t))
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	if err := s.Send(t.Context(), msg); err != nil {
		t.Fatalf("send via mailpit: %v", err)
	}
	t.Log("sent — inspect it at http://localhost:8025")
}
