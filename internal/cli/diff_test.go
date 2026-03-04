package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
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
