package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
)

// TestDiff_NoEditURL_NewEntry verifies that diff prints a message for unpublished entries
// and exits without error.
func TestDiff_NoEditURL_NewEntry(t *testing.T) {
	fm := article.Frontmatter{
		Title: "Unpublished",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}
	path, _ := setupPushTest(t, "", fm, "local body\n")

	cmd := newDiffCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if !strings.Contains(out.String(), "No editUrl in frontmatter") {
		t.Errorf("expected no-editUrl message, got: %s", out.String())
	}
}

// TestDiff_NoDifferences verifies that diff prints "No differences." when local and remote match.
func TestDiff_NoDifferences(t *testing.T) {
	const entryID = "10"
	var srvURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/"+entryID, func(w http.ResponseWriter, r *http.Request) {
		editHref := srvURL + "/user/example.hateblo.jp/atom/entry/" + entryID
		writeEntryXMLFull(w, "Title", "same body\n", false, editHref, "https://example.com/entry/"+entryID)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srvURL + "/user/example.hateblo.jp/atom/entry/" + entryID
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/" + entryID,
	}
	path, _ := setupPushTest(t, editURL, fm, "same body\n")

	cmd := newDiffCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if !strings.Contains(out.String(), "No differences.") {
		t.Errorf("expected 'No differences.', got: %s", out.String())
	}
}

// TestDiff_LocalImageNote verifies that diff prints a note to stderr when the local article
// contains a local image reference.
func TestDiff_LocalImageNote(t *testing.T) {
	const entryID = "20"
	var srvURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/"+entryID, func(w http.ResponseWriter, r *http.Request) {
		editHref := srvURL + "/user/example.hateblo.jp/atom/entry/" + entryID
		writeEntryXMLFull(w, "Title", "remote body\n", false, editHref, "https://example.com/entry/"+entryID)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srvURL + "/user/example.hateblo.jp/atom/entry/" + entryID
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/" + entryID,
	}
	path, _ := setupPushTest(t, editURL, fm, "![alt](photo.jpg)\n")

	cmd := newDiffCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if !strings.Contains(errBuf.String(), "note:") || !strings.Contains(errBuf.String(), "local images") {
		t.Errorf("expected local-image note in stderr, got %q", errBuf.String())
	}
}

// TestDiff_WithDifferences verifies that diff outputs a unified diff when content differs.
func TestDiff_WithDifferences(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/11", func(w http.ResponseWriter, r *http.Request) {
		writeEntryXMLFull(w, "Title", "remote body\n", false,
			fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/11", r.Host),
			"https://example.com/entry/11")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/11"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/11",
	}
	path, _ := setupPushTest(t, editURL, fm, "local body\n")

	cmd := newDiffCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	diffOut := out.String()
	if !strings.Contains(diffOut, "-local body") {
		t.Errorf("expected '-local body' in diff output, got:\n%s", diffOut)
	}
	if !strings.Contains(diffOut, "+remote body") {
		t.Errorf("expected '+remote body' in diff output, got:\n%s", diffOut)
	}
}

// TestDiff_NoArgs_Error verifies that diff with no arguments and no --all returns an error.
func TestDiff_NoArgs_Error(t *testing.T) {
	cmd := newDiffCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no arguments, got nil")
	}
	if !strings.Contains(err.Error(), "at least one file argument") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestDiff_AllAndArgs_Error verifies that --all combined with file arguments returns an error.
func TestDiff_AllAndArgs_Error(t *testing.T) {
	fm := article.Frontmatter{Title: "T", Date: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	path, _ := setupPushTest(t, "", fm, "body\n")
	cmd := newDiffCmd()
	cmd.SetArgs([]string{"--all", path})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --all with args, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestDiff_MultipleFiles_AllProcessed verifies that diff with multiple file arguments
// shows diffs for each file.
func TestDiff_MultipleFiles_AllProcessed(t *testing.T) {
	mux := http.NewServeMux()
	for _, id := range []string{"40", "41"} {
		id := id
		mux.HandleFunc("/user/example.hateblo.jp/atom/entry/"+id, func(w http.ResponseWriter, r *http.Request) {
			writeEntryXMLFull(w, "Title", "remote body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/"+id, r.Host),
				"https://example.com/entry/"+id)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	var paths []string
	for _, id := range []string{"40", "41"} {
		editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/" + id
		fm := article.Frontmatter{
			Title:   "Title",
			Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			EditURL: editURL,
			URL:     "https://example.com/entry/" + id,
		}
		p, _ := setupPushTest(t, editURL, fm, "local body\n")
		paths = append(paths, p)
	}

	cmd := newDiffCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(paths)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff failed: %v\noutput: %s", err, out.String())
	}

	// Both files should show diffs.
	if strings.Count(out.String(), "-local body") != 2 {
		t.Errorf("expected 2 '-local body' hunks in output, got:\n%s", out.String())
	}
}

// TestDiff_All_SkipsNoEditURL verifies that --all shows a "No editUrl" message for
// unpublished files and continues processing the remaining files without returning an error.
func TestDiff_All_SkipsNoEditURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/42", func(w http.ResponseWriter, r *http.Request) {
		writeEntryXMLFull(w, "Title", "remote body\n", false,
			fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/42", r.Host),
			"https://example.com/entry/42")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// File with editUrl.
	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/42"
	fmWith := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		EditURL: editURL,
		URL:     "https://example.com/entry/42",
	}
	withEdit := filepath.Join(dir, "with_edit.md")
	if err := article.Write(withEdit, &article.Article{Frontmatter: fmWith, Body: "local body\n"}); err != nil {
		t.Fatal(err)
	}

	// File without editUrl.
	fmNo := article.Frontmatter{Title: "Unpublished", Date: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	noEdit := filepath.Join(dir, "no_edit.md")
	if err := article.Write(noEdit, &article.Article{Frontmatter: fmNo, Body: "body\n"}); err != nil {
		t.Fatal(err)
	}

	cmd := newDiffCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--all"})
	// diff --all must not return an error; no-editUrl files are shown as messages, not errors.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff --all failed: %v\noutput: %s", err, out.String())
	}

	outStr := out.String()
	// The unpublished file must produce a "No editUrl" message.
	if !strings.Contains(outStr, "No editUrl in frontmatter") {
		t.Errorf("expected 'No editUrl in frontmatter' in output, got:\n%s", outStr)
	}
	// The published file must show a diff.
	if !strings.Contains(outStr, "-local body") {
		t.Errorf("expected '-local body' in diff output, got:\n%s", outStr)
	}
}
