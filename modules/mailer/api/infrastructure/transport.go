package infrastructure

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/modules/mailer/application"
)

// SMTP sends through a mail server — in production a transactional provider on port 587
// (STARTTLS) or 465 (TLS); a VPS's own port 25 is usually blocked and its IP distrusted.
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	// TLS is "starttls" (upgrade a plain connection; refused if the server does not offer it),
	// "tls" (TLS from the first byte, port 465), or "none" (a local test server only).
	TLS string
}

const smtpTimeout = 60 * time.Second

func (t SMTP) Send(ctx context.Context, e application.Envelope) error {
	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
	defer cancel()
	addr := net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	tlsConfig := &tls.Config{ServerName: t.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	var err error
	if t.TLS == "tls" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connect %s: %w", addr, err)
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	// Cancelling ctx (the app shutting down) ends the conversation at once, not at the deadline.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stop()
	c, err := smtp.NewClient(conn, t.Host)
	if err != nil {
		conn.Close()
		return classify(err)
	}
	defer c.Close()
	if t.TLS == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("%s does not offer STARTTLS — set SMTP_TLS=tls for port 465", addr)
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if t.Username != "" {
		// PlainAuth refuses to send the password over an unencrypted connection to a remote host.
		if err := c.Auth(smtp.PlainAuth("", t.Username, t.Password, t.Host)); err != nil {
			return classify(fmt.Errorf("auth: %w", err))
		}
	}
	if err := c.Mail(e.From); err != nil {
		return classify(err)
	}
	if err := c.Rcpt(e.To); err != nil {
		return classify(err)
	}
	w, err := c.Data()
	if err != nil {
		return classify(err)
	}
	msg, err := BuildMessage(e, time.Now())
	if err != nil {
		return application.PermanentError{Err: err}
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return classify(err)
	}
	return c.Quit()
}

// classify marks a 5xx answer as permanent: the server will not accept it however often it is
// tried. Anything else (4xx, network) is worth retrying.
func classify(err error) error {
	var tp *textproto.Error
	if errors.As(err, &tp) && tp.Code >= 500 {
		return application.PermanentError{Err: err}
	}
	return err
}

// BuildMessage renders the email as MIME: plain text, or text and HTML as alternatives.
// Header values are encoded (RFC 2047); bodies are quoted-printable UTF-8 with CRLF lines.
func BuildMessage(e application.Envelope, now time.Time) ([]byte, error) {
	from := mail.Address{Name: e.FromName, Address: e.From}
	to := mail.Address{Name: e.ToName, Address: e.To}
	domain := e.From[strings.LastIndex(e.From, "@")+1:]
	var buf bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	header("From", from.String())
	header("To", to.String())
	header("Subject", mime.QEncoding.Encode("utf-8", e.Subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", fmt.Sprintf("<%d.%s@%s>", e.ID, randomHex(8), domain))
	header("MIME-Version", "1.0")
	if e.HTML == "" {
		header("Content-Type", `text/plain; charset="utf-8"`)
		header("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		if err := writeQP(&buf, e.Text); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	boundary := "alt-" + randomHex(12)
	header("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	buf.WriteString("\r\n")
	for _, part := range []struct{ kind, body string }{{"text/plain", e.Text}, {"text/html", e.HTML}} {
		fmt.Fprintf(&buf, "--%s\r\nContent-Type: %s; charset=\"utf-8\"\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary, part.kind)
		if err := writeQP(&buf, part.body); err != nil {
			return nil, err
		}
		buf.WriteString("\r\n")
	}
	fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	return buf.Bytes(), nil
}

func writeQP(buf *bytes.Buffer, s string) error {
	w := quotedprintable.NewWriter(buf)
	if _, err := w.Write([]byte(strings.ReplaceAll(s, "\r\n", "\n"))); err != nil {
		return err
	}
	return w.Close()
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Log writes email to the log instead of sending it — development without a mail server.
// The module refuses it in production, where a dropped email must never be silent.
type Log struct {
	Logger *slog.Logger
}

func (l Log) Send(_ context.Context, e application.Envelope) error {
	l.Logger.Info("mailer: email written to the log, not sent (no SMTP_HOST)", "to", e.To, "subject", e.Subject, "text", e.Text)
	return nil
}
