package mail_test

import (
	"strings"
	"testing"

	"ortalyq/backend/internal/mail"
	"ortalyq/backend/internal/mail/mailtest"
)

func TestInvitationTemplates(t *testing.T) {
	d := mail.InvitationData{To: "a@b.kz", Student: "Данияр", Teacher: "Айгерим", Marathon: "Марафон", Link: "http://x/invite/T", Expires: "28.11.2026 15:00 (UTC+05:00)"}
	for _, l := range append(mail.Locales, "de") {
		m := mail.Invitation(l, d)
		if m.To != "a@b.kz" || !strings.Contains(m.Subject, "Марафон") || !strings.Contains(m.Text, "http://x/invite/T") ||
			!strings.Contains(m.Text, "28.11.2026") || strings.Contains(m.Text, "{") {
			t.Fatalf("%s: %+v", l, m)
		}
	}
	if !strings.Contains(mail.Invitation("kk", d).Text, "Сәлеметсіз") || !strings.Contains(mail.Invitation("en", d).Text, "Hello") ||
		!strings.Contains(mail.Invitation("de", d).Text, "Здравствуйте") {
		t.Fatal("languages")
	}
}

func TestSMTPSenderWithTestReceiver(t *testing.T) {
	sink, err := mailtest.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	s, err := mail.NewSender(mail.Config{Driver: "smtp", SMTPAddr: sink.Addr(), From: "noreply@ortalyq.test"})
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("Длинный текст письма с кириллицей. ", 20)
	id, err := s.Send(t.Context(), mail.Message{To: "student@ortalyq.test", Subject: "Тема «тест»", Text: text})
	if err != nil || id == "" {
		t.Fatal(id, err)
	}
	msgs := sink.Messages()
	if len(msgs) != 1 || msgs[0].To != "student@ortalyq.test" || msgs[0].Body() != text || !strings.Contains(msgs[0].Data, "Subject: =?UTF-8?B?") {
		t.Fatalf("received: %+v", msgs)
	}
	sink.Fail(true)
	if _, err := s.Send(t.Context(), mail.Message{To: "x@y.z", Subject: "s", Text: "t"}); err == nil || !strings.Contains(err.Error(), "550") {
		t.Fatalf("rejected recipient must fail: %v", err)
	}
}

func TestNewSender(t *testing.T) {
	if s, err := mail.NewSender(mail.Config{}); err != nil || s == nil {
		t.Fatal("log is the default")
	}
	if _, err := mail.NewSender(mail.Config{Driver: "smtp"}); err == nil {
		t.Fatal("smtp without an address must fail")
	}
	if _, err := mail.NewSender(mail.Config{Driver: "sendgrid"}); err == nil {
		t.Fatal("unknown driver must fail")
	}
}
