package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestListCmd_VerboseFlag(t *testing.T) {
	t.Run("verbose shows per-file warning", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "bad.md", "---\ntitle: [broken\n")

		root := NewRootCmd()
		var errBuf bytes.Buffer
		root.SetErr(&errBuf)
		root.SetOut(&bytes.Buffer{})
		root.SetArgs([]string{"list", "--dir", dir, "--verbose"})

		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(errBuf.String(), "warning:") {
			t.Errorf("expected warning on stderr with --verbose, got %q", errBuf.String())
		}
	})

	t.Run("no verbose shows summary only", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "bad.md", "---\ntitle: [broken\n")

		root := NewRootCmd()
		var errBuf bytes.Buffer
		root.SetErr(&errBuf)
		root.SetOut(&bytes.Buffer{})
		root.SetArgs([]string{"list", "--dir", dir})

		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(errBuf.String(), "failed to read") {
			t.Errorf("expected no per-file warning without --verbose, got %q", errBuf.String())
		}
		if !strings.Contains(errBuf.String(), "1 file(s) skipped due to read errors") {
			t.Errorf("expected summary warning without --verbose, got %q", errBuf.String())
		}
	})
}

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

		if err := runList(cmd, dir, false, false, false, "", false); err != nil {
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

		if err := runList(cmd, dir, false, false, false, "", false); err != nil {
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

		if err := runList(cmd, dir, true, false, false, "", false); err != nil {
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

		if err := runList(cmd, dir, false, true, false, "", false); err != nil {
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

	t.Run("invalid frontmatter is skipped with warning", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "valid.md", "---\ntitle: Valid\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\n")
		writeMD(t, dir, "bad.md", "---\ntitle: [broken\n")

		var out, errBuf bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)

		if err := runList(cmd, dir, false, false, true, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(errBuf.String(), "warning:") {
			t.Errorf("expected warning in stderr, got %q", errBuf.String())
		}
		if !strings.Contains(out.String(), "Valid") {
			t.Errorf("expected valid article in output, got %q", out.String())
		}
	})

	t.Run("read error is suppressed without verbose", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "bad.md", "---\ntitle: [broken\n")
		writeMD(t, dir, "valid.md", "---\ntitle: Valid\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\n")

		var out, errBuf bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)

		if err := runList(cmd, dir, false, false, false, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(errBuf.String(), "failed to read") {
			t.Errorf("expected no per-file warning without verbose, got %q", errBuf.String())
		}
		if !strings.Contains(errBuf.String(), "1 file(s) skipped due to read errors") {
			t.Errorf("expected summary warning without verbose, got %q", errBuf.String())
		}
		if !strings.Contains(out.String(), "Valid") {
			t.Errorf("expected valid article in output, got %q", out.String())
		}
	})

	t.Run("draft and published conflict", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		err := runList(cmd, t.TempDir(), true, true, false, "", false)
		if err == nil {
			t.Fatal("expected error for --draft + --published")
		}
		if !strings.Contains(err.Error(), "cannot be used together") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("category filter shows only matching articles", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "go.md", "---\ntitle: Go Article\ndate: 2026-01-01T00:00:00Z\ndraft: false\ncategory:\n  - Go\n---\n")
		writeMD(t, dir, "blog.md", "---\ntitle: Blog Article\ndate: 2026-02-01T00:00:00Z\ndraft: false\ncategory:\n  - Blog\n---\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runList(cmd, dir, false, false, false, "Go", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Go Article") {
			t.Errorf("expected Go Article in output, got %q", out)
		}
		if strings.Contains(out, "Blog Article") {
			t.Errorf("expected Blog Article to be filtered out, got %q", out)
		}
	})

	t.Run("categories lists category counts sorted by count desc", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "a.md", "---\ntitle: A\ndate: 2026-01-01T00:00:00Z\ndraft: false\ncategory:\n  - Go\n---\n")
		writeMD(t, dir, "b.md", "---\ntitle: B\ndate: 2026-02-01T00:00:00Z\ndraft: false\ncategory:\n  - Go\n---\n")
		writeMD(t, dir, "c.md", "---\ntitle: C\ndate: 2026-03-01T00:00:00Z\ndraft: false\ncategory:\n  - Blog\n---\n")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&bytes.Buffer{})

		if err := runList(cmd, dir, false, false, false, "", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Go") || !strings.Contains(out, "Blog") {
			t.Errorf("expected Go and Blog in categories output, got %q", out)
		}
		goIdx := strings.Index(out, "Go")
		blogIdx := strings.Index(out, "Blog")
		if goIdx > blogIdx {
			t.Errorf("expected Go (count=2) before Blog (count=1), got %q", out)
		}
		if !strings.Contains(out, "2") || !strings.Contains(out, "1") {
			t.Errorf("expected counts 2 and 1 in output, got %q", out)
		}
	})

	t.Run("categories conflicts with draft flag", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		err := runList(cmd, t.TempDir(), true, false, false, "", true)
		if err == nil {
			t.Fatal("expected error for --categories + --draft")
		}
		if !strings.Contains(err.Error(), "--categories cannot be used with") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("categories conflicts with published flag", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		err := runList(cmd, t.TempDir(), false, true, false, "", true)
		if err == nil {
			t.Fatal("expected error for --categories + --published")
		}
		if !strings.Contains(err.Error(), "--categories cannot be used with") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("categories conflicts with category flag", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		err := runList(cmd, t.TempDir(), false, false, false, "Go", true)
		if err == nil {
			t.Fatal("expected error for --categories + --category")
		}
		if !strings.Contains(err.Error(), "--categories cannot be used with") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}
