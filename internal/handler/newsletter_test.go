package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/srmdn/foliocms/internal/config"
	"github.com/srmdn/foliocms/internal/handler"
	"github.com/srmdn/foliocms/internal/ratelimit"
)

type recordingSender struct {
	calls []sentEmail
}

type sentEmail struct {
	to, subject, body string
}

func (r *recordingSender) Send(to, subject, body string) error {
	r.calls = append(r.calls, sentEmail{to, subject, body})
	return nil
}

func newsletterRouter(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/api/subscribe", h.Subscribe)
	r.Post("/api/admin/newsletter/send", h.SendNewsletter)
	return r
}

func newNewsletterHandler(t *testing.T, dir string, siteURL string) (*handler.Handler, *recordingSender) {
	t.Helper()
	database := newTestDB(t)
	cfg := &config.Config{
		ContentDir: dir,
		JWTSecret:  "test-secret",
		SMTPHost:   "smtp.example.com",
		SiteURL:    siteURL,
	}
	h := handler.New(database, cfg)
	sender := &recordingSender{}
	h.SetMailer(sender)
	return h, sender
}

func subscribeEmail(t *testing.T, r *chi.Mux, email string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"email": email})
	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("subscribe %s: got %d", email, w.Code)
	}
}

func TestAdminSubscribersOmitsToken(t *testing.T) {
	h, _ := newNewsletterHandler(t, t.TempDir(), "https://example.com")
	r := newsletterRouter(h)
	r.Get("/api/admin/subscribers", h.ListSubscribers)

	subscribeEmail(t, r, "noleak@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/subscribers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if strings.Contains(w.Body.String(), `"token"`) {
		t.Errorf("response leaks token key: %s", w.Body.String())
	}
}

func TestSendNewsletterInjectsUnsubscribeLink(t *testing.T) {
	dir := t.TempDir()
	h, sender := newNewsletterHandler(t, dir, "https://example.com")
	r := newsletterRouter(h)

	subscribeEmail(t, r, "alice@example.com")
	subscribeEmail(t, r, "bob@example.com")

	payload, _ := json.Marshal(map[string]string{
		"subject": "Hello",
		"body":    "Newsletter body.",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/newsletter/send", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("send: got %d, body: %s", w.Code, w.Body.String())
	}
	if len(sender.calls) != 2 {
		t.Fatalf("expected 2 sends, got %d", len(sender.calls))
	}
	for _, call := range sender.calls {
		if !strings.Contains(call.body, "https://example.com/api/unsubscribe?token=") {
			t.Errorf("email to %s missing unsubscribe link; body: %s", call.to, call.body)
		}
		if !strings.Contains(call.body, "Newsletter body.") {
			t.Errorf("email to %s missing original body", call.to)
		}
	}
}

func TestSendNewsletterNoSubscribers(t *testing.T) {
	dir := t.TempDir()
	h, sender := newNewsletterHandler(t, dir, "https://example.com")
	r := newsletterRouter(h)

	payload, _ := json.Marshal(map[string]string{"subject": "Hi", "body": "Body."})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/newsletter/send", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 sends, got %d", len(sender.calls))
	}
}

func TestSendNewsletterSMTPNotConfigured(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir) // SMTPHost is empty
	r := newsletterRouter(h)

	payload, _ := json.Marshal(map[string]string{"subject": "Hi", "body": "Body."})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/newsletter/send", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestSubscribeNormalizesEmail(t *testing.T) {
	database := newTestDB(t)
	h := handler.New(database, &config.Config{ContentDir: t.TempDir(), JWTSecret: "test-secret"})
	r := newsletterRouter(h)

	subscribeEmail(t, r, "  Alice@Example.COM  ")

	var email string
	database.QueryRow(`SELECT email FROM subscribers`).Scan(&email)
	if email != "alice@example.com" {
		t.Errorf("stored email = %q, want alice@example.com", email)
	}
}

func TestSubscribeRejectsInvalidEmail(t *testing.T) {
	h, _ := newNewsletterHandler(t, t.TempDir(), "https://example.com")
	r := newsletterRouter(h)

	for _, bad := range []string{"not-an-email", "user@nodot", "user@.com", "a@b@c.com"} {
		b, _ := json.Marshal(map[string]string{"email": bad})
		req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("email %q: got %d, want 400", bad, w.Code)
		}
	}
}

func TestSubscribeRateLimited(t *testing.T) {
	h, _ := newNewsletterHandler(t, t.TempDir(), "https://example.com")
	r := newsletterRouter(h)

	// Five invalid emails trip the per-IP throttle (every one is a failure).
	for i := 0; i < 5; i++ {
		b, _ := json.Marshal(map[string]string{"email": "not-an-email"})
		req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status = %d, want 400", i+1, w.Code)
		}
	}
	// A valid email is also refused: the IP is throttled.
	b, _ := json.Marshal(map[string]string{"email": "fresh@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429; body: %s", w.Code, w.Body.String())
	}
	v := w.Header().Get("Retry-After")
	if v == "" {
		t.Fatal("expected Retry-After header")
	}
	if n, err := strconv.Atoi(v); err != nil || n < 1 {
		t.Fatalf("bad Retry-After %q", v)
	}
}

func TestSubscribeSuccessResetsLimit(t *testing.T) {
	h, _ := newNewsletterHandler(t, t.TempDir(), "https://example.com")
	r := newsletterRouter(h)
	h.SetSubscribeLimiter(ratelimit.New(ratelimit.Options{
		Window:       50 * time.Millisecond,
		MaxFailures:  3,
		LockoutAfter: 3,
		Lockout:      200 * time.Millisecond,
	}))

	for i := 0; i < 2; i++ {
		b, _ := json.Marshal(map[string]string{"email": "not-an-email"})
		req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status = %d, want 400", i+1, w.Code)
		}
	}
	subscribeEmail(t, r, "works@example.com")
	for i := 0; i < 2; i++ {
		b, _ := json.Marshal(map[string]string{"email": "not-an-email"})
		req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d after reset: status = %d, want 400", i+1, w.Code)
		}
	}
}
