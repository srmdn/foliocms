package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/srmdn/foliocms/internal/adminui"
	"github.com/srmdn/foliocms/internal/config"
	"github.com/srmdn/foliocms/internal/db"
	"github.com/srmdn/foliocms/internal/demo"
	"github.com/srmdn/foliocms/internal/handler"
	"github.com/srmdn/foliocms/internal/media"
	"github.com/srmdn/foliocms/internal/middleware"
	"github.com/srmdn/foliocms/internal/rebuild"
)

// version is set at build time via -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	envFile := flag.String("env", ".env", "path to env file")
	setup := flag.Bool("setup", false, "run first-time setup wizard")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	cfg, err := config.Load(*envFile)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	if err := database.Migrate("migrations"); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	if *setup {
		if err := runSetup(database, cfg); err != nil {
			log.Fatalf("setup: %v", err)
		}
		fmt.Println("Setup complete. Run folio without --setup to start the server.")
		os.Exit(0)
	}

	if cfg.DemoMode {
		log.Println("demo mode: seeding demo content")
		if err := demo.Apply(database, cfg); err != nil {
			log.Fatalf("demo seed: %v", err)
		}
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.SecurityHeaders)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	h := handler.New(database, cfg)
	rb := rebuild.New(cfg.ThemeDir, cfg.ThemeBuildCmd, cfg.ThemeService)
	h.SetRebuilder(rb)
	if cfg.MediaStorage == "s3" {
		h.SetMediaDriver(media.NewS3Driver(
			cfg.S3Endpoint,
			cfg.S3Bucket,
			cfg.S3Region,
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			cfg.S3PublicURL,
		))
	} else {
		h.SetMediaDriver(media.NewLocalDriver(cfg.MediaDir, cfg.SiteURL))
	}

	// Admin SPA (embedded React build)
	adminHandler := adminui.Handler()
	r.Group(func(r chi.Router) {
		r.Use(middleware.AdminCSP)
		r.Handle("/admin", adminHandler)
		r.Handle("/admin/*", adminHandler)
	})

	// Serve local media files (local driver only; S3 files are served from the bucket directly)
	if cfg.MediaStorage != "s3" {
		r.Handle("/media/*", http.StripPrefix("/media/", http.FileServer(http.Dir(cfg.MediaDir))))
	}

	// Public routes
	r.Get("/feed.xml", h.GetFeed)
	r.Get("/sitemap.xml", h.GetSitemap)
	r.Post("/api/login", h.Login)
	r.Post("/api/logout", h.Logout)
	r.Get("/api/posts", h.ListPosts)
	r.Get("/api/posts/{slug}", h.GetPost)
	r.Post("/api/webhook/rebuild", h.WebhookRebuild)
	r.Post("/api/subscribe", h.Subscribe)
	r.Get("/api/unsubscribe", h.Unsubscribe)
	r.Get("/api/settings", h.GetPublicSettings)
	r.Get("/api/demo", h.DemoInfo)

	// Protected routes (JWT + CSRF)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(cfg.JWTSecret))
		r.Get("/api/csrf-token", h.GetCSRFToken)

		r.Group(func(r chi.Router) {
			r.Use(middleware.VerifyCSRF(cfg.JWTSecret))
			r.Get("/api/admin/posts", h.ListAllPosts)
			r.Get("/api/admin/posts/{slug}", h.GetAdminPost)
			r.Post("/api/admin/posts/{slug}", h.CreatePost)
			r.Put("/api/admin/posts/{slug}", h.UpdatePost)
			r.Delete("/api/admin/posts/{slug}", h.DeletePost)
			r.Post("/api/admin/posts/{slug}/duplicate", h.DuplicatePost)
			r.Put("/api/admin/account/password", h.ChangePassword)
			r.Post("/api/admin/rebuild", h.TriggerRebuild)
			r.Get("/api/admin/rebuild/status", h.RebuildStatus)
			r.Get("/api/admin/subscribers", h.ListSubscribers)
			r.Delete("/api/admin/subscribers/{id}", h.DeleteSubscriber)
			r.Post("/api/admin/newsletter/send", h.SendNewsletter)
			r.Get("/api/admin/settings", h.GetAdminSettings)
			r.Put("/api/admin/settings", h.UpdateSettings)
			r.Post("/api/admin/media", h.UploadMedia)
			r.Get("/api/admin/media", h.ListMedia)
			r.Delete("/api/admin/media/{key}", h.DeleteMedia)
			r.Post("/api/admin/demo/reset", h.DemoReset)
		})
	})

	srv := &http.Server{
		Addr:         "127.0.0.1:" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("folio listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("folio stopped")
}
