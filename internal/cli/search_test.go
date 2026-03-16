package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunSearch(t *testing.T) {
	t.Run("matches title", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "a.md", "---\ntitle: Go Programming\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\nsome body text\n")
		writeMD(t, dir, "b.md", "---\ntitle: Other Article\ndate: 2026-02-01T00:00:00Z\ndraft: false\n---\nanother body\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runSearch(cmd, "Go", dir, false, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Go Programming") {
			t.Errorf("expected Go Programming in output, got %q", out)
		}
		if strings.Contains(out, "Other Article") {
			t.Errorf("expected Other Article to be excluded, got %q", out)
		}
	})

	t.Run("matches body", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "a.md", "---\ntitle: Article One\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\nThis is about golang\n")
		writeMD(t, dir, "b.md", "---\ntitle: Article Two\ndate: 2026-02-01T00:00:00Z\ndraft: false\n---\nThis is about python\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runSearch(cmd, "golang", dir, false, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Article One") {
			t.Errorf("expected Article One in output, got %q", out)
		}
		if strings.Contains(out, "Article Two") {
			t.Errorf("expected Article Two to be excluded, got %q", out)
		}
	})

	t.Run("title flag excludes body-only matches", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "a.md", "---\ntitle: Great Title\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\nno keyword here\n")
		writeMD(t, dir, "b.md", "---\ntitle: Normal Title\ndate: 2026-02-01T00:00:00Z\ndraft: false\n---\nkeyword is in the body\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		// --title flag: search title only; "keyword" appears in body of b but not title
		if err := runSearch(cmd, "keyword", dir, true, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if strings.Contains(out, "Normal Title") {
			t.Errorf("expected Normal Title to be excluded with --title flag, got %q", out)
		}
	})

	t.Run("body flag excludes title-only matches", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "a.md", "---\ntitle: Keyword In Title\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\nno match in body\n")
		writeMD(t, dir, "b.md", "---\ntitle: Other Article\ndate: 2026-02-01T00:00:00Z\ndraft: false\n---\nkeyword is here in body\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		// --body flag: search body only; "keyword" appears in title of a but not body
		if err := runSearch(cmd, "keyword", dir, false, true, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if strings.Contains(out, "Keyword In Title") {
			t.Errorf("expected Keyword In Title to be excluded with --body flag, got %q", out)
		}
		if !strings.Contains(out, "Other Article") {
			t.Errorf("expected Other Article in output, got %q", out)
		}
	})

	t.Run("case insensitive search", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "a.md", "---\ntitle: Hello World\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\nsome text\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runSearch(cmd, "hello", dir, false, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "Hello World") {
			t.Errorf("expected case-insensitive match, got %q", buf.String())
		}
	})

	t.Run("no matches returns no articles found", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "a.md", "---\ntitle: Unrelated\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\nunrelated body\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runSearch(cmd, "nonexistent", dir, false, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.String() != "No articles found.\n" {
			t.Errorf("expected 'No articles found.\\n', got %q", buf.String())
		}
	})
}
