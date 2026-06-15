package handler

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/srmdn/foliocms/internal/model"
)

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language"`
	Items       []rssItem `xml:"item"`
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

// GetFeed serves an RSS 2.0 feed of published posts.
func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(
		`SELECT id, slug, title, description, tags, draft, publish_date, created_at, updated_at
		 FROM posts WHERE draft = 0 AND (publish_date IS NULL OR publish_date <= CURRENT_TIMESTAMP)
		 ORDER BY publish_date DESC`,
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

	settings, err := h.loadSettings()
	if err != nil {
		http.Error(w, "could not load settings", http.StatusInternalServerError)
		return
	}

	siteURL := strings.TrimRight(h.cfg.SiteURL, "/")
	items := make([]rssItem, len(posts))
	for i, p := range posts {
		link := siteURL + "/blog/" + p.Slug
		items[i] = rssItem{
			Title:       p.Title,
			Link:        link,
			Description: p.Description,
			PubDate:     formatRSSDate(p),
			GUID:        link,
		}
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       settings["site_name"],
			Link:        siteURL,
			Description: settings["site_description"],
			Language:    "en-us",
			Items:       items,
		},
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(feed)
}

func formatRSSDate(p model.Post) string {
	t := p.PublishDate
	if t.IsZero() {
		t = p.CreatedAt
	}
	return t.UTC().Format(time.RFC1123Z)
}
