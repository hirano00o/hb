package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func makeArticle(title, body string, draft bool) *article.Article {
	return &article.Article{
		Frontmatter: article.Frontmatter{
			Title: title,
			Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			Draft: draft,
		},
		Body: body,
	}
}

// TestHasChanges verifies that hasChanges detects field-level differences.
func TestHasChanges(t *testing.T) {
	base := makeArticle("Title", "body\n", false)

	if hasChanges(base, base) {
		t.Error("identical articles should have no changes")
	}

	different := makeArticle("Title", "body\n", false)
	different.Body = "different\n"
	if !hasChanges(base, different) {
		t.Error("expected body change to be detected")
	}

	different2 := makeArticle("Other", "body\n", false)
	if !hasChanges(base, different2) {
		t.Error("expected title change to be detected")
	}

	different3 := makeArticle("Title", "body\n", true)
	if !hasChanges(base, different3) {
		t.Error("expected draft change to be detected")
	}
}

// TestArticleToString verifies that articleToString renders frontmatter + body.
func TestArticleToString(t *testing.T) {
	a := makeArticle("Hello", "content here\n", false)
	s, err := articleToString(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(s, "---\n") {
		t.Errorf("expected --- prefix, got %q", s)
	}
	if !strings.Contains(s, "title: Hello") {
		t.Errorf("expected title in output, got %q", s)
	}
	if !strings.HasSuffix(s, "content here\n") {
		t.Errorf("expected body at end, got %q", s)
	}
}

// TestUnifiedDiff verifies diff output for changed and unchanged articles.
func TestUnifiedDiff(t *testing.T) {
	a := makeArticle("Title", "line1\nline2\n", false)
	b := makeArticle("Title", "line1\nline3\n", false)

	diff, err := unifiedDiff("test.md", a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff == "" {
		t.Error("expected non-empty diff for different articles")
	}
	if !strings.Contains(diff, "-line2") {
		t.Errorf("expected removed line2 in diff, got %q", diff)
	}

	same, err := unifiedDiff("test.md", a, a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if same != "" {
		t.Errorf("expected empty diff for identical articles, got %q", same)
	}
}

// TestGlobMD verifies that only .md files are returned.
func TestGlobMD(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "d.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := globMD(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 .md files, got %d: %v", len(files), files)
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			t.Errorf("non-.md file returned: %s", f)
		}
	}
}

// TestGlobMD_Empty verifies that an empty directory returns no files.
func TestGlobMD_Empty(t *testing.T) {
	dir := t.TempDir()
	files, err := globMD(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no files, got %v", files)
	}
}

// TestCollectLocalEditURLs verifies editUrl collection and warning output for unreadable files.
func TestCollectLocalEditURLs(t *testing.T) {
	dir := t.TempDir()

	// valid article with editUrl
	validContent := "---\ntitle: Test\ndate: 2026-03-01T00:00:00Z\neditUrl: https://example.com/entry/1\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "valid.md"), []byte(validContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// malformed frontmatter (unreadable as article)
	badContent := "---\ntitle: [broken\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte(badContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	known, err := collectLocalEditURLs(dir, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := known["https://example.com/entry/1"]; !ok {
		t.Errorf("expected editUrl to be collected, got %v", known)
	}
	if !strings.Contains(buf.String(), "warning:") {
		t.Errorf("expected warning for bad.md, got %q", buf.String())
	}
}

// TestConfirmAction verifies confirmAction returns the correct boolean for various inputs.
func TestConfirmAction(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"n\n", false},
		{"N\n", false},
		{"\n", false},   // empty line → no
		{"yes\n", false}, // full word not accepted
	}
	for _, tc := range cases {
		cmd := &cobra.Command{}
		cmd.SetIn(strings.NewReader(tc.input))
		var out bytes.Buffer
		cmd.SetOut(&out)
		got, err := confirmAction(cmd, "Confirm? [y/N]: ")
		if err != nil {
			t.Errorf("input %q: unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("input %q: got %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestConfirmAction_EOF verifies confirmAction returns false (no error) on EOF.
func TestConfirmAction_EOF(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	var out bytes.Buffer
	cmd.SetOut(&out)
	got, err := confirmAction(cmd, "Confirm? [y/N]: ")
	if err != nil {
		t.Errorf("unexpected error on EOF: %v", err)
	}
	if got {
		t.Error("expected false on EOF")
	}
}

// TestResolveConflict_NoExist verifies that a non-existent path is returned as-is.
func TestResolveConflict_NoExist(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	var out bytes.Buffer
	cmd.SetOut(&out)

	dest, skip, err := resolveConflict(cmd, filepath.Join(t.TempDir(), "nonexistent.md"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Error("expected no skip for non-existent file")
	}
	if !strings.HasSuffix(dest, "nonexistent.md") {
		t.Errorf("expected original path, got %s", dest)
	}
}

// TestResolveConflict_Skip verifies that 's' input causes a skip.
func TestResolveConflict_Skip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("s\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	_, skip, err := resolveConflict(cmd, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Error("expected skip for 's' input")
	}
}

// TestResolveConflict_CustomName verifies that a user-provided name is used.
func TestResolveConflict_CustomName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("renamed.md\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	dest, skip, err := resolveConflict(cmd, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Error("expected no skip")
	}
	if !strings.HasSuffix(dest, "renamed.md") {
		t.Errorf("expected renamed.md, got %s", dest)
	}
}

// TestResolveConflict_AutoRename verifies that empty input triggers auto-rename.
func TestResolveConflict_AutoRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	dest, skip, err := resolveConflict(cmd, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Error("expected no skip")
	}
	if dest == path {
		t.Error("expected auto-renamed path, got original")
	}
	if !strings.HasSuffix(dest, ".md") {
		t.Errorf("expected .md extension, got %s", dest)
	}
}

// TestResolveConflict_Force verifies that --force triggers auto-rename without prompting.
func TestResolveConflict_Force(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	// no input; force should not read from stdin
	cmd.SetIn(strings.NewReader(""))
	var out bytes.Buffer
	cmd.SetOut(&out)

	dest, skip, err := resolveConflict(cmd, path, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Error("expected no skip for force")
	}
	if dest == path {
		t.Error("expected auto-renamed path, got original")
	}
}

// TestResolveConflict_CustomName_PathTraversal verifies that a path traversal attempt
// in the custom name is neutralised and the file is placed in the original directory.
func TestResolveConflict_CustomName_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("../../etc/passwd\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	dest, skip, err := resolveConflict(cmd, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Error("expected no skip")
	}
	// Must stay inside the original directory.
	if filepath.Dir(dest) != dir {
		t.Errorf("path escaped directory: got %s, want inside %s", dest, dir)
	}
	if filepath.Base(dest) != "passwd" {
		t.Errorf("expected base name 'passwd', got %s", filepath.Base(dest))
	}
}
