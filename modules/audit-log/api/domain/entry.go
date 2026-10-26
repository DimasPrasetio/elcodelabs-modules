// Package domain holds the audit trail's entry.
package domain

import (
	"encoding/json"
	"time"
)

// Entry is one recorded change.
type Entry struct {
	ID         int64
	OccurredAt time.Time
	ActorID    int64  // 0: no one signed in
	Kind       string // change | access | export | security
	CompanyID  int64
	BranchID   int64
	Module     string
	Action     string
	Target     string
	Summary    string
	Data       json.RawMessage
	IP         string
	RequestID  string
}

// Limits of what is stored; longer values are cut, never refused — losing a record is worse.
const (
	MaxModule  = 40
	MaxAction  = 80
	MaxTarget  = 100
	MaxSummary = 300
	MaxData    = 64 << 10
)

// Cut shortens s to at most n runes.
func Cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
