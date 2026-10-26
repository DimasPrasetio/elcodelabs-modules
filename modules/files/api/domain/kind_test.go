package domain

import (
	"archive/zip"
	"bytes"
	"testing"
)

var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

func zipBytes() []byte {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, _ := w.Create("word/document.xml")
	_, _ = f.Write([]byte("<w:document/>"))
	_ = w.Close()
	return b.Bytes()
}

func TestDetectTrustsTheBytesNotTheName(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		want string
	}{
		{"logo.png", png, "image/png"},
		{"logo.jpg", png, "image/png"}, // the name lies; the bytes win
		{"surat.docx", zipBytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"arsip.zip", zipBytes(), "application/zip"},
		{"data.csv", []byte("a,b\n1,2\n"), "text/csv"},
		{"catatan.txt", []byte("halo"), "text/plain"},
		{"icon.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), "application/octet-stream"},
		{"icon.svg", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`), "text/xml"},
		{"page.html", []byte("<html><script>alert(1)</script></html>"), "text/html"},
		{"evil.png", []byte("<html><script>alert(1)</script></html>"), "text/html"},
		{"script.js", []byte("alert(1)"), "application/octet-stream"},
	}
	for _, c := range cases {
		if got := Detect(c.head, c.name); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
		if c.want == "text/xml" || c.want == "text/html" || c.want == "application/octet-stream" || c.want == "application/zip" {
			if Allowed(Detect(c.head, c.name), DefaultTypes) {
				t.Errorf("%s is allowed by default", c.name)
			}
		}
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{
		"../../etc/passwd":      "passwd",
		`C:\Users\x\nota.pdf`:   "nota.pdf",
		"a\"b\x00c.txt":         "abc.txt",
		"":                      "berkas",
		"  laporan  bulan.pdf ": "laporan  bulan.pdf",
	} {
		if got := CleanName(in); got != want {
			t.Errorf("CleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
