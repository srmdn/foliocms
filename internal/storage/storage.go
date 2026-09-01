package storage

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Frontmatter is the YAML header written to each post's index.md.
type Frontmatter struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	PublishDate string   `yaml:"publishDate"`
	Draft       bool     `yaml:"draft"`
	Tags        []string `yaml:"tags"`
	HeroImage   string   `yaml:"heroImage,omitempty"`
}

// PostFile represents a post file on disk.
type PostFile struct {
	Frontmatter Frontmatter
	Body        string // markdown body (after frontmatter)
}

// Storage handles reading and writing post files under a content directory.
type Storage struct {
	contentDir string
}

func New(contentDir string) *Storage {
	return &Storage{contentDir: contentDir}
}

// PostDir returns the directory for a given slug.
func (s *Storage) PostDir(slug string) string {
	return filepath.Join(s.contentDir, slug)
}

// PostPath returns the full path to a post's index.md.
func (s *Storage) PostPath(slug string) string {
	return filepath.Join(s.contentDir, slug, "index.md")
}

// Read reads and parses a post file from disk.
func (s *Storage) Read(slug string) (*PostFile, error) {
	raw, err := os.ReadFile(s.PostPath(slug))
	if err != nil {
		return nil, fmt.Errorf("reading post %q: %w", slug, err)
	}
	return parse(raw)
}

// Write creates or overwrites a post file on disk.
func (s *Storage) Write(slug string, pf *PostFile) error {
	if err := os.MkdirAll(s.PostDir(slug), 0755); err != nil {
		return fmt.Errorf("creating post dir %q: %w", slug, err)
	}
	content, err := marshal(pf)
	if err != nil {
		return err
	}
	return os.WriteFile(s.PostPath(slug), content, 0644)
}

// Delete removes the post directory and all its contents.
func (s *Storage) Delete(slug string) error {
	if err := os.RemoveAll(s.PostDir(slug)); err != nil {
		return fmt.Errorf("deleting post %q: %w", slug, err)
	}
	return nil
}

// Exists reports whether a post file exists on disk.
func (s *Storage) Exists(slug string) bool {
	_, err := os.Stat(s.PostPath(slug))
	return err == nil
}

// Copy copies a post directory from srcSlug to dstSlug, preserving all assets.
func (s *Storage) Copy(srcSlug, dstSlug string) error {
	return copyDir(s.PostDir(srcSlug), s.PostDir(dstSlug))
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("creating dir %q: %w", dst, err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading dir %q: %w", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %q: %w", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %q: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying %q -> %q: %w", src, dst, err)
	}
	return nil
}

// Rename renames a post directory from oldSlug to newSlug (all assets move with it).
func (s *Storage) Rename(oldSlug, newSlug string) error {
	oldDir := s.PostDir(oldSlug)
	newDir := s.PostDir(newSlug)
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		return fmt.Errorf("post %q not found", oldSlug)
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("slug %q already taken", newSlug)
	}
	return os.Rename(oldDir, newDir)
}

// SaveHeroImage decodes a base64 data URI and writes the image to the post directory.
// Returns the relative path (e.g. "./hero.jpg") to store in frontmatter.
func (s *Storage) SaveHeroImage(slug, dataURI string) (string, error) {
	if !strings.HasPrefix(dataURI, "data:") {
		return "", fmt.Errorf("not a data URI")
	}
	semi := strings.Index(dataURI, ";")
	comma := strings.Index(dataURI, ",")
	if semi == -1 || comma == -1 || comma < semi {
		return "", fmt.Errorf("invalid data URI format")
	}
	mimeType := dataURI[5:semi]
	imgData, err := base64.StdEncoding.DecodeString(dataURI[comma+1:])
	if err != nil {
		return "", fmt.Errorf("decoding image: %w", err)
	}
	ext := heroImageExt(mimeType)
	postDir := s.PostDir(slug)
	if err := os.MkdirAll(postDir, 0755); err != nil {
		return "", fmt.Errorf("creating post dir: %w", err)
	}
	// Remove any stale hero files before writing the new one.
	for _, e := range []string{".webp", ".jpg", ".png", ".gif"} {
		os.Remove(filepath.Join(postDir, "hero"+e))
	}
	filename := "hero" + ext
	if err := os.WriteFile(filepath.Join(postDir, filename), imgData, 0644); err != nil {
		return "", fmt.Errorf("writing hero image: %w", err)
	}
	return "./" + filename, nil
}

// ReadHeroImageAsDataURI reads a local hero image file and returns it as a base64 data URI.
// If heroPath is not a relative local path it is returned unchanged.
func (s *Storage) ReadHeroImageAsDataURI(slug, heroPath string) (string, error) {
	if !strings.HasPrefix(heroPath, "./") {
		return heroPath, nil
	}
	fullPath := filepath.Join(s.PostDir(slug), strings.TrimPrefix(heroPath, "./"))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("reading hero image: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(fullPath))
	return "data:" + extToMime(ext) + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func heroImageExt(mimeType string) string {
	switch mimeType {
	case "image/webp":
		return ".webp"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

func extToMime(ext string) string {
	switch ext {
	case ".webp":
		return "image/webp"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

func parse(raw []byte) (*PostFile, error) {
	const delim = "---"

	// Trim only leading whitespace so trailing newlines in the body are preserved.
	content := strings.TrimLeft(string(raw), " \t\r\n")

	if !strings.HasPrefix(content, delim) {
		return nil, fmt.Errorf("missing frontmatter delimiter")
	}

	// Skip past the opening --- and its trailing newline.
	rest := content[len(delim):]
	if len(rest) == 0 || rest[0] != '\n' {
		return nil, fmt.Errorf("missing frontmatter delimiter")
	}
	rest = rest[1:]

	// Find the closing ---.
	idx := strings.Index(rest, "\n"+delim)
	if idx == -1 {
		return nil, fmt.Errorf("unclosed frontmatter")
	}

	yamlPart := rest[:idx]

	// Skip \n--- and the blank line marshal writes before the body.
	// marshal writes "---\n\n" so two newlines follow the closing delimiter.
	body := rest[idx+1+len(delim):]
	body = strings.TrimPrefix(body, "\n\n")
	// Fallback for files with only one newline after ---.
	body = strings.TrimPrefix(body, "\n")

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	return &PostFile{Frontmatter: fm, Body: body}, nil
}

func marshal(pf *PostFile) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(pf.Frontmatter); err != nil {
		return nil, fmt.Errorf("encoding frontmatter: %w", err)
	}
	enc.Close()
	buf.WriteString("---\n\n")
	buf.WriteString(pf.Body)
	return buf.Bytes(), nil
}

// FormatPublishDate returns the current instant as an RFC3339 UTC string —
// the canonical publish_date persisted in files and SQLite.
func FormatPublishDate(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ParsePublishDate reads a publishDate value from frontmatter or the DB.
// Accepts RFC3339 (with T/Z), plain dates (YYYY-MM-DD), and common spacers.
func ParsePublishDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05",
		"2006-01-02 15:04", "2006-01-02",
	} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
