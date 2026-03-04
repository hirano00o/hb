package article_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/hatena"
)

func TestRead_WithFrontmatter(t *testing.T) {
	a, err := article.Read("testdata/with_frontmatter.md")
	if err != nil {
		t.Fatal(err)
	}
	if a.Frontmatter.Title != "Test Article" {
		t.Errorf("title: got %q", a.Frontmatter.Title)
	}
	if a.Frontmatter.Draft {
		t.Error("draft should be false")
	}
	if len(a.Frontmatter.Category) != 2 {
		t.Errorf("category: got %v", a.Frontmatter.Category)
	}
	if a.Frontmatter.EditURL != "https://blog.hatena.ne.jp/user/example.hateblo.jp/atom/entry/123456789" {
		t.Errorf("editUrl: got %q", a.Frontmatter.EditURL)
	}
	if a.Body == "" {
		t.Error("body should not be empty")
	}
}

func TestRead_NoFrontmatter(t *testing.T) {
	a, err := article.Read("testdata/no_frontmatter.md")
	if err != nil {
		t.Fatal(err)
	}
	if a.Frontmatter.Title != "" {
		t.Errorf("title should be empty, got %q", a.Frontmatter.Title)
	}
	if a.Body == "" {
		t.Error("body should not be empty")
	}
}

func TestRead_InvalidYAML(t *testing.T) {
	_, err := article.Read("testdata/invalid_yaml.md")
	if err == nil {
		t.Error("expected error for invalid YAML frontmatter")
	}
}

func TestWrite_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	fm := article.Frontmatter{
		Title: "Round Trip",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: false,
	}
	orig := &article.Article{Frontmatter: fm, Body: "body content\n"}
	if err := article.Write(path, orig); err != nil {
		t.Fatal(err)
	}
	read, err := article.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if read.Frontmatter.Title != "Round Trip" {
		t.Errorf("title: got %q", read.Frontmatter.Title)
	}
	if read.Body != "body content\n" {
		t.Errorf("body: got %q", read.Body)
	}
}

func TestGenerateFilename(t *testing.T) {
	date := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		title string
		draft bool
		want  string
	}{
		{"Hello World", false, "20260301_Hello-World.md"},
		{"Hello World", true, "draft_20260301_Hello-World.md"},
		{"test/file:name", false, "20260301_test_file_name.md"},
	}
	for _, tt := range tests {
		got := article.GenerateFilename(tt.title, date, tt.draft)
		if got != tt.want {
			t.Errorf("GenerateFilename(%q, draft=%v) = %q, want %q", tt.title, tt.draft, got, tt.want)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct{ in, want string }{
		{"normal", "normal"},
		{`a/b\c:d*e?f"g<h>i|j`, "a_b_c_d_e_f_g_h_i_j"},
		{"hello world", "hello-world"},
		{"hello  world", "hello--world"},
	}
	for _, tt := range tests {
		got := article.SanitizeFilename(tt.in)
		if got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFromEntry_WithScheduledAt(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	e := &hatena.Entry{
		Title:       "Scheduled Entry",
		Content:     "body",
		Date:        time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		Draft:       true,
		ScheduledAt: scheduledAt,
	}
	a := article.FromEntry(e)
	if a.Frontmatter.ScheduledAt == nil {
		t.Fatal("ScheduledAt should not be nil")
	}
	if !a.Frontmatter.ScheduledAt.Equal(scheduledAt) {
		t.Errorf("ScheduledAt: got %v, want %v", a.Frontmatter.ScheduledAt, scheduledAt)
	}
	if !a.Frontmatter.Draft {
		t.Error("draft should be true")
	}
}

func TestFromEntry_NoScheduledAt(t *testing.T) {
	e := &hatena.Entry{
		Title:   "Plain Draft",
		Content: "body",
		Draft:   true,
	}
	a := article.FromEntry(e)
	if a.Frontmatter.ScheduledAt != nil {
		t.Errorf("ScheduledAt should be nil for plain draft, got %v", a.Frontmatter.ScheduledAt)
	}
}

func TestToEntry_WithScheduledAt(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	a := &article.Article{
		Frontmatter: article.Frontmatter{
			Title:       "Scheduled",
			Draft:       false,
			ScheduledAt: &scheduledAt,
		},
		Body: "body",
	}
	e := a.ToEntry()
	// ToEntry maps ScheduledAt but does not force Draft; push.go is responsible for that.
	if !e.ScheduledAt.Equal(scheduledAt) {
		t.Errorf("ScheduledAt: got %v, want %v", e.ScheduledAt, scheduledAt)
	}
}

func TestToEntry_NoScheduledAt(t *testing.T) {
	a := &article.Article{
		Frontmatter: article.Frontmatter{
			Title: "Normal",
			Draft: false,
		},
		Body: "body",
	}
	e := a.ToEntry()
	if !e.ScheduledAt.IsZero() {
		t.Errorf("ScheduledAt should be zero, got %v", e.ScheduledAt)
	}
}

func TestFromEntry(t *testing.T) {
	e := &hatena.Entry{
		Title:      "Entry Title",
		Content:    "body",
		Date:       time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:      true,
		Categories: []string{"Go"},
		URL:        "https://example.com/entry",
		EditURL:    "https://blog.hatena.ne.jp/atom/entry/1",
		CustomURL:  "custom",
	}
	a := article.FromEntry(e)
	if a.Frontmatter.Title != "Entry Title" {
		t.Errorf("title: got %q", a.Frontmatter.Title)
	}
	if !a.Frontmatter.Draft {
		t.Error("draft should be true")
	}
	if a.Body != "body" {
		t.Errorf("body: got %q", a.Body)
	}
}

func TestToEntry(t *testing.T) {
	a := &article.Article{
		Frontmatter: article.Frontmatter{
			Title:         "Article Title",
			Date:          time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			Draft:         false,
			Category:      []string{"CLI"},
			EditURL:       "https://blog.hatena.ne.jp/atom/entry/42",
			CustomURLPath: "path",
		},
		Body: "content",
	}
	e := a.ToEntry()
	if e.Title != "Article Title" {
		t.Errorf("title: got %q", e.Title)
	}
	if e.Draft {
		t.Error("draft should be false")
	}
	if e.EditURL != "https://blog.hatena.ne.jp/atom/entry/42" {
		t.Errorf("editURL: got %q", e.EditURL)
	}
	if e.Content != "content" {
		t.Errorf("content: got %q", e.Content)
	}
	if !e.Updated.IsZero() {
		t.Errorf("Updated should be zero (set by server), got %v", e.Updated)
	}
}
