// Package mailtest is a minimal SMTP receiver for tests: it accepts messages on 127.0.0.1 and keeps them in
// memory, or rejects recipients to simulate a delivery failure. It never relays anything.
package mailtest

import (
	"bufio"
	"encoding/base64"
	"net"
	"strings"
	"sync"
)

type Received struct {
	From, To string
	Data     string
}

// Body returns the decoded text body (base64 part after the headers).
func (r Received) Body() string {
	i := strings.Index(r.Data, "\r\n\r\n")
	if i < 0 {
		return ""
	}
	b, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimSpace(r.Data[i+4:]), "\r\n", ""))
	return string(b)
}

type Server struct {
	ln   net.Listener
	mu   sync.Mutex
	msgs []Received
	fail bool
}

// Start listens on a free local port.
func Start() (*Server, error) { return StartAt("127.0.0.1:0") }

// StartAt listens on the given local address (e.g. 127.0.0.1:2525 for the dev receiver).
func StartAt(addr string) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln}
	go s.serve()
	return s, nil
}

func (s *Server) Addr() string { return s.ln.Addr().String() }
func (s *Server) Close() error { return s.ln.Close() }

// Fail makes the receiver reject recipients (550) until Fail(false).
func (s *Server) Fail(on bool) { s.mu.Lock(); s.fail = on; s.mu.Unlock() }

func (s *Server) Messages() []Received {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Received(nil), s.msgs...)
}

func (s *Server) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	w := func(line string) { _, _ = c.Write([]byte(line + "\r\n")) }
	w("220 mailtest ready")
	var cur Received
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			w("250 mailtest")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			cur = Received{From: strings.TrimSpace(line[10:])}
			w("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			s.mu.Lock()
			fail := s.fail
			s.mu.Unlock()
			if fail {
				w("550 mailbox unavailable")
				continue
			}
			cur.To = strings.Trim(strings.TrimSpace(line[8:]), "<>")
			w("250 ok")
		case cmd == "DATA":
			w("354 end with .")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			cur.Data = b.String()
			s.mu.Lock()
			s.msgs = append(s.msgs, cur)
			s.mu.Unlock()
			w("250 queued")
		case cmd == "QUIT":
			w("221 bye")
			return
		case cmd == "RSET", cmd == "NOOP":
			w("250 ok")
		default:
			w("502 not implemented")
		}
	}
}
