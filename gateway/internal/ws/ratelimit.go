package ws

import (
	"sync"
	"time"
)

// rateLimiter is a simple per-connection token bucket: capacity == refill rate
// per second, refilled continuously. It bounds how fast a single client can
// submit frames without pulling in an external dependency.
type rateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	last     time.Time
}

func newRateLimiter(perSec int) *rateLimiter {
	if perSec <= 0 {
		perSec = 20
	}
	r := float64(perSec)
	return &rateLimiter{tokens: r, capacity: r, rate: r, last: time.Now()}
}

// allow consumes one token, refilling for elapsed time first. Returns false when
// the bucket is empty.
func (l *rateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.last).Seconds()
	l.last = now
	l.tokens += elapsed * l.rate
	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
