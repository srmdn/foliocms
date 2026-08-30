package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func passthrough() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	SecurityHeaders(passthrough()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for key, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := rec.Header().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if got := rec.Header().Get("Permissions-Policy"); got == "" {
		t.Error("expected Permissions-Policy header")
	}
}

func TestAdminCSP(t *testing.T) {
	rec := httptest.NewRecorder()
	AdminCSP(passthrough()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP missing default-src 'self': %s", csp)
	}
	if strings.Contains(csp, "script-src 'self'") && strings.Contains(csp, "unsafe-inline") &&
		strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("script-src must not allow unsafe-inline: %s", csp)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP missing frame-ancestors 'none': %s", csp)
	}
	if !strings.Contains(csp, "object-src 'none'") {
		t.Errorf("CSP missing object-src 'none': %s", csp)
	}
}
