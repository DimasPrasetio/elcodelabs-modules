// Package ports is what the audit-log module needs from outside, in its own words.
package ports

import "context"

// UserDirectory names the people in the trail. Optional: without it an actor shows as
// "Pengguna #12".
type UserDirectory interface {
	// Names returns a display name for each ID it knows.
	Names(ctx context.Context, ids []int64) (map[int64]string, error)
}
