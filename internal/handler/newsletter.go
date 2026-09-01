package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// emailRe is a pragmatic basic-format check (hostname requires a TLD dot);
// full RFC parsing is unnecessary for a subscribe endpoint.
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Subscribe adds a new subscriber. POST /api/subscribe
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	// Throttle failures per IP; a successful subscription resets the limit.
	key := "sub:" + clientIP(r)
	if res := h.subscribeLimiter.Allow(key); !res.Allowed {
		writeRateLimited(w, res.RetryAfter, "too many requests, try again later")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !emailRe.MatchString(email) {
		h.subscribeLimiter.Fail(key)
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}

	token := uuid.New().String()
	_, err := h.db.Exec(
		`INSERT INTO subscribers (email, token) VALUES (?, ?)`,
		email, token,
	)
	if err != nil {
		// SQLite unique constraint violation
		if isUniqueErr(err) {
			h.subscribeLimiter.Fail(key)
			writeError(w, http.StatusConflict, "already subscribed")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not subscribe")
		return
	}

	h.subscribeLimiter.Success(key)
	w.WriteHeader(http.StatusCreated)
}

// Unsubscribe removes a subscriber by token. GET /api/unsubscribe?token=xxx
func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	res, err := h.db.Exec(`DELETE FROM subscribers WHERE token = ?`, token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not unsubscribe")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>Unsubscribed</title>` +
		`<style>body{font-family:sans-serif;max-width:480px;margin:80px auto;text-align:center;color:#333}` +
		`h1{font-size:1.4rem;margin-bottom:.5rem}p{color:#666}</style></head>` +
		`<body><h1>You have been unsubscribed.</h1><p>You will no longer receive emails from this site.</p></body></html>`))
}

// subscriberView is the admin-facing shape. The unsubscribe token is an
// internal key and must never leave the server.
type subscriberView struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	SubscribedAt time.Time `json:"subscribed_at"`
}

// ListSubscribers returns all subscribers. GET /api/admin/subscribers
func (h *Handler) ListSubscribers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(
		`SELECT id, email, subscribed_at FROM subscribers ORDER BY subscribed_at DESC`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list subscribers")
		return
	}
	defer rows.Close()

	var subs []subscriberView
	for rows.Next() {
		var s subscriberView
		if err := rows.Scan(&s.ID, &s.Email, &s.SubscribedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		subs = append(subs, s)
	}
	if subs == nil {
		subs = []subscriberView{}
	}
	writeJSON(w, http.StatusOK, subs)
}

// DeleteSubscriber removes a subscriber by ID. DELETE /api/admin/subscribers/{id}
func (h *Handler) DeleteSubscriber(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := h.db.Exec(`DELETE FROM subscribers WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete subscriber")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "subscriber not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SendNewsletter sends an email to all subscribers. POST /api/admin/newsletter/send
func (h *Handler) SendNewsletter(w http.ResponseWriter, r *http.Request) {
	if h.cfg.SMTPHost == "" {
		writeError(w, http.StatusServiceUnavailable, "SMTP not configured")
		return
	}

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Subject == "" || req.Body == "" {
		writeError(w, http.StatusBadRequest, "subject and body are required")
		return
	}

	rows, err := h.db.Query(`SELECT email, token FROM subscribers`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch subscribers")
		return
	}
	defer rows.Close()

	type recipient struct {
		email string
		token string
	}
	var recipients []recipient
	for rows.Next() {
		var rc recipient
		if err := rows.Scan(&rc.email, &rc.token); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		recipients = append(recipients, rc)
	}

	if len(recipients) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"sent": 0, "message": "no subscribers"})
		return
	}

	var sendErrs []string
	for _, rc := range recipients {
		unsubURL := h.cfg.SiteURL + "/api/unsubscribe?token=" + rc.token
		personalizedBody := req.Body + "\n\n--\nTo unsubscribe, visit: " + unsubURL
		if err := h.mailer.Send(rc.email, req.Subject, personalizedBody); err != nil {
			sendErrs = append(sendErrs, rc.email+": "+err.Error())
		}
	}
	if len(sendErrs) > 0 {
		writeError(w, http.StatusInternalServerError, "send failures: "+strings.Join(sendErrs, "; "))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"sent": len(recipients)})
}

func isUniqueErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
