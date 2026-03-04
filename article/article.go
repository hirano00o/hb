package article

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hirano00o/hb/hatena"
)

// Article represents a local Markdown file with frontmatter and body.
type Article struct {
	Frontmatter Frontmatter
	Body        string
}

// Read reads an Article from the Markdown file at path.
func Read(path string) (*Article, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read article %s: %w", path, err)
	}
	return parse(string(data))
}

func parse(content string) (*Article, error) {
	yamlPart, body, err := split(content)
	if err != nil {
		return nil, err
	}
	if yamlPart == "" {
		return &Article{Body: body}, nil
	}
	fm, err := parseFrontmatter(yamlPart)
	if err != nil {
		return nil, err
	}
	return &Article{Frontmatter: *fm, Body: body}, nil
}

// Write writes an Article to the Markdown file at path, creating parent directories as needed.
func Write(path string, a *Article) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	header, err := RenderFrontmatter(&a.Frontmatter)
	if err != nil {
		return err
	}
	content := header + a.Body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write article %s: %w", path, err)
	}
	return nil
}

var unsafeChars = regexp.MustCompile(`[/\\:*?"<>|]`)

// SanitizeFilename replaces spaces with hyphens and other unsafe characters with underscores.
func SanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	return unsafeChars.ReplaceAllString(name, "_")
}

// GenerateFilename produces a canonical filename for an article.
// Format: [draft_]<YYYYmmdd>_<sanitized-title>.md
func GenerateFilename(title string, date time.Time, draft bool) string {
	var b strings.Builder
	if draft {
		b.WriteString("draft_")
	}
	b.WriteString(date.Format("20060102"))
	b.WriteString("_")
	b.WriteString(SanitizeFilename(title))
	b.WriteString(".md")
	return b.String()
}

// FromEntry creates an Article from a hatena.Entry.
func FromEntry(e *hatena.Entry) *Article {
	fm := Frontmatter{
		Title:         e.Title,
		Date:          e.Date,
		Draft:         e.Draft,
		Category:      e.Categories,
		URL:           e.URL,
		EditURL:       e.EditURL,
		CustomURLPath: e.CustomURL,
	}
	if !e.ScheduledAt.IsZero() {
		fm.ScheduledAt = &e.ScheduledAt
	}
	return &Article{Frontmatter: fm, Body: e.Content}
}

// ToEntry converts the Article to a hatena.Entry.
func (a *Article) ToEntry() *hatena.Entry {
	e := &hatena.Entry{
		Title:      a.Frontmatter.Title,
		Content:    a.Body,
		Date:       a.Frontmatter.Date,
		Draft:      a.Frontmatter.Draft,
		Categories: a.Frontmatter.Category,
		URL:        a.Frontmatter.URL,
		EditURL:    a.Frontmatter.EditURL,
		CustomURL:  a.Frontmatter.CustomURLPath,
	}
	if a.Frontmatter.ScheduledAt != nil {
		e.ScheduledAt = *a.Frontmatter.ScheduledAt
	}
	return e
}
