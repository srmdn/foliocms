package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/srmdn/foliocms/internal/model"
	"github.com/srmdn/foliocms/internal/storage"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type postRequest struct {
	Slug        string   `json:"slug"` // new slug for rename on update; ignored on create
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Draft       bool     `json:"draft"`
	PublishDate string   `json:"publish_date"`
	Body        string   `json:"body"`
	HeroImage   string   `json:"hero_image"` // base64 data URI; empty = preserve existing on update
}

type postResponse struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Draft       bool      `json:"draft"`
	PublishDate time.Time `json:"publish_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Body        string    `json:"body,omitempty"`
	HeroImage   string    `json:"hero_image,omitempty"`
}

type postListResponse struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Draft       bool      `json:"draft"`
	PublishDate time.Time `json:"publish_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListPosts returns published posts (public).
func (h *Handler) ListPosts(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id, slug, title, description, tags, draft, publish_date, created_at, updated_at
		 FROM posts WHERE draft = 0 AND (publish_date IS NULL OR publish_date <= CURRENT_TIMESTAMP)`
	args := []any{}

	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if tag != "" {
		query += ` AND instr(',' || replace(tags, ' ', '') || ',', ',' || ? || ',') > 0`
		args = append(args, strings.ReplaceAll(tag, " ", ""))
	}

	query += ` ORDER BY publish_date DESC`

	rows, err := h.db.Query(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch posts")
		return
	}
	defer rows.Close()

	posts, err := scanPosts(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read posts")
		return
	}
	writeJSON(w, http.StatusOK, toPostListResponses(posts))
}

// ListAllPosts returns all posts including drafts (admin).
func (h *Handler) ListAllPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(
		`SELECT id, slug, title, description, tags, draft, publish_date, created_at, updated_at
		 FROM posts ORDER BY updated_at DESC`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch posts")
		return
	}
	defer rows.Close()

	posts, err := scanPosts(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read posts")
		return
	}
	writeJSON(w, http.StatusOK, toPostListResponses(posts))
}

// GetPost returns a single published post with its body (public).
func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var post model.Post
	err := scanPost(h.db.QueryRow(
		`SELECT id, slug, title, description, tags, draft, publish_date, created_at, updated_at
		 FROM posts WHERE slug = ? AND draft = 0`, slug,
	), &post)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch post")
		return
	}

	store := storage.New(h.cfg.ContentDir)
	pf, err := store.Read(slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read post content")
		return
	}

	heroImage := ""
	if pf.Frontmatter.HeroImage != "" {
		heroImage, _ = store.ReadHeroImageAsDataURI(slug, pf.Frontmatter.HeroImage)
	}

	writeJSON(w, http.StatusOK, toPostResponse(post, pf.Body, heroImage))
}

// GetAdminPost returns a single post regardless of draft status (admin).
func (h *Handler) GetAdminPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var post model.Post
	err := scanPost(h.db.QueryRow(
		`SELECT id, slug, title, description, tags, draft, publish_date, created_at, updated_at
		 FROM posts WHERE slug = ?`, slug,
	), &post)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch post")
		return
	}

	store := storage.New(h.cfg.ContentDir)
	pf, err := store.Read(slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read post content")
		return
	}

	heroImage := ""
	if pf.Frontmatter.HeroImage != "" {
		heroImage, _ = store.ReadHeroImageAsDataURI(slug, pf.Frontmatter.HeroImage)
	}

	writeJSON(w, http.StatusOK, toPostResponse(post, pf.Body, heroImage))
}

// CreatePost creates a new post (admin).
func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !slugPattern.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "slug must be lowercase letters, numbers, and hyphens only")
		return
	}

	var req postRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	store := storage.New(h.cfg.ContentDir)
	if store.Exists(slug) {
		writeError(w, http.StatusConflict, "post already exists")
		return
	}

	publishDate := req.PublishDate
	if publishDate == "" {
		publishDate = storage.FormatPublishDate(time.Now())
	}

	// Save hero image if provided.
	heroImagePath := ""
	if req.HeroImage != "" {
		if path, err := store.SaveHeroImage(slug, req.HeroImage); err == nil {
			heroImagePath = path
		}
	}

	if err := store.Write(slug, &storage.PostFile{
		Frontmatter: storage.Frontmatter{
			Title:       req.Title,
			Description: req.Description,
			PublishDate: publishDate,
			Draft:       req.Draft,
			Tags:        req.Tags,
			HeroImage:   heroImagePath,
		},
		Body: req.Body,
	}); err != nil {
		store.Delete(slug)
		writeError(w, http.StatusInternalServerError, "could not write post file")
		return
	}

	_, err := h.db.Exec(
		`INSERT INTO posts (slug, title, description, tags, draft, publish_date)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		slug, req.Title, req.Description, strings.Join(req.Tags, ","),
		boolToInt(req.Draft), publishDate,
	)
	if err != nil {
		store.Delete(slug)
		writeError(w, http.StatusInternalServerError, "could not save post")
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// UpdatePost updates an existing post's metadata and body (admin).
// If req.Slug differs from the URL slug, the post directory is renamed.
func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	originalSlug := chi.URLParam(r, "slug")

	var req postRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	// Determine the target slug (req.Slug overrides URL slug for rename).
	newSlug := strings.TrimSpace(req.Slug)
	if newSlug == "" {
		newSlug = originalSlug
	}
	if !slugPattern.MatchString(newSlug) {
		writeError(w, http.StatusBadRequest, "slug must be lowercase letters, numbers, and hyphens only")
		return
	}

	store := storage.New(h.cfg.ContentDir)
	if !store.Exists(originalSlug) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}

	// Rename the content directory if the slug changed.
	if newSlug != originalSlug {
		if store.Exists(newSlug) {
			writeError(w, http.StatusConflict, "slug already taken")
			return
		}
		if err := store.Rename(originalSlug, newSlug); err != nil {
			writeError(w, http.StatusInternalServerError, "could not rename post")
			return
		}
	}

	publishDate := req.PublishDate
	if publishDate == "" {
		publishDate = storage.FormatPublishDate(time.Now())
	}

	// Preserve existing hero image path unless a new one is provided.
	heroImagePath := ""
	if req.HeroImage != "" {
		if path, err := store.SaveHeroImage(newSlug, req.HeroImage); err == nil {
			heroImagePath = path
		}
	} else {
		if existing, err := store.Read(newSlug); err == nil {
			heroImagePath = existing.Frontmatter.HeroImage
		}
	}

	if err := store.Write(newSlug, &storage.PostFile{
		Frontmatter: storage.Frontmatter{
			Title:       req.Title,
			Description: req.Description,
			PublishDate: publishDate,
			Draft:       req.Draft,
			Tags:        req.Tags,
			HeroImage:   heroImagePath,
		},
		Body: req.Body,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "could not write post file")
		return
	}

	_, err := h.db.Exec(
		`UPDATE posts SET slug=?, title=?, description=?, tags=?, draft=?, publish_date=?, updated_at=CURRENT_TIMESTAMP
		 WHERE slug=?`,
		newSlug, req.Title, req.Description, strings.Join(req.Tags, ","),
		boolToInt(req.Draft), publishDate, originalSlug,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update post")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeletePost removes a post's file and DB record (admin).
func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	store := storage.New(h.cfg.ContentDir)
	if !store.Exists(slug) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}

	if err := store.Delete(slug); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete post files")
		return
	}

	if _, err := h.db.Exec(`DELETE FROM posts WHERE slug = ?`, slug); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete post record")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// helpers

