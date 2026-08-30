package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func fastLimiter() *Limiter {
	return New(Options{
		Window:       50 * time.Millisecond,
		MaxFailures:  3,
		LockoutAfter: 3,
		Lockout:      200 * time.Millisecond,
	})
}

func TestDefaultMaxFailures(t *testing.T) {
	l := New(Options{})
	if l.opts.MaxFailures != 3 {
		t.Fatalf("MaxFailures default = %d, want 3", l.opts.MaxFailures)
	}
}

func TestAllowBelowLimit(t *testing.T) {
	l := fastLimiter()
	for i := 0; i < 2; i++ {
		l.Fail("ip:1")
		if res := l.Allow("ip:1"); !res.Allowed {
			t.Fatalf("attempt %d: expected allowed, got blocked", i+1)
		}
	}
}

func TestBlockAtLimit(t *testing.T) {
	l := fastLimiter()
	for i := 0; i < 3; i++ {
		l.Fail("ip:1")
	}
	res := l.Allow("ip:1")
	if res.Allowed {
		t.Fatal("expected blocked after 3 failures")
	}
	if res.RetryAfter <= 0 || res.RetryAfter > time.Minute {
		t.Fatalf("unexpected retry-after: %v", res.RetryAfter)
	}
}

func TestSuccessResets(t *testing.T) {
	l := fastLimiter()
	for i := 0; i < 2; i++ {
		l.Fail("ip:1")
	}
	l.Success("ip:1")
	for i := 0; i < 2; i++ {
		l.Fail("ip:1")
		if res := l.Allow("ip:1"); !res.Allowed {
			t.Fatalf("attempt %d: expected allowed after success reset", i+1)
		}
	}
}

func TestAllowedAfterWindowPasses(t *testing.T) {
	l := New(Options{
		Window:       50 * time.Millisecond,
		MaxFailures:  3,
		LockoutAfter: 0, // escalation disabled: window block only
	})
	for i := 0; i < 3; i++ {
		l.Fail("ip:1")
	}
	if res := l.Allow("ip:1"); res.Allowed {
		t.Fatal("expected blocked in active window")
	}
	time.Sleep(60 * time.Millisecond)
	if res := l.Allow("ip:1"); !res.Allowed {
		t.Fatalf("expected allowed after window passed, got retry=%v", res.RetryAfter)
	}
}

func TestEscalatesToLockout(t *testing.T) {
	l := New(Options{
		Window:       50 * time.Millisecond,
		MaxFailures:  3,
		LockoutAfter: 6,
		Lockout:      200 * time.Millisecond,
	})
	// First window: 3 failures, then let it pass.
	for i := 0; i < 3; i++ {
		l.Fail("ip:1")
	}
	time.Sleep(60 * time.Millisecond)
	// Later windows allow one attempt before blocking again; the cumulative
	// counter keeps growing. Three more failures reach the LockoutAfter
	// threshold.
	for i := 0; i < 3; i++ {
		time.Sleep(60 * time.Millisecond)
		if res := l.Allow("ip:1"); !res.Allowed {
			t.Fatalf("attempt %d: expected allowed at window start, blocked", i+1)
		}
		l.Fail("ip:1")
	}
	res := l.Allow("ip:1")
	if res.Allowed {
		t.Fatal("expected lockout after cumulative failures")
	}
	if res.RetryAfter < 150*time.Millisecond {
		t.Fatalf("expected lockout-length retry-after, got %v", res.RetryAfter)
	}
	// Windows may pass, but the lockout still applies.
	time.Sleep(70 * time.Millisecond)
	if res := l.Allow("ip:1"); res.Allowed {
		t.Fatal("lockout must survive the window")
	}
}

func TestLockoutExpires(t *testing.T) {
	l := fastLimiter()
	for i := 0; i < 3; i++ {
		l.Fail("ip:1")
	}
	time.Sleep(250 * time.Millisecond)
	if res := l.Allow("ip:1"); !res.Allowed {
		t.Fatalf("expected allowed after lockout expired, got retry=%v", res.RetryAfter)
	}
}

func TestConcurrentUse(t *testing.T) {
	l := fastLimiter()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				l.Fail("ip:1")
				l.Allow("ip:1")
			}
			l.Success("ip:1")
		}()
	}
	wg.Wait()
}
