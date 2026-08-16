package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllowsBurstThenThrottles(t *testing.T) {
	rl := newRateLimiter(10, 3)
	for i := 0; i < 3; i++ {
		if !rl.allow("client") {
			t.Fatalf("request %d within burst was throttled", i+1)
		}
	}
	// The 4th immediate request exceeds the burst of 3.
	if rl.allow("client") {
		t.Fatal("4th immediate request was allowed beyond burst")
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	tick := time.Now()
	rl := newRateLimiter(10, 2)
	rl.now = func() time.Time { return tick }

	if !rl.allow("c") {
		t.Fatal("first request throttled")
	}
	if !rl.allow("c") {
		t.Fatal("second request throttled")
	}
	if rl.allow("c") {
		t.Fatal("third request beyond burst allowed")
	}

	// Advance 200ms at 10 tokens/sec -> +2 tokens, enough for one more.
	tick = tick.Add(200 * time.Millisecond)
	if !rl.allow("c") {
		t.Fatal("request after refill was throttled")
	}
}

func TestRateLimiterIsolatesClients(t *testing.T) {
	rl := newRateLimiter(1, 1)
	if !rl.allow("a") {
		t.Fatal("client a first request throttled")
	}
	if rl.allow("a") {
		t.Fatal("client a second request allowed")
	}
	// A different client has its own bucket.
	if !rl.allow("b") {
		t.Fatal("client b was throttled by client a's bucket")
	}
}

func TestRateLimiterSweepDropsIdleBuckets(t *testing.T) {
	tick := time.Now()
	rl := newRateLimiter(100, 10)
	rl.now = func() time.Time { return tick }

	rl.allow("transient")
	if _, ok := rl.buckets["transient"]; !ok {
		t.Fatal("bucket was not recorded")
	}
	// Advance well past the idle threshold.
	tick = tick.Add(10 * time.Minute)
	rl.sweepLocked(tick)
	// Trigger a call on a different key so the map is touched.
	rl.allow("other")
	if _, ok := rl.buckets["transient"]; ok {
		t.Fatal("idle bucket was not swept")
	}
}

func TestLimitMutationReturns429WhenThrottled(t *testing.T) {
	s := &Server{mutationLimiter: newRateLimiter(1, 1)}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limitMutation(w, r) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/x", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first request = %d, want 204", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/x", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request = %d, want 429", second.Code)
	}
	if got := second.Header().Get("Retry-After"); got == "" {
		t.Error("429 response is missing Retry-After header")
	}
}

func TestLimitMutationNoLimiterAllowsAll(t *testing.T) {
	s := &Server{} // mutationLimiter is nil
	allowed := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.limitMutation(w, r) {
			allowed++
		}
	})
	for i := 0; i < 100; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/x", nil))
	}
	if allowed != 100 {
		t.Fatalf("nil limiter allowed %d requests, want 100", allowed)
	}
}

// Concurrent clients must not race on the bucket map. Run with -race to
// exercise this.
func TestRateLimiterConcurrent(t *testing.T) {
	rl := newRateLimiter(1000, 100)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "client"
			if n%2 == 0 {
				key = "other"
			}
			for j := 0; j < 50; j++ {
				_ = rl.allow(key)
			}
		}(i)
	}
	wg.Wait()
}
