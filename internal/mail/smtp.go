package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strings"
	"time"
)

type tlsMode string

const (
	tlsSTARTTLS tlsMode = "starttls" // plain connect, upgrade via STARTTLS (port 587)
	tlsImplicit tlsMode = "implicit" // TLS from the first byte (port 465)
	tlsNone     tlsMode = "none"     // cleartext; local relays and Mailpit only
)

const dialTimeout = 30 * time.Second

// SMTPSender delivers through a single relay. It opens a fresh connection per
// message: share notifications are low-volume and a pooled connection to a
// corporate relay tends to get closed out from under us anyway.
type SMTPSender struct {
	Host     string
	Port     string
	Username string
	Password string
	TLS      tlsMode
	// From is both the envelope sender (Return-Path, what SPF checks) and the
	// From header address. Never a user's own address.
	From     string
	FromName string
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	body, err := s.build(msg)
	if err != nil {
		return fmt.Errorf("build message: %w", err)
	}

	c, err := s.dial(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if s.TLS == tlsSTARTTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp %s: server does not advertise STARTTLS (set MAIL_SMTP_TLS=none if this relay is genuinely plaintext)", s.addr())
		}
		if err := c.StartTLS(&tls.Config{ServerName: s.Host}); err != nil {
			return fmt.Errorf("smtp %s: starttls: %w", s.addr(), err)
		}
	}

	if s.Username != "" {
		// PlainAuth refuses to send credentials over an unencrypted link, which
		// is the behaviour we want everywhere except a local test relay.
		ok, mechanisms := c.Extension("AUTH")
		if !ok {
			return fmt.Errorf("smtp %s: credentials configured but server does not offer AUTH", s.addr())
		}
		// Only PLAIN is implemented. Naming what the relay actually offers
		// turns an opaque 535 into a diagnosis on the first real attempt.
		if !strings.Contains(strings.ToUpper(mechanisms), "PLAIN") {
			return fmt.Errorf("smtp %s: relay offers AUTH %s but only PLAIN is implemented", s.addr(), mechanisms)
		}
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return fmt.Errorf("smtp %s: auth failed (relay offers AUTH %s): %w", s.addr(), mechanisms, err)
		}
	}

	if err := c.Mail(s.From); err != nil {
		return fmt.Errorf("smtp %s: MAIL FROM %s: %w", s.addr(), s.From, err)
	}
	for _, rcpt := range msg.To {
		if err := c.Rcpt(rcpt); err != nil {
			// A relay that refuses external recipients surfaces here, as 550.
			return fmt.Errorf("smtp %s: RCPT TO %s: %w", s.addr(), rcpt, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp %s: DATA: %w", s.addr(), err)
	}
	if _, err := w.Write(body); err != nil {
		w.Close()
		return fmt.Errorf("smtp %s: write body: %w", s.addr(), err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp %s: close body: %w", s.addr(), err)
	}
	return c.Quit()
}

func (s *SMTPSender) addr() string { return net.JoinHostPort(s.Host, s.Port) }

func (s *SMTPSender) dial(ctx context.Context) (*smtp.Client, error) {
	d := &net.Dialer{Timeout: dialTimeout}

	var conn net.Conn
	var err error
	if s.TLS == tlsImplicit {
		conn, err = (&tls.Dialer{NetDialer: d, Config: &tls.Config{ServerName: s.Host}}).DialContext(ctx, "tcp", s.addr())
	} else {
		conn, err = d.DialContext(ctx, "tcp", s.addr())
	}
	if err != nil {
		return nil, fmt.Errorf("smtp dial %s: %w", s.addr(), err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("smtp handshake %s: %w", s.addr(), err)
	}
	return c, nil
}

// build renders RFC 5322 bytes: headers plus a multipart/alternative body so
// every client gets a body it can render.
func (s *SMTPSender) build(msg Message) ([]byte, error) {
	boundary, err := randomToken()
	if err != nil {
		return nil, err
	}
	msgID, err := s.messageID()
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	writeHeader(&b, "From", (&netmail.Address{Name: s.fromDisplay(msg), Address: s.From}).String())
	writeHeader(&b, "To", formatAddressList(msg.To))
	if msg.ReplyTo != "" {
		writeHeader(&b, "Reply-To", (&netmail.Address{Name: msg.ReplyToName, Address: msg.ReplyTo}).String())
	}
	writeHeader(&b, "Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader(&b, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&b, "Message-ID", msgID)
	writeHeader(&b, "MIME-Version", "1.0")
	writeHeader(&b, "Auto-Submitted", "auto-generated")
	writeHeader(&b, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
	b.WriteString("\r\n")

	// Plain text first: least capable renderer wins ties in every mail client.
	for _, part := range []struct{ ctype, content string }{
		{"text/plain; charset=utf-8", msg.Text},
		{"text/html; charset=utf-8", msg.HTML},
	} {
		if part.content == "" {
			continue
		}
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		writeHeader(&b, "Content-Type", part.ctype)
		writeHeader(&b, "Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		qp := quotedprintable.NewWriter(&b)
		if _, err := qp.Write([]byte(normalizeNewlines(part.content))); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)

	return b.Bytes(), nil
}

// fromDisplay renders "John Doe (via FileBox)" when a human triggered the
// mail, and the bare service name otherwise.
func (s *SMTPSender) fromDisplay(msg Message) string {
	if msg.SenderName == "" {
		return s.FromName
	}
	return fmt.Sprintf("%s (via %s)", msg.SenderName, s.FromName)
}

func (s *SMTPSender) messageID() (string, error) {
	tok, err := randomToken()
	if err != nil {
		return "", err
	}
	domain := s.Host
	if _, d, ok := strings.Cut(s.From, "@"); ok {
		domain = d
	}
	return fmt.Sprintf("<%s@%s>", tok, domain), nil
}

// formatAddressList renders recipients through netmail.Address so display
// names and non-ASCII are encoded the same way as From/Reply-To. Entries that
// don't parse are passed through unchanged; the server rejects them if invalid.
func formatAddressList(addrs []string) string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if parsed, err := netmail.ParseAddress(a); err == nil {
			if parsed.Name == "" {
				out = append(out, parsed.Address)
			} else {
				out = append(out, parsed.String())
			}
			continue
		}
		out = append(out, a)
	}
	return strings.Join(out, ", ")
}

func writeHeader(b *bytes.Buffer, name, value string) {
	fmt.Fprintf(b, "%s: %s\r\n", name, value)
}

// normalizeNewlines converts to CRLF without doubling the \r in input that is
// already CRLF — templates produce LF, but callers may pass either.
func normalizeNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}

func randomToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}
