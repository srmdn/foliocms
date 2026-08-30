package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/srmdn/foliocms/internal/handler"
	"github.com/srmdn/foliocms/internal/ratelimit"
)

func newLoginHandler(t *testing.T) (*handler.Handler, *chi.Mux) {
	t.Helper()
	h, _ := newAccountHandler(t)
	r := chi.NewRouter()
	r.Post("/api/login", h.Login)
	return h, r
}

func loginReq(t *testing.T, r *chi.Mux, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func retryAfterSec(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	v := w.Header().Get("Retry-After")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("invalid Retry-After %q: %v", v, err)
	}
	return n
}

func TestLoginBlocksAfterThreeFailures(t *testing.T) {
	_, r := newLoginHandler(t)
	for i := 0; i < 3; i++ {
		if w := loginReq(t, r, "admin@example.com", "wrongpass"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, w.Code)
		}
	}
	// Even a valid pair is refused once the key is throttled.
	w := loginReq(t, r, "admin@example.com", "rightpass")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429; body: %s", w.Code, w.Body.String())
	}
	if s := retryAfterSec(t, w); s < 1 {
		t.Fatalf("expected Retry-After >= 1s, got %d", s)
	}
}

func TestLoginUnknownAccountCountsToo(t *testing.T) {
	_, r := newLoginHandler(t)
	for i := 0; i < 3; i++ {
		if w := loginReq(t, r, "nobody@example.com", "wrongpass"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, w.Code)
		}
	}
	if w := loginReq(t, r, "nobody@example.com", "rightpass"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
}

func TestLoginSuccessResetsFailures(t *testing.T) {
	h, database := newAccountHandler(t)
	insertUser(t, database, "admin@example.com", "rightpass")
	r := chi.NewRouter()
	r.Post("/api/login", h.Login)

	for i := 0; i < 2; i++ {
		if w := loginReq(t, r, "admin@example.com", "wrongpass"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, w.Code)
		}
	}
	if w := loginReq(t, r, "admin@example.com", "rightpass"); w.Code != http.StatusOK {
		t.Fatalf("login after failures: status = %d, want 200", w.Code)
	}
	const triesAfterReset = 2
	for i := 0; i < triesAfterReset; i++ {
		if w := loginReq(t, r, "admin@example.com", "wrongpass"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d after reset: status = %d, want 401", i+1, w.Code)
		}
	}
}

func TestLoginEscalatesToLockout(t *testing.T) {
	// Short lockout so the escalation path is testable without waiting the
	// production one hour.
	h, r := newLoginHandler(t)
	h.SetLoginLimiter(ratelimit.New(ratelimit.Options{
		Window:       50 * time.Millisecond,
		MaxFailures:  3,
		LockoutAfter: 3,
		Lockout:      200 * time.Millisecond,
	}))
	for i := 0; i < 3; i++ {
		if w := loginReq(t, r, "admin@example.com", "wrongpass"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, w.Code)
		}
	}
	w := loginReq(t, r, "admin@example.com", "wrongpass")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 after lockout", w.Code)
	}
	if s := retryAfterSec(t, w); s < 1 {
		t.Fatalf("expected lockout Retry-After >= 1s, got %d", s)
	}
}
