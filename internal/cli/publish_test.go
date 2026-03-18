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

func setupPublishTest(t *testing.T, name string, fm article.Frontmatter, body string) string {
	t.Helper()
	dir := t.TempDir()
	a := &article.Article{Frontmatter: fm, Body: body}
	p := filepath.Join(dir, name)
	if err := article.Write(p, a); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPublish_DraftFile_RemovesDraftPrefix(t *testing.T) {
	fm := article.Frontmatter{
		Title: "My Post",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: true,
	}
	path := setupPublishTest(t, "draft_20260301_My-Post.md", fm, "body\n")

	cmd := newPublishCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old file must be gone.
	if _, err := os.Stat(path); err == nil {
		t.Error("expected old draft_ file to be removed")
	}

	// New file must exist with draft=false.
	newPath := filepath.Join(filepath.Dir(path), "20260301_My-Post.md")
	updated, err := article.Read(newPath)
	if err != nil {
		t.Fatalf("read published file: %v", err)
	}
	if updated.Frontmatter.Draft {
		t.Error("expected draft=false after publish")
	}
	if !strings.Contains(out.String(), "Published:") {
		t.Errorf("expected 'Published:' in output, got: %s", out.String())
	}
}

func TestPublish_NoDraftPrefix_NoRename(t *testing.T) {
	fm := article.Frontmatter{
		Title: "My Post",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: true,
	}
	path := setupPublishTest(t, "my-post.md", fm, "body\n")

	cmd := newPublishCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File must still exist at original path.
	updated, err := article.Read(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if updated.Frontmatter.Draft {
		t.Error("expected draft=false after publish")
	}
}

func TestUnpublish_AddsDraftPrefix(t *testing.T) {
	fm := article.Frontmatter{
		Title: "My Post",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: false,
	}
	path := setupPublishTest(t, "20260301_My-Post.md", fm, "body\n")

	cmd := newUnpublishCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old file must be gone.
	if _, err := os.Stat(path); err == nil {
		t.Error("expected old file to be removed")
	}

	// New file must exist with draft=true.
	newPath := filepath.Join(filepath.Dir(path), "draft_20260301_My-Post.md")
	updated, err := article.Read(newPath)
	if err != nil {
		t.Fatalf("read unpublished file: %v", err)
	}
	if !updated.Frontmatter.Draft {
		t.Error("expected draft=true after unpublish")
	}
	if !strings.Contains(out.String(), "Unpublished:") {
		t.Errorf("expected 'Unpublished:' in output, got: %s", out.String())
	}
}

func TestUnpublish_AlreadyHasDraftPrefix_NoRename(t *testing.T) {
	fm := article.Frontmatter{
		Title: "My Post",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: false,
	}
	path := setupPublishTest(t, "draft_post.md", fm, "body\n")

	cmd := newUnpublishCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File must still exist at original path.
	updated, err := article.Read(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !updated.Frontmatter.Draft {
		t.Error("expected draft=true after unpublish")
	}
}

func TestPublish_ConflictError(t *testing.T) {
	dir := t.TempDir()
	fm := article.Frontmatter{
		Title: "My Post",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: true,
	}
	// Create the draft_ file.
	draftPath := filepath.Join(dir, "draft_post.md")
	a := &article.Article{Frontmatter: fm, Body: "body\n"}
	if err := article.Write(draftPath, a); err != nil {
		t.Fatal(err)
	}
	// Create the destination file (without draft_ prefix) to trigger conflict.
	if err := os.WriteFile(filepath.Join(dir, "post.md"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newPublishCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{draftPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for destination conflict")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}
