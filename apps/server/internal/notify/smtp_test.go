package notify

import (
	"bufio"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is a just-enough SMTP server: it records the envelope and the DATA of each mail.
type fakeSMTP struct {
	addr     string
	rejectTo bool

	mu    sync.Mutex
	froms []string
	tos   []string
	datas []string
}

func startFakeSMTP(t *testing.T, rejectTo bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{addr: ln.Addr().String(), rejectTo: rejectTo}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(c)
	say := func(s string) { _, _ = io.WriteString(c, s+"\r\n") }
	say("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			say("250 fake")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			f.mu.Lock()
			f.froms = append(f.froms, strings.TrimSpace(line[len("MAIL FROM:"):]))
			f.mu.Unlock()
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			if f.rejectTo {
				say("550 no such user")
				continue
			}
			f.mu.Lock()
			f.tos = append(f.tos, strings.TrimSpace(line[len("RCPT TO:"):]))
			f.mu.Unlock()
			say("250 ok")
		case cmd == "DATA":
			say("354 go ahead")
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
			f.mu.Lock()
			f.datas = append(f.datas, b.String())
			f.mu.Unlock()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("250 ok")
		}
	}
}

func (f *fakeSMTP) sent() (froms, tos, datas []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.froms...), append([]string(nil), f.tos...), append([]string(nil), f.datas...)
}

func newTestSMTP(t *testing.T, f *fakeSMTP) *SMTP {
	t.Helper()
	s, err := NewSMTP("smtp://"+f.addr, "Tepati <no-reply@tepati.test>")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSMTPSendsAMultipartMail(t *testing.T) {
	f := startFakeSMTP(t, false)
	s := newTestSMTP(t, f)
	err := s.Send(context.Background(), Message{
		To: "bima@tepati.test", ToName: "Bima Sakti", Subject: "Buktimu ditolak — “UTBK”",
		Text: "Halo Bima,\nbukti ditolak. Café ☕", HTML: "<p>Halo Bima, bukti ditolak. Café ☕</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	froms, tos, datas := f.sent()
	if len(datas) != 1 || !strings.Contains(froms[0], "no-reply@tepati.test") || !strings.Contains(tos[0], "bima@tepati.test") {
		t.Fatalf("envelope from=%v to=%v datas=%d", froms, tos, len(datas))
	}
	m, err := mail.ReadMessage(strings.NewReader(datas[0]))
	if err != nil {
		t.Fatal(err)
	}
	dec := new(mime.WordDecoder)
	subject, err := dec.DecodeHeader(m.Header.Get("Subject"))
	if err != nil || subject != "Buktimu ditolak — “UTBK”" {
		t.Fatalf("subject = %q, %v", subject, err)
	}
	if to := m.Header.Get("To"); !strings.Contains(to, "bima@tepati.test") {
		t.Fatalf("To = %q", to)
	}
	for _, h := range []string{"Date", "Message-Id", "From"} {
		if m.Header.Get(h) == "" {
			t.Errorf("missing %s header", h)
		}
	}
	mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/alternative" {
		t.Fatalf("content type = %q, %v", mediaType, err)
	}
	mr := multipart.NewReader(m.Body, params["boundary"])
	var types []string
	var bodies []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(p) // multipart.Reader decodes quoted-printable for us
		types = append(types, strings.SplitN(p.Header.Get("Content-Type"), ";", 2)[0])
		bodies = append(bodies, string(b))
	}
	// Plain text first, HTML last: clients show the last part they understand.
	if len(types) != 2 || types[0] != "text/plain" || types[1] != "text/html" {
		t.Fatalf("parts = %v", types)
	}
	if !strings.Contains(bodies[0], "Café ☕") || !strings.Contains(bodies[1], "Café ☕") {
		t.Fatalf("bodies lost their UTF-8: %q", bodies)
	}
}

func TestSMTPRefusesAddressesThatCouldInjectHeaders(t *testing.T) {
	f := startFakeSMTP(t, false)
	s := newTestSMTP(t, f)
	for _, to := range []string{"a@b.test\r\nBcc: x@evil.test", "not an address", ""} {
		if err := s.Send(context.Background(), Message{To: to, Subject: "x", Text: "x", HTML: "x"}); err == nil {
			t.Errorf("Send accepted recipient %q", to)
		}
	}
	if _, _, datas := f.sent(); len(datas) != 0 {
		t.Fatalf("%d mails reached the server", len(datas))
	}
}

func TestSMTPReportsAServerRefusal(t *testing.T) {
	f := startFakeSMTP(t, true)
	s := newTestSMTP(t, f)
	err := s.Send(context.Background(), Message{To: "x@tepati.test", Subject: "x", Text: "x", HTML: "x"})
	if err == nil || !strings.Contains(err.Error(), "550") {
		t.Fatalf("err = %v, want the 550 refusal", err)
	}
}

func TestSMTPGivesUpWhenTheContextEnds(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() { // accepts, then says nothing
		c, err := ln.Accept()
		if err == nil {
			time.Sleep(3 * time.Second)
			_ = c.Close()
		}
	}()
	s, err := NewSMTP("smtp://"+ln.Addr().String(), "Tepati <no-reply@tepati.test>")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := s.Send(ctx, Message{To: "x@tepati.test", Subject: "x", Text: "x", HTML: "x"}); err == nil {
		t.Fatal("expected an error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("Send ignored the context deadline, took %v", time.Since(start))
	}
}

func TestNewSMTPValidatesItsInput(t *testing.T) {
	for _, tc := range []struct{ url, from string }{
		{"http://localhost:1025", "a@b.test"},
		{"smtp://", "a@b.test"},
		{"smtp://localhost:1025", "not an address"},
	} {
		if _, err := NewSMTP(tc.url, tc.from); err == nil {
			t.Errorf("NewSMTP(%q, %q) succeeded", tc.url, tc.from)
		}
	}
}
