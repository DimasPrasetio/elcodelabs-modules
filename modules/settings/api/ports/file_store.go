// Package ports is what the settings module needs from outside, in its own words.
package ports

import (
	"context"
	"errors"
	"io"
)

// ErrFileNotFound means the store has no such file.
var ErrFileNotFound = errors.New("file not found")

// ErrFileRejected means the store refused the upload (type or size); its text says why.
var ErrFileRejected = errors.New("file rejected")

// FileStore keeps uploaded files — the company logo. Optional: without it the logo cannot be
// uploaded and the rest of the settings work as before.
type FileStore interface {
	// Save stores r (at most maxBytes, of one of the given content types) and returns its ID.
	Save(ctx context.Context, name string, r io.Reader, maxBytes int64, allowed []string) (int64, error)
	// Open returns the file's content and content type.
	Open(ctx context.Context, id int64) (io.ReadCloser, string, error)
	Delete(ctx context.Context, id int64) error
}
