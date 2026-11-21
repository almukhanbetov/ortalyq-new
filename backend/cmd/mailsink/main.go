// Command mailsink is a local SMTP test receiver for development and browser checks: it accepts letters on
// 127.0.0.1 only, never relays them, and writes each one (decoded text body) to a file in -dir.
//
//	go run ./cmd/mailsink -addr 127.0.0.1:2525 -dir /tmp/mailsink
//	MAIL_DRIVER=smtp MAIL_SMTP_ADDR=127.0.0.1:2525 MAIL_FROM=noreply@ortalyq.local go run ./cmd/api serve
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ortalyq/backend/internal/mail/mailtest"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:2525", "listen address (local only)")
	dir := flag.String("dir", "./data/mailsink", "where to write received letters")
	flag.Parse()
	if !strings.HasPrefix(*addr, "127.0.0.1:") && !strings.HasPrefix(*addr, "localhost:") {
		log.Fatal("mailsink listens on 127.0.0.1 only")
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatal(err)
	}
	s, err := mailtest.StartAt(*addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("mailsink on %s, letters in %s", s.Addr(), *dir)
	written := 0
	for {
		msgs := s.Messages()
		for ; written < len(msgs); written++ {
			m := msgs[written]
			name := filepath.Join(*dir, fmt.Sprintf("%04d-%s.txt", written+1, strings.NewReplacer("@", "_at_", "/", "_").Replace(m.To)))
			body := fmt.Sprintf("To: %s\nFrom: %s\n\n%s", m.To, m.From, m.Body())
			if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
				log.Print(err)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}
