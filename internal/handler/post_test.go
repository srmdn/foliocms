package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/srmdn/foliocms/internal/handler"
)

func routerWithHandler(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/posts", h.ListPosts)
	r.Get("/api/posts/{slug}", h.GetPost)
	r.Get("/api/admin/posts", h.ListAllPosts)
	r.Get("/api/admin/posts/{slug}", h.GetAdminPost)
	r.Post("/api/admin/posts/{slug}", h.CreatePost)
	r.Put("/api/admin/posts/{slug}", h.UpdatePost)
	r.Delete("/api/admin/posts/{slug}", h.DeletePost)
	return r
}

func createPost(t *testing.T, r *chi.Mux, slug string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/posts/"+slug, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestListPostsEmpty(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCreatePostSuccess(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	w := createPost(t, r, "my-post", map[string]any{
		"title":       "My Post",
		"description": "A description",
		"draft":       false,
		"tags":        []string{"go"},
		"body":        "# Hello\n",
	})

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d\nbody: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestCreatePostInvalidSlug(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	// Uppercase letters are rejected by the slug pattern; URL must still be valid.
	w := createPost(t, r, "INVALID", map[string]any{"title": "T"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreatePostMissingTitle(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	w := createPost(t, r, "valid-slug", map[string]any{"title": ""})

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreatePostDuplicateSlug(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "dup-slug", map[string]any{"title": "First"})
	w := createPost(t, r, "dup-slug", map[string]any{"title": "Second"})

	if w.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestGetPostNotFound(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/api/posts/no-such-post", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetAdminPostFound(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "found-post", map[string]any{
		"title": "Found Post",
		"draft": true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/posts/found-post", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d\nbody: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestPublicListExcludesDrafts(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "draft-post", map[string]any{
		"title": "Draft",
		"draft": true,
	})
	createPost(t, r, "pub-post", map[string]any{
		"title":        "Published",
		"draft":        false,
		"publish_date": "2026-01-01",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var posts []map[string]any
	json.Unmarshal(w.Body.Bytes(), &posts)

	if len(posts) != 1 {
		t.Errorf("expected 1 published post, got %d", len(posts))
	}
}

func TestPublicListReturnsTagsArray(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "tagged-post", map[string]any{
		"title":        "Tagged",
		"draft":        false,
		"publish_date": "2026-01-01",
		"tags":         []string{"go", "cms"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var posts []struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &posts); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}
	if len(posts[0].Tags) != 2 || posts[0].Tags[0] != "go" || posts[0].Tags[1] != "cms" {
		t.Fatalf("expected tags [go cms], got %#v", posts[0].Tags)
	}
}

func TestPublicListExcludesFutureDated(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "future-post", map[string]any{
		"title":        "Future",
		"draft":        false,
		"publish_date": "2099-01-01",
	})
	createPost(t, r, "past-post", map[string]any{
		"title":        "Past",
		"draft":        false,
		"publish_date": "2026-01-01",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var posts []map[string]any
	json.Unmarshal(w.Body.Bytes(), &posts)

	if len(posts) != 1 {
		t.Errorf("expected 1 post (future-dated excluded), got %d", len(posts))
	}
	if len(posts) == 1 && posts[0]["slug"] != "past-post" {
		t.Errorf("expected slug past-post, got %v", posts[0]["slug"])
	}
}

func TestPublicListFiltersByTag(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "go-post", map[string]any{
		"title":        "Go Post",
		"draft":        false,
		"publish_date": "2026-01-01",
		"tags":         []string{"go", "cms"},
	})
	createPost(t, r, "js-post", map[string]any{
		"title":        "JS Post",
		"draft":        false,
		"publish_date": "2026-01-01",
		"tags":         []string{"javascript"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts?tag=go", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var posts []struct {
		Slug string `json:"slug"`
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &posts); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 tagged post, got %d", len(posts))
	}
	if posts[0].Slug != "go-post" {
		t.Fatalf("expected slug go-post, got %s", posts[0].Slug)
	}
}

func TestPublicListTagFilterUsesExactMatch(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "go-post", map[string]any{
		"title":        "Go Post",
		"draft":        false,
		"publish_date": "2026-01-01",
		"tags":         []string{"go"},
	})
	createPost(t, r, "golang-post", map[string]any{
		"title":        "Golang Post",
		"draft":        false,
		"publish_date": "2026-01-01",
		"tags":         []string{"golang"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts?tag=go", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var posts []struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &posts); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 exact-match post, got %d", len(posts))
	}
	if posts[0].Slug != "go-post" {
		t.Fatalf("expected slug go-post, got %s", posts[0].Slug)
	}
}

func TestUpdatePost(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "update-me", map[string]any{"title": "Original"})

	updated, _ := json.Marshal(map[string]any{"title": "Updated", "draft": false})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/posts/update-me", bytes.NewReader(updated))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d\nbody: %s", w.Code, http.StatusNoContent, w.Body.String())
	}
}

func TestGetAdminPostReturnsTagsArray(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "tag-admin", map[string]any{
		"title": "Tagged Admin",
		"tags":  []string{"go", "cms"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/posts/tag-admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var post struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &post); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(post.Tags) != 2 || post.Tags[0] != "go" || post.Tags[1] != "cms" {
		t.Fatalf("expected tags [go cms], got %#v", post.Tags)
	}
}

func TestDeletePost(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	r := routerWithHandler(h)

	createPost(t, r, "delete-me", map[string]any{"title": "Delete Me"})

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/posts/delete-me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNoContent)
	}

	// Confirm gone from admin list
	req2 := httptest.NewRequest(http.MethodGet, "/api/admin/posts/delete-me", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("after delete, expected 404, got %d", w2.Code)
	}
}