func scanPosts(rows *sql.Rows) ([]model.Post, error) {
	var posts []model.Post
	for rows.Next() {
		var p model.Post
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Title, &p.Description,
			&p.Tags, &p.Draft, &p.PublishDate, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func scanPost(row *sql.Row, p *model.Post) error {
	return row.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Description,
		&p.Tags, &p.Draft, &p.PublishDate, &p.CreatedAt, &p.UpdatedAt,
	)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func splitTags(tags string) []string {
	if tags == "" {
		return []string{}
	}

	parts := strings.Split(tags, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

func toPostListResponses(posts []model.Post) []postListResponse {
	responses := make([]postListResponse, 0, len(posts))
	for _, post := range posts {
		responses = append(responses, postListResponse{
			ID:          post.ID,
			Slug:        post.Slug,
			Title:       post.Title,
			Description: post.Description,
			Tags:        splitTags(post.Tags),
			Draft:       post.Draft,
			PublishDate: post.PublishDate,
			CreatedAt:   post.CreatedAt,
			UpdatedAt:   post.UpdatedAt,
		})
	}
	return responses
}

func toPostResponse(post model.Post, body, heroImage string) postResponse {
	return postResponse{
		ID:          post.ID,
		Slug:        post.Slug,
		Title:       post.Title,
		Description: post.Description,
		Tags:        splitTags(post.Tags),
		Draft:       post.Draft,
		PublishDate: post.PublishDate,
		CreatedAt:   post.CreatedAt,
		UpdatedAt:   post.UpdatedAt,
		Body:        body,
		HeroImage:   heroImage,
	}
}
