package domain

import "time"

// Stored is a file's row.
type Stored struct {
	ID          int64
	Owner       string
	Name        string
	ContentType string
	Size        int64
	SHA256      string
	Key         string // where the bytes are, relative to the store's root
	CreatedAt   time.Time
	CreatedBy   int64
	Locked      bool // attached to a posted document: never deleted
}
