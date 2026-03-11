package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunOpen(t *testing.T) {
	// helper: create a temp markdown file with given frontmatter content.
	writeArticle := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		p := filepath.Join(dir, "test.md")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("opens browser with valid URL", func(t *testing.T) {
		orig := openBrowser
		defer func() { openBrowser = orig }()

		var captured string
		openBrowser = func(u string) error {
			captured = u
			return nil
		}

		path := writeArticle(t, "---\ntitle: Test\nurl: https://example.com/entry/1\n---\nbody\n")

		cmd := newOpenCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{path})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if captured != "https://example.com/entry/1" {
			t.Errorf("expected URL https://example.com/entry/1, got %q", captured)
		}
		if !strings.Contains(out.String(), "Opened:") {
			t.Errorf("expected Opened message, got: %s", out.String())
		}
	})

	t.Run("error when URL is empty", func(t *testing.T) {
		path := writeArticle(t, "---\ntitle: Draft\n---\nbody\n")

		cmd := newOpenCmd()
		cmd.SetArgs([]string{path})
		cmd.SilenceUsage = true
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "no URL found") {
			t.Errorf("expected 'no URL found' error, got: %v", err)
		}
	})

	t.Run("error when file does not exist", func(t *testing.T) {
		cmd := newOpenCmd()
		cmd.SetArgs([]string{"/nonexistent/path/article.md"})
		cmd.SilenceUsage = true
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
