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

func routerWithSitemap(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/sitemap.xml", h.GetSitemap)
	r.Post("/api/admin/posts/{slug}", h.CreatePost)
	return r
}

func newSitemapHandler(t *testing.T, contentDir string) *handler.Handler {
	t.Helper()
	database := newTestDB(t)
	cfg := &config.Config{
		ContentDir: contentDir,
		SiteURL:    "https://example.com",
	}
	return handler.New(database, cfg)
}

func TestGetSitemapEmpty(t *testing.T) {
	dir := t.TempDir()
	h := newSitemapHandler(t, dir)
	r := routerWithSitemap(h)

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/xml") {
		t.Errorf("Content-Type: got %q, want application/xml", ct)
	}
	if !strings.Contains(w.Body.String(), "<urlset") {
		t.Errorf("body missing <urlset> element")
	}
}

func TestGetSitemapContainsPublishedPost(t *testing.T) {
	dir := t.TempDir()
	h := newSitemapHandler(t, dir)
	r := routerWithSitemap(h)

	createPost(t, r, "my-first-post", map[string]any{
		"title":        "My First Post",
		"description":  "A test post",
		"draft":        false,
		"publish_date": "2026-01-15",
	})

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "example.com/blog/my-first-post") {
		t.Errorf("sitemap missing post loc: %s", body)
	}
	if !strings.Contains(body, "<lastmod>") {
		t.Errorf("sitemap missing lastmod element")
	}
}

func TestGetSitemapExcludesDrafts(t *testing.T) {
	dir := t.TempDir()
	h := newSitemapHandler(t, dir)
	r := routerWithSitemap(h)

	createPost(t, r, "draft-post", map[string]any{
		"title": "Draft Post",
		"draft": true,
	})

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if strings.Contains(w.Body.String(), "draft-post") {
		t.Errorf("sitemap must not include draft posts")
	}
}

func TestGetSitemapExcludesFutureDatedPosts(t *testing.T) {
	dir := t.TempDir()
	h := newSitemapHandler(t, dir)
	r := routerWithSitemap(h)

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

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "future-post") {
		t.Errorf("sitemap must not include future-dated posts")
	}
	if !strings.Contains(body, "past-post") {
		t.Errorf("sitemap should include already-published posts")
	}
}

func TestGetSitemapValidXML(t *testing.T) {
	dir := t.TempDir()
	h := newSitemapHandler(t, dir)
	r := routerWithSitemap(h)

	createPost(t, r, "xml-post", map[string]any{
		"title":        "XML & Special <chars>",
		"description":  "Ampersands & brackets",
		"draft":        false,
		"publish_date": "2026-02-01",
	})

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if err := xml.Unmarshal(w.Body.Bytes(), &struct{ XMLName xml.Name }{}); err != nil {
		t.Errorf("sitemap is not valid XML: %v", err)
	}
}

func TestGetSitemapSitemapNsAttribute(t *testing.T) {
	dir := t.TempDir()
	h := newSitemapHandler(t, dir)
	r := routerWithSitemap(h)

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "sitemaps.org/schemas/sitemap/0.9") {
		t.Errorf("sitemap missing required xmlns attribute")
	}
}
