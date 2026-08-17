// Package mail delivers transactional email for the Send feature: share
// notifications now, expiry-extension requests later. Callers never choose the
// envelope sender or the From header — those are fixed per deploy so SPF/DKIM
// stay aligned with the relay. The human behind a message rides in ReplyTo.
package mail

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

// Message is one outgoing email. From and the SMTP envelope sender come from
// configuration, not from here.
type Message struct {
	To []string
	// ReplyTo is the address a recipient reaches by hitting reply — the user
	// who shared the files. Optional; omitted when there is no human to answer.
	ReplyTo string
	// ReplyToName is the display name paired with ReplyTo. Optional.
	ReplyToName string
	// SenderName is the human who caused this mail. It becomes the From
	// display name ("John Doe (via FileBox)") while the From address stays
	// the configured service address, so DMARC alignment survives. Optional.
	SenderName string
	Subject    string
	// Text and HTML are the two halves of a multipart/alternative body. Both
	// should be set; a text-only client otherwise sees an empty message.
	Text string
	HTML string
}

// Validate rejects the malformed and the dangerous: a message with no
// recipient, and any header field carrying a newline, which would let
// user-supplied input inject arbitrary SMTP headers.
func (m Message) Validate() error {
	if len(m.To) == 0 {
		return fmt.Errorf("message has no recipients")
	}
	fields := append([]string{m.Subject, m.ReplyTo, m.ReplyToName, m.SenderName}, m.To...)
	for _, f := range fields {
		if strings.ContainsAny(f, "\r\n") {
			return fmt.Errorf("header field contains a newline: %q", f)
		}
	}
	for _, addr := range m.To {
		if strings.TrimSpace(addr) == "" {
			return fmt.Errorf("message has an empty recipient address")
		}
	}
	return nil
}

// Sender delivers a Message. Implementations must be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// NoopSender logs what it would have sent and reports success. It is the
// default whenever MAIL_SMTP_HOST is unset, so dev and CI runs never put mail
// on the wire just because a code path fired.
type NoopSender struct{}

func (NoopSender) Send(_ context.Context, msg Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	log.Printf("mail: not configured, dropping message to %s (subject %q)",
		strings.Join(msg.To, ", "), msg.Subject)
	return nil
}

// NewFromEnv builds a Sender from MAIL_SMTP_* and MAIL_FROM_*.
//
// With MAIL_SMTP_HOST unset it returns a NoopSender rather than an error, so a
// deploy that hasn't got relay credentials yet still boots. When the host IS
// set the remaining config must be coherent — a half-configured relay fails
// loudly at startup instead of silently at the first share.
func NewFromEnv() (Sender, error) {
	host := os.Getenv("MAIL_SMTP_HOST")
	if host == "" {
		log.Println("mail: MAIL_SMTP_HOST unset — email delivery disabled")
		return NoopSender{}, nil
	}

	from := os.Getenv("MAIL_FROM_ADDRESS")
	if from == "" {
		return nil, fmt.Errorf("MAIL_SMTP_HOST is set but MAIL_FROM_ADDRESS is missing")
	}
	fromName := os.Getenv("MAIL_FROM_NAME")
	if fromName == "" {
		fromName = "FileBox"
	}

	port := os.Getenv("MAIL_SMTP_PORT")
	if port == "" {
		port = "587"
	}

	tlsMode := tlsMode(strings.ToLower(os.Getenv("MAIL_SMTP_TLS")))
	switch tlsMode {
	case "":
		tlsMode = tlsSTARTTLS
	case tlsSTARTTLS, tlsNone, tlsImplicit:
	default:
		return nil, fmt.Errorf("MAIL_SMTP_TLS must be one of starttls, implicit, none (got %q)", tlsMode)
	}

	user, pass := os.Getenv("MAIL_SMTP_USER"), os.Getenv("MAIL_SMTP_PASS")
	if user != "" && pass == "" {
		return nil, fmt.Errorf("MAIL_SMTP_USER is set but MAIL_SMTP_PASS is missing")
	}

	s := &SMTPSender{
		Host:     host,
		Port:     port,
		Username: user,
		Password: pass,
		TLS:      tlsMode,
		From:     from,
		FromName: fromName,
	}
	log.Printf("mail: sending via %s:%s (tls=%s) as %q <%s>", host, port, tlsMode, fromName, from)
	return s, nil
}

// IsEnabled reports whether s actually delivers. Callers use it to skip work
// (building links, rendering bodies) when mail is switched off.
func IsEnabled(s Sender) bool {
	_, noop := s.(NoopSender)
	return s != nil && !noop
}
