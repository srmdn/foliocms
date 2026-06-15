package handler_test

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/srmdn/foliocms/internal/config"
	"github.com/srmdn/foliocms/internal/handler"
)

func routerWithFeed(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/feed.xml", h.GetFeed)
	r.Post("/api/admin/posts/{slug}", h.CreatePost)
	return r
}

func newFeedHandler(t *testing.T, contentDir string) *handler.Handler {
	t.Helper()
	database := newTestDB(t)
	cfg := &config.Config{
		ContentDir: contentDir,
		SiteURL:    "https://example.com",
	}
	return handler.New(database, cfg)
}

func TestGetFeedEmpty(t *testing.T) {
	dir := t.TempDir()
	h := newFeedHandler(t, dir)
	r := routerWithFeed(h)

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/rss+xml") {
		t.Errorf("Content-Type: got %q, want application/rss+xml", ct)
	}
	if !strings.Contains(w.Body.String(), "<rss") {
		t.Errorf("body missing <rss> element")
	}
}

func TestGetFeedContainsPublishedPost(t *testing.T) {
	dir := t.TempDir()
	h := newFeedHandler(t, dir)
	r := routerWithFeed(h)

	createPost(t, r, "hello-world", map[string]any{
		"title":        "Hello World",
		"description":  "A test post",
		"draft":        false,
		"publish_date": "2026-01-15",
	})

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Hello World") {
		t.Errorf("feed missing post title")
	}
	if !strings.Contains(body, "example.com/blog/hello-world") {
		t.Errorf("feed missing post link")
	}
	if !strings.Contains(body, "A test post") {
		t.Errorf("feed missing post description")
	}
}

func TestGetFeedExcludesDrafts(t *testing.T) {
	dir := t.TempDir()
	h := newFeedHandler(t, dir)
	r := routerWithFeed(h)

	createPost(t, r, "draft-post", map[string]any{
		"title": "Draft Post",
		"draft": true,
	})

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if strings.Contains(w.Body.String(), "Draft Post") {
		t.Errorf("feed must not include draft posts")
	}
}

func TestGetFeedExcludesFutureDatedPosts(t *testing.T) {
	dir := t.TempDir()
	h := newFeedHandler(t, dir)
	r := routerWithFeed(h)

	createPost(t, r, "future-post", map[string]any{
		"title":        "Future Post",
		"draft":        false,
		"publish_date": "2099-01-01",
	})
	createPost(t, r, "past-post", map[string]any{
		"title":        "Past Post",
		"draft":        false,
		"publish_date": "2026-02-01",
	})

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "Future Post") {
		t.Errorf("feed must not include future-dated posts")
	}
	if !strings.Contains(body, "Past Post") {
		t.Errorf("feed should include already-published posts")
	}
}

func TestGetFeedValidXML(t *testing.T) {
	dir := t.TempDir()
	h := newFeedHandler(t, dir)
	r := routerWithFeed(h)

	createPost(t, r, "xml-post", map[string]any{
		"title":        "XML & Special <chars>",
		"description":  "Ampersands & brackets",
		"draft":        false,
		"publish_date": "2026-02-01",
	})

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if err := xml.Unmarshal(w.Body.Bytes(), &struct{ XMLName xml.Name }{}); err != nil {
		t.Errorf("feed is not valid XML: %v", err)
	}
}
