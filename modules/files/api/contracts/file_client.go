// Package contracts is what the files module offers other modules (through their own ports and
// an adapter in the composition root).
package contracts

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	// ErrNotFound means no (undeleted) file has that ID.
	ErrNotFound = errors.New("file not found")
	// ErrRejected means the upload was refused — its type or size; the text says which.
	ErrRejected = errors.New("file rejected")
)

// File is what is known about a stored file.
type File struct {
	ID          int64
	Owner       string // the module that stored it
	Name        string
	ContentType string // decided from the content, not the name
	Size        int64
	SHA256      string
	CreatedAt   time.Time
}

// Upload is a file to store. The reader is streamed to disk, never held in memory.
type Upload struct {
	Owner  string // the storing module's id
	Name   string // the original file name, for downloads
	Reader io.Reader
	// MaxBytes caps the size (0: the module's FILES_MAX_BYTES). Allowed narrows the content
	// types (nil: the module's defaults — images, PDF, office documents, text).
	MaxBytes int64
	Allowed  []string
}

// FileClient stores files for other modules. A module serves its own files through its own
// endpoint — it decides who may see them — reading them with Open.
type FileClient interface {
	Save(ctx context.Context, u Upload) (File, error)
	Get(ctx context.Context, id int64) (File, error)
	Open(ctx context.Context, id int64) (File, io.ReadSeekCloser, error)
	// Delete removes the file from use at once; its bytes are purged 30 days later.
	Delete(ctx context.Context, id int64) error
}
