package auth

import (
	"sync"
	"time"
)

// limiter blocks brute-force guessing: after too many FAILED sign-ins for the
// same email (or from the same IP) within a window, further attempts are
// refused until the oldest failure ages out -- even with the right password,
// so an attacker can't tell when they guessed correctly.
//
// It is in-memory (per API process). That is enough for a single-instance
// internal tool; behind several instances the limit is per instance.
type limiter struct {
	mu       sync.Mutex
	window   time.Duration
	maxEmail int
	maxIP    int
	fails    map[string][]time.Time
	now      func() time.Time
}

func newLimiter() *limiter {
	return &limiter{window: 15 * time.Minute, maxEmail: 5, maxIP: 30, fails: map[string][]time.Time{}, now: time.Now}
}

func emailKey(email string) string { return "e:" + email }
func ipKey(ip string) string       { return "i:" + ip }

// prune drops failures older than the window and returns what is left.
func (l *limiter) prune(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	kept := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = kept
	return kept
}

// blockedFor returns how long the caller must wait (0 = allowed).
func (l *limiter) blockedFor(email, ip string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	var wait time.Duration
	for _, c := range []struct {
		key string
		max int
	}{{emailKey(email), l.maxEmail}, {ipKey(ip), l.maxIP}} {
		fails := l.prune(c.key)
		if len(fails) >= c.max {
			if w := fails[len(fails)-c.max].Add(l.window).Sub(l.now()); w > wait {
				wait = w
			}
		}
	}
	return wait
}

func (l *limiter) fail(email, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.fails[emailKey(email)] = append(l.fails[emailKey(email)], now)
	l.fails[ipKey(ip)] = append(l.fails[ipKey(ip)], now)
}

// succeed forgets the email's failures (the IP's are kept: one good login
// must not reset a scan of many accounts from the same address).
func (l *limiter) succeed(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, emailKey(email))
}
