package mail

import (
	"os"
	"testing"
)

// TestSendViaRealRelay sends ONE REAL EMAIL through NewFromEnv, so it exercises
// the real TLS mode, auth, envelope sender and templates. Skipped unless
// MAIL_TEST_RECIPIENT is set, so it can't fire by accident:
//
//	set -a; source .env; set +a
//	MAIL_TEST_RECIPIENT=you@gmail.com go test ./internal/mail/ -run RealRelay -v
//
// Use an EXTERNAL address: that's what a corporate relay is likeliest to refuse.
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

	// Match the account actually sending. The fixture's corporate display name
	// over a personal Gmail relay is a phishing signature and gets filtered on
	// that alone, telling you nothing about the real relay.
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
