package handler

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/srmdn/foliocms/internal/auth"
	"github.com/srmdn/foliocms/internal/middleware"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	CSRFToken string `json:"csrf_token"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Rate-limit failure attempts per IP, per account, and per IP+account so
	// rotating spoofed X-Forwarded-For values still trip the account key.
	// Checks run before any password hashing: throttled requests must not
	// burn the CPU budget that protected logins use.
	ip := clientIP(r)
	email := strings.ToLower(req.Email)
	keys := []string{"ip:" + ip, "acct:" + email, "acct:" + ip + "\x00" + email}
	var retryAfter time.Duration
	for _, k := range keys {
		if res := h.loginLimiter.Allow(k); !res.Allowed && res.RetryAfter > retryAfter {
			retryAfter = res.RetryAfter
		}
	}
	if retryAfter > 0 {
		log.Printf("security: login rejected for %q from %s: rate limited (%s)", email, ip, retryAfter)
		writeRateLimited(w, retryAfter, "too many login attempts, try again later")
		return
	}

	var userID int64
	var passwdHash string
	err := h.db.QueryRow(
		`SELECT id, passwd_hash FROM users WHERE email = ?`, req.Email,
	).Scan(&userID, &passwdHash)
	if err != nil {
		log.Printf("security: failed login for %q from %s (user not found)", email, ip)
		recordLoginFailure(h, keys)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwdHash), []byte(req.Password)); err != nil {
		log.Printf("security: failed login for %q from %s (bad password)", email, ip)
		recordLoginFailure(h, keys)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	log.Printf("security: successful login for %q from %s", email, ip)

	for _, k := range keys {
		h.loginLimiter.Success(k)
	}

	token, err := auth.GenerateToken(userID, h.cfg.JWTSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	// Parse back to get claims (needed to derive CSRF token)
	claims, err := auth.ParseToken(token, h.cfg.JWTSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	writeJSON(w, http.StatusOK, loginResponse{
		CSRFToken: auth.CSRFTokenFromClaims(claims, h.cfg.JWTSecret),
	})
}

func recordLoginFailure(h *Handler, keys []string) {
	for _, k := range keys {
		h.loginLimiter.Fail(k)
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeRateLimited(w http.ResponseWriter, retryAfter time.Duration, message string) {
	seconds := int(retryAfter / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, message)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// GetCSRFToken returns the CSRF token for the currently authenticated session.
// Clients that lose the CSRF token (e.g. page refresh) can call this to
// retrieve it without logging in again.
func (h *Handler) GetCSRFToken(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"csrf_token": auth.CSRFTokenFromClaims(claims, h.cfg.JWTSecret),
	})
}
