package infrastructure

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/modules/mailer/application"
)

func TestBuildMessage(t *testing.T) {
	msg, err := BuildMessage(application.Envelope{ID: 3, From: "noreply@app.test", FromName: "Toko Maju", To: "ana@example.com", ToName: "Ana Ságara",
		Subject: "Atur ulang kata sandi — Toko Maju", Text: "Baris satu\nBaris dua", HTML: "<p>Halo</p>"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := string(msg)
	for _, want := range []string{"From: \"Toko Maju\" <noreply@app.test>\r\n", "Subject: =?utf-8?q?", "multipart/alternative", "Content-Type: text/html", "Message-ID: <3."} {
		if !strings.Contains(s, want) {
			t.Errorf("message lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Error("a bare LF: every line must end in CRLF")
	}
}

// fakeServer is the smallest SMTP server that can take a message, rejecting RCPT with code
// reject when it is set.
func fakeServer(t *testing.T, reject string) (addr string, got chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	got = make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		defer ln.Close()
		r := bufio.NewReader(conn)
		say := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		say("220 fake")
		var data strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "MAIL"):
				say("250 ok")
			case strings.HasPrefix(cmd, "RCPT"):
				if reject != "" {
					say(reject)
				} else {
					say("250 ok")
				}
			case cmd == "DATA":
				say("354 go")
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
					data.WriteString(l)
				}
				say("250 queued")
				got <- data.String()
			case cmd == "QUIT":
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().String(), got
}

func hostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, port, _ := net.SplitHostPort(addr)
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return host, p
}

func TestSMTPDelivers(t *testing.T) {
	addr, got := fakeServer(t, "")
	host, p := hostPort(t, addr)
	err := SMTP{Host: host, Port: p, TLS: "none"}.Send(context.Background(), application.Envelope{From: "noreply@app.test", To: "ana@example.com", Subject: "Halo", Text: "Isi pesan teks"})
	if err != nil {
		t.Fatal(err)
	}
	if body := <-got; !strings.Contains(body, "Subject: Halo") || !strings.Contains(body, "\r\n\r\nIsi pesan teks") {
		t.Fatalf("a plain-text message must arrive with its body; server got:\n%s", body)
	}
}

func TestShutdownCutsASlowServerShort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = conn.Write([]byte("220 fake\r\n"))
			time.Sleep(5 * time.Second) // never answers EHLO
		}
	}()
	host, p := hostPort(t, ln.Addr().String())
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()
	_ = SMTP{Host: host, Port: p, TLS: "none"}.Send(ctx, application.Envelope{From: "noreply@app.test", To: "a@example.com", Subject: "s", Text: "t"})
	if waited := time.Since(start); waited > 2*time.Second {
		t.Fatalf("Send ignored the cancellation for %v", waited)
	}
}

func TestSMTPRefusalIsPermanent(t *testing.T) {
	addr, _ := fakeServer(t, "550 no such user")
	host, p := hostPort(t, addr)
	err := SMTP{Host: host, Port: p, TLS: "none"}.Send(context.Background(), application.Envelope{From: "noreply@app.test", To: "nobody@example.com", Subject: "s", Text: "t"})
	if !errors.As(err, new(application.PermanentError)) {
		t.Fatalf("want a permanent error for 550, got %v", err)
	}
}

func TestStartTLSIsRequiredWhenAsked(t *testing.T) {
	addr, _ := fakeServer(t, "")
	host, p := hostPort(t, addr)
	err := SMTP{Host: host, Port: p, TLS: "starttls"}.Send(context.Background(), application.Envelope{From: "noreply@app.test", To: "a@example.com", Subject: "s", Text: "t"})
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("a server without STARTTLS must be refused, got %v", err)
	}
}
