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

// TestSync_NoEditURL verifies that sync returns an error when the file has no editUrl.
func TestSync_NoEditURL(t *testing.T) {
	fm := article.Frontmatter{
		Title: "No EditURL",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}
	path, _ := setupPushTest(t, "", fm, "local body\n")

	cmd := newSyncCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing editUrl, got nil")
	}
	if !strings.Contains(err.Error(), "editUrl is missing from frontmatter") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSync_NoDifferences verifies that sync prints "No changes." when local and remote match.
func TestSync_NoDifferences(t *testing.T) {
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

	cmd := newSyncCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if !strings.Contains(out.String(), "No changes.") {
		t.Errorf("expected 'No changes.', got: %s", out.String())
	}
}

// TestSync_WithDifferences_Confirm verifies that answering 'y' after sync overwrites the local file
// with remote content and prints "Updated: <path>".
func TestSync_WithDifferences_Confirm(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/21", func(w http.ResponseWriter, r *http.Request) {
		writeEntryXMLFull(w, "Title", "remote body\n", false,
			fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/21", r.Host),
			"https://example.com/entry/21")
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

	cmd := newSyncCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync failed: %v\noutput: %s", err, out.String())
	}

	if !strings.Contains(out.String(), "Updated: "+path) {
		t.Errorf("expected 'Updated: %s' in output, got: %s", path, out.String())
	}

	// Verify the file was overwritten with remote content.
	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read after sync: %v", err)
	}
	if a.Body != "remote body\n" {
		t.Errorf("expected body 'remote body\\n', got: %q", a.Body)
	}
}

// TestGlobMD_SkipsHiddenDirs verifies that globMD skips directories whose names start with ".".
func TestGlobMD_SkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()

	// Files that should be included.
	mustCreate := func(path string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	mustCreate(filepath.Join(root, "visible.md"))
	mustCreate(filepath.Join(root, "subdir", "nested.md"))
	// Files that should be excluded (inside hidden directories).
	mustCreate(filepath.Join(root, ".hidden", "secret.md"))
	mustCreate(filepath.Join(root, ".git", "config.md"))

	got, err := globMD(root)
	if err != nil {
		t.Fatalf("globMD: %v", err)
	}

	want := map[string]bool{
		filepath.Join(root, "visible.md"):        true,
		filepath.Join(root, "subdir", "nested.md"): true,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d files, got %d: %v", len(want), len(got), got)
	}
	for _, f := range got {
		if !want[f] {
			t.Errorf("unexpected file in result: %s", f)
		}
	}
}

// TestSync_WithDifferences_Abort verifies that answering 'N' after sync leaves the local file unchanged
// and prints "Aborted.".
func TestSync_WithDifferences_Abort(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/22", func(w http.ResponseWriter, r *http.Request) {
		writeEntryXMLFull(w, "Title", "remote body\n", false,
			fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/22", r.Host),
			"https://example.com/entry/22")
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

	cmd := newSyncCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("N\n"))
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync failed: %v", err)
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
