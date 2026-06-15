package handler

import (
	"encoding/xml"
	"net/http"
	"strings"
)

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// GetSitemap serves an XML sitemap of published posts.
func (h *Handler) GetSitemap(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(
		`SELECT id, slug, title, description, tags, draft, publish_date, created_at, updated_at
		 FROM posts WHERE draft = 0 AND (publish_date IS NULL OR publish_date <= CURRENT_TIMESTAMP)
		 ORDER BY updated_at DESC`,
	)
	if err != nil {
		http.Error(w, "could not fetch posts", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	posts, err := scanPosts(rows)
	if err != nil {
		http.Error(w, "could not read posts", http.StatusInternalServerError)
		return
	}

	siteURL := strings.TrimRight(h.cfg.SiteURL, "/")
	urls := make([]sitemapURL, len(posts))
	for i, p := range posts {
		urls[i] = sitemapURL{
			Loc:     siteURL + "/blog/" + p.Slug,
			LastMod: p.UpdatedAt.UTC().Format("2006-01-02"),
		}
	}

	urlset := sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(urlset)
}
