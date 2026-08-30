// Package ratelimit provides an in-memory failure limiter for login-like
// endpoints. Counters are per-key (e.g. IP, account, or combined). On process
// restart all state resets, which is acceptable for a single-instance CMS.
package ratelimit

import (
	"sync"
	"time"
)

// Options configures a Limiter.
type Options struct {
	// Window is the length of a failure window. Failures within the same
	// window are counted together.
	Window time.Duration
	// MaxFailures is the number of failures tolerated inside an active
	// window before the key is blocked until the window ends.
	MaxFailures int
	// LockoutAfter places the key under a longer Lockout once this many
	// failures accumulate since the last success. Zero disables escalation.
	LockoutAfter int
	// Lockout is the block duration used once LockoutAfter is reached.
	Lockout time.Duration
	// MaxEntries caps the number of tracked keys; old entries are pruned
	// first. Zero uses DefaultMaxEntries.
	MaxEntries int
}

// DefaultMaxEntries bounds memory growth when many different keys are probed.
const DefaultMaxEntries = 10_000

// Result describes the outcome of an Allow check. RetryAfter is non-zero only
// when Allowed is false.
type Result struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter is safe for concurrent use. It only remembers failures; successful
// operations call Success to clear the counters.
type Limiter struct {
	mu   sync.Mutex
	opts Options
	mem  map[string]*entry
}

type entry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

// New returns a limiter with sane defaults for missing Options fields.
func New(opts Options) *Limiter {
	if opts.Window <= 0 {
		opts.Window = time.Minute
	}
	if opts.MaxFailures <= 0 {
		opts.MaxFailures = 3
	}
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = DefaultMaxEntries
	}
	return &Limiter{opts: opts, mem: make(map[string]*entry)}
}

// Allow reports whether an attempt for key may proceed. RetryAfter carries how
// long the caller should wait before trying again.
func (l *Limiter) Allow(key string) Result {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.mem[key]
	if !ok {
		return Result{Allowed: true}
	}
	now := time.Now()
	if now.Before(e.lockedUntil) {
		return Result{Allowed: false, RetryAfter: e.lockedUntil.Sub(now)}
	}
	if e.failures >= l.opts.MaxFailures && now.Before(l.windowEnd(e)) {
		return Result{Allowed: false, RetryAfter: l.windowEnd(e).Sub(now)}
	}
	return Result{Allowed: true}
}

// Fail records a failure for key. Failures accumulate until Success is called
// or the entry is pruned after long inactivity.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, ok := l.mem[key]
	if !ok {
		e = &entry{windowStart: now}
		l.mem[key] = e
	} else if !now.Before(e.windowStart.Add(l.opts.Window)) {
		// Fresh window, but keep the cumulative failure count so sustained
		// attacks escalate instead of starting over every minute.
		e.windowStart = now
	}
	e.failures++
	if l.opts.LockoutAfter > 0 && e.failures >= l.opts.LockoutAfter {
		e.lockedUntil = now.Add(l.opts.Lockout)
	}
	l.prune(now)
}

// Success clears the counters for key.
func (l *Limiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.mem, key)
}

func (l *Limiter) windowEnd(e *entry) time.Time {
	return e.windowStart.Add(l.opts.Window)
}

func (l *Limiter) prune(now time.Time) {
	if len(l.mem) <= l.opts.MaxEntries {
		return
	}
	for k, e := range l.mem {
		if now.After(e.windowStart.Add(2*l.opts.Window)) && now.After(e.lockedUntil) {
			delete(l.mem, k)
		}
	}
	if len(l.mem) > l.opts.MaxEntries {
		var oldestKey string
		var oldest time.Time
		for k, e := range l.mem {
			if oldestKey == "" || e.windowStart.Before(oldest) {
				oldestKey, oldest = k, e.windowStart
			}
		}
		delete(l.mem, oldestKey)
	}
}
