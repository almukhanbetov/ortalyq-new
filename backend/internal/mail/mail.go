// Package mail sends e-mail through a transactional outbox. Callers enqueue a message inside their own database
// transaction (Outbox.Enqueue); the Dispatcher sends it after the commit, with retries and a limit of attempts.
//
// Drivers (MAIL_DRIVER):
//   - log (default): nothing leaves the server; the message is recorded in email_outbox and only its metadata is
//     logged (never the body: it may carry a one-time link);
//   - smtp: delivers to MAIL_SMTP_ADDR. Used against a local test receiver; connecting a real mail server is a
//     separate, explicit configuration step.
package mail

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Message is one e-mail (plain text).
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender delivers a message and returns the provider's id of it.
type Sender interface {
	Send(ctx context.Context, m Message) (string, error)
}

// LogSender records the fact of sending in the log, without the body.
type LogSender struct{}

func (LogSender) Send(_ context.Context, m Message) (string, error) {
	id := "log-" + uuid.NewString()
	slog.Info("mail (log driver, not sent)", "id", id, "to", m.To, "subject", m.Subject, "bytes", len(m.Text))
	return id, nil
}

// SMTPSender delivers through an SMTP server (plain SMTP; STARTTLS is used when the server offers it).
type SMTPSender struct {
	Addr     string // host:port
	From     string
	Username string
	Password string
}

func (s SMTPSender) Send(_ context.Context, m Message) (string, error) {
	id := uuid.NewString()
	host := s.Addr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, host)
	}
	if err := smtp.SendMail(s.Addr, auth, s.From, []string{m.To}, build(s.From, id, m)); err != nil {
		return "", err
	}
	return id, nil
}

// build renders an RFC 5322 message with UTF-8 headers and body.
func build(from, id string, m Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: =?UTF-8?B?%s?=\r\n", from, m.To, b64(m.Subject))
	fmt.Fprintf(&b, "Message-ID: <%s@ortalyq>\r\nDate: %s\r\nMIME-Version: 1.0\r\n", id, time.Now().UTC().Format(time.RFC1123Z))
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	body := b64(m.Text)
	for len(body) > 76 {
		b.WriteString(body[:76] + "\r\n")
		body = body[76:]
	}
	b.WriteString(body + "\r\n")
	return []byte(b.String())
}

// Config selects the driver.
type Config struct {
	Driver   string // log | smtp
	SMTPAddr string
	From     string
	Username string
	Password string
}

// NewSender returns the configured driver; unknown drivers are an error, so a typo never sends real mail.
func NewSender(c Config) (Sender, error) {
	switch c.Driver {
	case "", "log":
		return LogSender{}, nil
	case "smtp":
		if c.SMTPAddr == "" || c.From == "" {
			return nil, errors.New("MAIL_DRIVER=smtp needs MAIL_SMTP_ADDR and MAIL_FROM")
		}
		return SMTPSender{Addr: c.SMTPAddr, From: c.From, Username: c.Username, Password: c.Password}, nil
	}
	return nil, fmt.Errorf("unknown MAIL_DRIVER %q", c.Driver)
}
