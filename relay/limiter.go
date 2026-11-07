package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}

type rateEntry struct {
	window time.Time
	count  int
}

func newRateLimiter() *rateLimiter { return &rateLimiter{entries: make(map[string]rateEntry)} }

func (r *rateLimiter) allow(namespace, value string, limit int, window time.Duration, now time.Time) bool {
	if limit <= 0 {
		return true
	}
	key := namespace + ":" + digest(value)
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.entries[key]
	if entry.window.IsZero() || !now.Before(entry.window) {
		entry = rateEntry{window: now.Add(window)}
	}
	if entry.count >= limit {
		r.entries[key] = entry
		return false
	}
	entry.count++
	r.entries[key] = entry
	return true
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}
