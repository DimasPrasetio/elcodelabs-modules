// Package domain holds the files module's rules: which content is accepted, and what a stored
// name may contain.
package domain

import (
	"net/http"
	"path"
	"strings"
	"unicode"
)

// DefaultTypes are what an upload may be when its module does not narrow it. Never SVG or HTML:
// served from the app's origin they could run script.
var DefaultTypes = []string{
	"image/png", "image/jpeg", "image/webp", "image/gif",
	"application/pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"text/plain", "text/csv",
}

var officeByExt = map[string]string{
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

// Detect decides a file's content type from its first bytes. A name only narrows what the
// bytes already say: an Office document is a ZIP whose extension names the kind, and plain
// text is text/plain or (.csv) text/csv. Anything else the bytes do not identify is
// application/octet-stream, which no default allows.
func Detect(head []byte, name string) string {
	sniffed := http.DetectContentType(head)
	if i := strings.IndexByte(sniffed, ';'); i >= 0 {
		sniffed = sniffed[:i]
	}
	ext := strings.ToLower(path.Ext(name))
	switch {
	case sniffed == "application/zip" && officeByExt[ext] != "":
		return officeByExt[ext]
	case sniffed == "text/plain" && ext == ".csv":
		return "text/csv"
	case sniffed == "text/plain" && (ext == ".txt" || ext == ""):
		return "text/plain"
	case sniffed == "text/plain":
		// Text with another extension (.svg, .html, .js, …) is refused rather than trusted.
		return "application/octet-stream"
	}
	return sniffed
}

// Allowed reports whether kind is in the list.
func Allowed(kind string, list []string) bool {
	for _, k := range list {
		if k == kind {
			return true
		}
	}
	return false
}

// CleanName keeps a file name safe to store and to put in a download header: no path, no
// control characters, at most 200 characters.
func CleanName(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		name = "berkas"
	}
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	return name
}
