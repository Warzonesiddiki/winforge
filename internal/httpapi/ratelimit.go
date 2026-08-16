package httpapi

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// rateLimiter is a simple token-bucket rate limiter. It is defense-in-depth:
// the loopback bind, session token, and same-origin checks already restrict
// who can mutate system state, but a misbehaving or compromised local process
// could still flood the dashboard API. The limiter bounds mutating requests
// per source identity without any third-party dependency.
//
// The bucket is keyed by the remote address (from http.Request.RemoteAddr)
// so a process that opens many connections cannot exhaust a shared budget.
type rateLimiter struct {
	mu       sync.Mutex
	rate     int // tokens added per second
	burst    int // maximum tokens
	buckets  map[string]*bucket
	now      func() time.Time // injectable clock for tests
	lastTick time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter returns a limiter that allows up to rate requests per second
// with a short-term burst of burst.
func newRateLimiter(rate, burst int) *rateLimiter {
	return &rateLimiter{
		rate:    rate,
		burst:   burst,
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

// allow reports whether a request from key is permitted.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	// Lazily sweep stale entries roughly once per refill interval so the
	// map cannot grow without bound when many short-lived clients connect.
	if l.lastTick.IsZero() || now.Sub(l.lastTick) > time.Minute {
		l.sweepLocked(now)
		l.lastTick = now
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.burst), last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * float64(l.rate)
		if b.tokens > float64(l.burst) {
			b.tokens = float64(l.burst)
		}
		b.last = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweepLocked drops buckets that have been idle long enough to refill fully,
// which keeps the map bounded for long-running servers. Caller holds l.mu.
func (l *rateLimiter) sweepLocked(now time.Time) {
	idleThreshold := time.Duration(float64(l.burst)/float64(l.rate)) * time.Second
	if idleThreshold < time.Minute {
		idleThreshold = time.Minute
	}
	for key, b := range l.buckets {
		if now.Sub(b.last) > idleThreshold {
			delete(l.buckets, key)
		}
	}
}

// retryAfter returns a conservative Retry-After duration in seconds.
func (l *rateLimiter) retryAfter() int {
	if l.rate <= 0 {
		return 1
	}
	// One token refills every 1/rate seconds; round up to a whole second.
	secs := 1
	if float64(l.rate) < 1 {
		secs = int(1/float64(l.rate)) + 1
	}
	return secs
}

// defaultMutationRate is the sustained mutating requests per second allowed
// from a single remote address. 5 is generous for interactive use but
// throttles a flood.
const (
	defaultMutationRate  = 5
	defaultMutationBurst = 15
)

// limitMutation applies the rate limiter to mutating requests. It returns
// true when the request should proceed.
func (s *Server) limitMutation(w http.ResponseWriter, r *http.Request) bool {
	if s.mutationLimiter == nil {
		return true
	}
	key := r.RemoteAddr
	if key == "" {
		key = "unknown"
	}
	if s.mutationLimiter.allow(key) {
		return true
	}
	w.Header().Set("Retry-After", strconv.Itoa(s.mutationLimiter.retryAfter()))
	writeErr(w, http.StatusTooManyRequests, errTooManyRequests)
	return false
}
