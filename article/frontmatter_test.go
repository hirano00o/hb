package article

import (
	"strings"
	"testing"
	"time"
)

func TestSplit_WithFrontmatter(t *testing.T) {
	content := "---\ntitle: hello\n---\nbody text\n"
	yaml, body, err := split(content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yaml, "title: hello") {
		t.Errorf("yaml part: got %q", yaml)
	}
	if body != "body text\n" {
		t.Errorf("body: got %q", body)
	}
}

func TestSplit_NoFrontmatter(t *testing.T) {
	content := "just body\n"
	yaml, body, err := split(content)
	if err != nil {
		t.Fatal(err)
	}
	if yaml != "" {
		t.Errorf("expected empty yaml, got %q", yaml)
	}
	if body != content {
		t.Errorf("body should equal input, got %q", body)
	}
}

func TestSplit_UnclosedFrontmatter(t *testing.T) {
	content := "---\ntitle: no close\n"
	_, _, err := split(content)
	if err == nil {
		t.Error("expected error for unclosed frontmatter")
	}
}

func TestSplit_ClosingDashNotDelimiter(t *testing.T) {
	// "---extra" must NOT be treated as a closing delimiter
	content := "---\ntitle: hello\n---extra\n---\nbody text\n"
	yaml, body, err := split(content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yaml, "title: hello") {
		t.Errorf("yaml part: got %q", yaml)
	}
	if body != "body text\n" {
		t.Errorf("body: got %q", body)
	}
}

func TestParseFrontmatter(t *testing.T) {
	yaml := "title: Test\ndate: 2026-03-01T12:00:00+09:00\ndraft: false\ncategory:\n  - Go\n"
	fm, err := parseFrontmatter(yaml)
	if err != nil {
		t.Fatal(err)
	}
	if fm.Title != "Test" {
		t.Errorf("title: got %q", fm.Title)
	}
	if fm.Draft {
		t.Error("draft should be false")
	}
	if len(fm.Category) != 1 || fm.Category[0] != "Go" {
		t.Errorf("category: got %v", fm.Category)
	}
	wantDate := time.Date(2026, 3, 1, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	if !fm.Date.Equal(wantDate) {
		t.Errorf("date: got %v, want %v", fm.Date, wantDate)
	}
}

func TestParseFrontmatter_Invalid(t *testing.T) {
	_, err := parseFrontmatter("title: [broken")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestRenderFrontmatter(t *testing.T) {
	fm := &Frontmatter{Title: "Hello", Draft: true}
	out, err := RenderFrontmatter(fm)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "---\n") {
		t.Errorf("should start with ---: %q", out)
	}
	if !strings.Contains(out, "title: Hello") {
		t.Errorf("missing title: %q", out)
	}
	if !strings.HasSuffix(out, "---\n") {
		t.Errorf("should end with ---: %q", out)
	}
}
