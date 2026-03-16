package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
)

func TestRename_UpdatesTitleAndFilename(t *testing.T) {
	dir := t.TempDir()
	fm := article.Frontmatter{
		Title: "Old Title",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: false,
	}
	oldPath := filepath.Join(dir, "20260301_Old-Title.md")
	a := &article.Article{Frontmatter: fm, Body: "body\n"}
	if err := article.Write(oldPath, a); err != nil {
		t.Fatal(err)
	}

	cmd := newRenameCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--title", "New Title", oldPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old file must be gone.
	if _, err := os.Stat(oldPath); err == nil {
		t.Error("expected old file to be removed")
	}

	// New file must exist with updated title.
	newPath := filepath.Join(dir, "20260301_New-Title.md")
	updated, err := article.Read(newPath)
	if err != nil {
		t.Fatalf("read renamed file: %v", err)
	}
	if updated.Frontmatter.Title != "New Title" {
		t.Errorf("expected title 'New Title', got %q", updated.Frontmatter.Title)
	}
	if !strings.Contains(out.String(), "Renamed:") {
		t.Errorf("expected 'Renamed:' in output, got: %s", out.String())
	}
}

func TestRename_ConflictError(t *testing.T) {
	dir := t.TempDir()
	fm := article.Frontmatter{
		Title: "Old Title",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: false,
	}
	oldPath := filepath.Join(dir, "20260301_Old-Title.md")
	a := &article.Article{Frontmatter: fm, Body: "body\n"}
	if err := article.Write(oldPath, a); err != nil {
		t.Fatal(err)
	}
	// Create the destination to trigger conflict.
	if err := os.WriteFile(filepath.Join(dir, "20260301_New-Title.md"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRenameCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--title", "New Title", oldPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for destination conflict")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRename_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	fm := article.Frontmatter{
		Title: "Old Title",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: false,
	}
	oldPath := filepath.Join(dir, "20260301_Old-Title.md")
	a := &article.Article{Frontmatter: fm, Body: "body\n"}
	if err := article.Write(oldPath, a); err != nil {
		t.Fatal(err)
	}
	// Create the destination file.
	if err := os.WriteFile(filepath.Join(dir, "20260301_New-Title.md"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRenameCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--title", "New Title", "--force", oldPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}

	// New file must have updated title.
	newPath := filepath.Join(dir, "20260301_New-Title.md")
	updated, err := article.Read(newPath)
	if err != nil {
		t.Fatalf("read renamed file: %v", err)
	}
	if updated.Frontmatter.Title != "New Title" {
		t.Errorf("expected title 'New Title', got %q", updated.Frontmatter.Title)
	}
}

func TestRename_MissingTitleFlag(t *testing.T) {
	dir := t.TempDir()
	cmd := newRenameCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{filepath.Join(dir, "some.md")})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --title is missing")
	}
}
