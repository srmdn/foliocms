package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/srmdn/foliocms/internal/config"
	"github.com/srmdn/foliocms/internal/db"
	"github.com/srmdn/foliocms/internal/mailer"
	"github.com/srmdn/foliocms/internal/media"
	"github.com/srmdn/foliocms/internal/ratelimit"
	"github.com/srmdn/foliocms/internal/rebuild"
)

// MailSender is satisfied by *mailer.Mailer and any test double.
type MailSender interface {
	Send(to, subject, body string) error
}

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	db               *db.DB
	cfg              *config.Config
	rebuilder        *rebuild.Rebuilder
	mailer           MailSender
	mediaDriver      media.MediaDriver
	loginLimiter     *ratelimit.Limiter
	subscribeLimiter *ratelimit.Limiter
}

func New(database *db.DB, cfg *config.Config) *Handler {
	m := mailer.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	return &Handler{
		db:     database,
		cfg:    cfg,
		mailer: m,
		loginLimiter: ratelimit.New(ratelimit.Options{
			Window:       time.Minute,
			MaxFailures:  3,
			LockoutAfter: 3,
			Lockout:      time.Hour,
		}),
		subscribeLimiter: ratelimit.New(ratelimit.Options{
			Window:       time.Minute,
			MaxFailures:  5,
			LockoutAfter: 5,
			Lockout:      15 * time.Minute,
		}),
	}
}

func (h *Handler) SetMailer(m MailSender) {
	h.mailer = m
}

// SetLoginLimiter replaces the auth failure limiter (used by tests).
func (h *Handler) SetLoginLimiter(l *ratelimit.Limiter) {
	h.loginLimiter = l
}

// SetSubscribeLimiter replaces the subscription limiter (used by tests).
func (h *Handler) SetSubscribeLimiter(l *ratelimit.Limiter) {
	h.subscribeLimiter = l
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
