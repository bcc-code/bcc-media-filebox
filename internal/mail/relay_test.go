package mail

import (
	"os"
	"testing"
)

// TestSendViaRealRelay sends ONE REAL EMAIL through the configured relay,
// using the same NewFromEnv path the server uses — so it exercises the real
// TLS mode, auth, envelope sender and templates, not a test-only shortcut.
//
// It is skipped unless MAIL_TEST_RECIPIENT names an address, so it can never
// fire by accident in CI or a normal `go test ./...`:
//
//	set -a; source .env; set +a
//	MAIL_TEST_RECIPIENT=you@gmail.com go test ./internal/mail/ -run RealRelay -v
//
// Send it to an EXTERNAL address (a personal Gmail): that is the case a
// corporate relay is most likely to refuse, and the one Send depends on.
func TestSendViaRealRelay(t *testing.T) {
	to := os.Getenv("MAIL_TEST_RECIPIENT")
	if to == "" {
		t.Skip("set MAIL_TEST_RECIPIENT=you@example.com to send one real email through the configured relay")
	}

	sender, err := NewFromEnv()
	if err != nil {
		t.Fatalf("mail config: %v", err)
	}
	if !IsEnabled(sender) {
		t.Fatal("MAIL_SMTP_HOST is not set — nothing to test against")
	}

	// Make the sender identity match the account actually sending. The fixture's
	// John Doe <john.doe@bcc.no> over a personal Gmail relay is a phishing
	// signature — free-mail address, corporate display name, off-domain
	// Reply-To — and gets filtered on that alone, telling you nothing about
	// how the real relay will land.
	d := sampleNotification(t)
	if from := os.Getenv("MAIL_FROM_ADDRESS"); from != "" {
		d.SenderEmail = from
		if name := os.Getenv("MAIL_TEST_SENDER_NAME"); name != "" {
			d.SenderName = name
		} else {
			d.SenderName = ""
		}
	}

	msg, err := BuildShareNotification(to, d)
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}

	t.Logf("sending to %s, links point at %s", to, sampleOrigin())
	if err := sender.Send(t.Context(), msg); err != nil {
		// A relay that refuses external recipients fails here, as 550 on RCPT TO.
		t.Fatalf("send failed: %v", err)
	}
	t.Log("accepted by the relay — now check the inbox AND the spam folder,")
	t.Log("then open 'Show original' in Gmail to confirm SPF/DKIM/DMARC pass")
}
