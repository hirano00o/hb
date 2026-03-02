package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
)

// TestFetch_NoEditURL verifies that fetch returns an error when the file has no editUrl.
func TestFetch_NoEditURL(t *testing.T) {
	fm := article.Frontmatter{
		Title: "No EditURL",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}
	path, _ := setupPushTest(t, "", fm, "local body\n")

	cmd := newFetchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing editUrl, got nil")
	}
	if !strings.Contains(err.Error(), "has no editUrl in frontmatter") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestFetch_NoDifferences verifies that fetch prints "No changes." when local and remote match.
func TestFetch_NoDifferences(t *testing.T) {
	const entryID = "20"
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

	cmd := newFetchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if !strings.Contains(out.String(), "No changes.") {
		t.Errorf("expected 'No changes.', got: %s", out.String())
	}
}

// TestFetch_WithDifferences_Confirm verifies that answering 'y' overwrites the local file
// with remote content and prints "Updated: <path>".
func TestFetch_WithDifferences_Confirm(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/21", func(w http.ResponseWriter, r *http.Request) {
		writeEntryXML(w, "Title", "remote body\n", false)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/21"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/21",
	}
	path, _ := setupPushTest(t, editURL, fm, "local body\n")

	cmd := newFetchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fetch failed: %v\noutput: %s", err, out.String())
	}

	if !strings.Contains(out.String(), "Updated: "+path) {
		t.Errorf("expected 'Updated: %s' in output, got: %s", path, out.String())
	}

	// Verify the file was overwritten with remote content.
	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read after fetch: %v", err)
	}
	if a.Body != "remote body\n" {
		t.Errorf("expected body 'remote body\\n', got: %q", a.Body)
	}
}

// TestFetch_WithDifferences_Abort verifies that answering 'N' leaves the local file unchanged
// and prints "Aborted.".
func TestFetch_WithDifferences_Abort(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/22", func(w http.ResponseWriter, r *http.Request) {
		writeEntryXML(w, "Title", "remote body\n", false)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/22"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/22",
	}
	path, _ := setupPushTest(t, editURL, fm, "local body\n")

	cmd := newFetchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("N\n"))
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	if !strings.Contains(out.String(), "Aborted.") {
		t.Errorf("expected 'Aborted.' in output, got: %s", out.String())
	}

	// Verify the local file was NOT overwritten.
	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read after abort: %v", err)
	}
	if a.Body != "local body\n" {
		t.Errorf("expected body 'local body\\n' unchanged, got: %q", a.Body)
	}
}
