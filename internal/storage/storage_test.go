package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/srmdn/foliocms/internal/storage"
)

func TestParseRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s := storage.New(dir)
	slug := "hello-world"

	original := &storage.PostFile{
		Frontmatter: storage.Frontmatter{
			Title:       "Hello World",
			Description: "A test post",
			PublishDate: "2026-01-01",
			Draft:       false,
			Tags:        []string{"go", "test"},
		},
		Body: "# Hello\n\nThis is the body.\n",
	}

	if err := s.Write(slug, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := s.Read(slug)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Frontmatter.Title != original.Frontmatter.Title {
		t.Errorf("Title: got %q, want %q", got.Frontmatter.Title, original.Frontmatter.Title)
	}
	if got.Frontmatter.Description != original.Frontmatter.Description {
		t.Errorf("Description: got %q, want %q", got.Frontmatter.Description, original.Frontmatter.Description)
	}
	if got.Frontmatter.PublishDate != original.Frontmatter.PublishDate {
		t.Errorf("PublishDate: got %q, want %q", got.Frontmatter.PublishDate, original.Frontmatter.PublishDate)
	}
	if got.Frontmatter.Draft != original.Frontmatter.Draft {
		t.Errorf("Draft: got %v, want %v", got.Frontmatter.Draft, original.Frontmatter.Draft)
	}
	if len(got.Frontmatter.Tags) != len(original.Frontmatter.Tags) {
		t.Errorf("Tags length: got %d, want %d", len(got.Frontmatter.Tags), len(original.Frontmatter.Tags))
	} else {
		for i, tag := range original.Frontmatter.Tags {
			if got.Frontmatter.Tags[i] != tag {
				t.Errorf("Tags[%d]: got %q, want %q", i, got.Frontmatter.Tags[i], tag)
			}
		}
	}
	if got.Body != original.Body {
		t.Errorf("Body: got %q, want %q", got.Body, original.Body)
	}
}

func TestWriteCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	s := storage.New(dir)
	slug := "new-post"

	pf := &storage.PostFile{
		Frontmatter: storage.Frontmatter{Title: "New Post"},
		Body:        "body",
	}

	if err := s.Write(slug, pf); err != nil {
		t.Fatalf("Write: %v", err)
	}

	postPath := filepath.Join(dir, slug, "index.md")
	if _, err := os.Stat(postPath); err != nil {
		t.Errorf("expected file at %s: %v", postPath, err)
	}
}

func TestExistsReturnsTrueAfterWrite(t *testing.T) {
	dir := t.TempDir()
	s := storage.New(dir)
	slug := "exists-test"

	if s.Exists(slug) {
		t.Error("Exists should be false before write")
	}

	if err := s.Write(slug, &storage.PostFile{Frontmatter: storage.Frontmatter{Title: "T"}, Body: ""}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !s.Exists(slug) {
		t.Error("Exists should be true after write")
	}
}

func TestDeleteRemovesPost(t *testing.T) {
	dir := t.TempDir()
	s := storage.New(dir)
	slug := "delete-test"

	s.Write(slug, &storage.PostFile{Frontmatter: storage.Frontmatter{Title: "T"}, Body: ""})

	if err := s.Delete(slug); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if s.Exists(slug) {
		t.Error("Exists should be false after delete")
	}
}

func TestReadMissingFrontmatterDelimiter(t *testing.T) {
	dir := t.TempDir()
	s := storage.New(dir)
	slug := "bad-post"

	postDir := filepath.Join(dir, slug)
	os.MkdirAll(postDir, 0755)
	os.WriteFile(filepath.Join(postDir, "index.md"), []byte("no frontmatter here"), 0644)

	_, err := s.Read(slug)
	if err == nil {
		t.Error("expected error for missing frontmatter delimiter")
	}
}

func TestReadUnclosedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	s := storage.New(dir)
	slug := "unclosed-post"

	postDir := filepath.Join(dir, slug)
	os.MkdirAll(postDir, 0755)
	os.WriteFile(filepath.Join(postDir, "index.md"), []byte("---\ntitle: No closing\n"), 0644)

	_, err := s.Read(slug)
	if err == nil {
		t.Error("expected error for unclosed frontmatter")
	}
}

func TestFormatPublishDate(t *testing.T) {
	tt := time.Date(2026, 3, 27, 15, 30, 0, 0, time.FixedZone("WIB", 7*60*60))
	got := storage.FormatPublishDate(tt)
	want := "2026-03-27T08:30:00Z"
	if got != want {
		t.Errorf("FormatPublishDate: got %q, want %q", got, want)
	}
}

func TestParsePublishDate(t *testing.T) {
	cases := map[string]struct {
		in       string
		want     string
		expected bool
	}{
		"rfc3339":      {"2026-03-27T08:30:00Z", "2026-03-27T08:30:00Z", true},
		"date only":    {"2026-03-27", "2026-03-27T00:00:00Z", true},
		"sqlite style": {"2026-03-27 08:30:00", "2026-03-27T08:30:00Z", true},
		"t separator":  {"2026-03-27T08:30:00", "2026-03-27T08:30:00Z", true},
		"empty":        {"", "", false},
		"garbage":      {"not-a-date", "", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := storage.ParsePublishDate(c.in)
			if ok != c.expected {
				t.Fatalf("ok = %v, want %v", ok, c.expected)
			}
			if c.expected && got.UTC().Format(time.RFC3339) != c.want {
				t.Errorf("got %q, want %q", got.UTC().Format(time.RFC3339), c.want)
			}
		})
	}
}
