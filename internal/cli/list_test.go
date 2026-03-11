package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func writeMD(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunList(t *testing.T) {
	t.Run("no files", func(t *testing.T) {
		dir := t.TempDir()
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runList(cmd, dir, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.String() != "No articles found.\n" {
			t.Errorf("expected 'No articles found.\\n', got %q", buf.String())
		}
	})

	t.Run("sorted by date descending", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "old.md", "---\ntitle: Old\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\n")
		writeMD(t, dir, "new.md", "---\ntitle: New\ndate: 2026-03-01T00:00:00Z\ndraft: false\n---\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runList(cmd, dir, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		newIdx := strings.Index(out, "New")
		oldIdx := strings.Index(out, "Old")
		if newIdx < 0 || oldIdx < 0 {
			t.Fatalf("expected both articles in output, got %q", out)
		}
		if newIdx > oldIdx {
			t.Errorf("expected New before Old (date desc), got %q", out)
		}
	})

	t.Run("draft only", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "pub.md", "---\ntitle: Published\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\n")
		writeMD(t, dir, "draft.md", "---\ntitle: Draft\ndate: 2026-02-01T00:00:00Z\ndraft: true\n---\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runList(cmd, dir, true, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Draft") {
			t.Errorf("expected Draft in output, got %q", out)
		}
		if strings.Contains(out, "Published") {
			t.Errorf("expected Published to be filtered out, got %q", out)
		}
	})

	t.Run("published only", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "pub.md", "---\ntitle: Published\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\n")
		writeMD(t, dir, "draft.md", "---\ntitle: Draft\ndate: 2026-02-01T00:00:00Z\ndraft: true\n---\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runList(cmd, dir, false, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Published") {
			t.Errorf("expected Published in output, got %q", out)
		}
		if strings.Contains(out, "Draft") {
			t.Errorf("expected Draft to be filtered out, got %q", out)
		}
	})

	t.Run("draft and published conflict", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		err := runList(cmd, t.TempDir(), true, true)
		if err == nil {
			t.Fatal("expected error for --draft + --published")
		}
		if !strings.Contains(err.Error(), "cannot be used together") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}
