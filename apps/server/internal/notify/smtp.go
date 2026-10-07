package notify

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

// sendTimeout bounds one delivery when the caller's context has no deadline.
const sendTimeout = 30 * time.Second

// SMTP sends mail through one SMTP server. SMTP_URL picks it: smtp://host:port speaks plain
// SMTP and upgrades with STARTTLS when the server offers it (Mailpit offers nothing, which
// is what we want in dev); smtps://host:port uses implicit TLS; user:password@ adds PLAIN auth.
type SMTP struct {
	addr, host string
	implicit   bool
	auth       smtp.Auth
	from       mail.Address
}

func NewSMTP(rawURL, from string) (*SMTP, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("SMTP_URL: %w", err)
	}
	if u.Scheme != "smtp" && u.Scheme != "smtps" {
		return nil, fmt.Errorf("SMTP_URL: scheme must be smtp or smtps, got %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, errors.New("SMTP_URL: missing host")
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"smtp": "25", "smtps": "465"}[u.Scheme]
	}
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("MAIL_FROM: %w", err)
	}
	s := &SMTP{addr: net.JoinHostPort(u.Hostname(), port), host: u.Hostname(), implicit: u.Scheme == "smtps", from: *addr}
	if u.User != nil {
		pass, _ := u.User.Password()
		s.auth = smtp.PlainAuth("", u.User.Username(), pass, s.host)
	}
	return s, nil
}

// Send delivers one message and returns when the server accepted it or the context ended.
func (s *SMTP) Send(ctx context.Context, m Message) error {
	to, err := mail.ParseAddress(m.To)
	if err != nil || to.Name != "" || strings.ContainsAny(m.To, "\r\n") {
		return fmt.Errorf("smtp: bad recipient %q", m.To)
	}
	to.Name = oneLine(m.ToName)
	body, err := s.build(*to, m)
	if err != nil {
		return err
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, sendTimeout)
		defer cancel()
	}
	conn, err := s.dial(ctx)
	if err != nil {
		return fmt.Errorf("smtp: connect %s: %w", s.addr, err)
	}
	// Closing the connection is how a cancelled context interrupts a blocked read.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp: greeting: %w", err)
	}
	defer c.Close()
	if !s.implicit {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("smtp: starttls: %w", err)
			}
		}
	}
	if s.auth != nil {
		if err := c.Auth(s.auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fmt.Errorf("smtp: RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: DATA: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: send: %w", err)
	}
	_ = c.Quit()
	return nil
}

func (s *SMTP) dial(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return nil, err
	}
	if !s.implicit {
		return conn, nil
	}
	tc := tls.Client(conn, &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12})
	if err := tc.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tc, nil
}

// build writes the RFC 5322 message: headers, then plain text and HTML as multipart/alternative.
// The plain part comes first because mail clients show the last alternative they understand.
func (s *SMTP) build(to mail.Address, m Message) ([]byte, error) {
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	var out bytes.Buffer
	hdr := func(k, v string) { out.WriteString(k + ": " + v + "\r\n") }
	hdr("From", s.from.String())
	hdr("To", to.String())
	hdr("Subject", mime.QEncoding.Encode("utf-8", oneLine(m.Subject)))
	hdr("Date", time.Now().UTC().Format(time.RFC1123Z))
	hdr("Message-ID", "<"+hex.EncodeToString(id[:])+"@"+senderDomain(s.from.Address)+">")
	hdr("MIME-Version", "1.0")
	hdr("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	out.WriteString("\r\n")

	for _, part := range []struct{ typ, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.typ + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(pw)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	out.Write(buf.Bytes())
	return out.Bytes(), nil
}

func senderDomain(addr string) string {
	if i := strings.LastIndex(addr, "@"); i >= 0 && i < len(addr)-1 {
		return addr[i+1:]
	}
	return "tepati.local"
}
