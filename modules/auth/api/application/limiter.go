package application

import (
	"net/netip"
	"sync"
	"time"
)

// Rule limits one key (an email, an address, or both) to Max counted attempts per window.
type Rule struct {
	Key string
	Max int
}

// Limiter slows down password guessing. In memory — one container per app; a restart
// forgets, which only gives an attacker a fresh window.
//
// An attempt is counted before the password is checked (Reserve), so parallel requests cannot
// slip past the limit; a successful sign-in then forgets the keys that belong to that person.
// The number of keys is capped: when a flood of distinct keys fills it, the keys used least
// recently are dropped — memory stays bounded, and a flood never locks everyone out.
type Limiter struct {
	Window  time.Duration
	MaxKeys int
	now     func() time.Time

	mu    sync.Mutex
	hits  map[string][]time.Time
	calls int
}

func NewLimiter(window time.Duration, maxKeys int) *Limiter {
	return &Limiter{Window: window, MaxKeys: maxKeys, now: time.Now, hits: map[string][]time.Time{}}
}

// recent drops key's attempts older than the window and returns the rest.
func (l *Limiter) recent(key string, now time.Time) []time.Time {
	cutoff := now.Add(-l.Window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, key)
		return nil
	}
	l.hits[key] = kept
	return kept
}

// sweep forgets every expired key.
func (l *Limiter) sweep(now time.Time) {
	for key := range l.hits {
		l.recent(key, now)
	}
}

// Reserve counts one attempt against every rule, or none: it refuses when any key is already
// at its limit.
func (l *Limiter) Reserve(rules ...Rule) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if l.calls++; l.calls%256 == 0 || len(l.hits) >= l.MaxKeys {
		l.sweep(now)
	}
	newKeys := 0
	for _, r := range rules {
		hits := l.recent(r.Key, now)
		if len(hits) >= r.Max {
			return false
		}
		if hits == nil {
			newKeys++
		}
	}
	if over := len(l.hits) + newKeys - l.MaxKeys; over > 0 {
		l.evict(over, rules)
	}
	for _, r := range rules {
		l.hits[r.Key] = append(l.hits[r.Key], now)
	}
	return true
}

// evict drops the n keys whose last attempt is oldest, sparing the ones this call is about.
func (l *Limiter) evict(n int, keep []Rule) {
	spare := map[string]bool{}
	for _, r := range keep {
		spare[r.Key] = true
	}
	for ; n > 0; n-- {
		oldest, at := "", time.Time{}
		for k, hits := range l.hits {
			if spare[k] || len(hits) == 0 {
				continue
			}
			if last := hits[len(hits)-1]; oldest == "" || last.Before(at) {
				oldest, at = k, last
			}
		}
		if oldest == "" {
			return
		}
		delete(l.hits, oldest)
	}
}

// Forget clears keys, after a successful sign-in.
func (l *Limiter) Forget(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.hits, k)
	}
}

// ClientKey is the address a limit counts against: an IPv4 address, or an IPv6 /64 — one
// subscriber gets a whole /64, so counting single IPv6 addresses would limit nothing.
func ClientKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil || a.Is4() || a.Is4In6() {
		return ip
	}
	p, err := a.Prefix(64)
	if err != nil {
		return ip
	}
	return p.String()
}
