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
	"github.com/hirano00o/hb/hatena"
)

func setupDeleteTest(t *testing.T, editURL string) string {
	t.Helper()
	dir := t.TempDir()
	a := &article.Article{
		Frontmatter: article.Frontmatter{
			Title:   "Test Entry",
			Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			EditURL: editURL,
			URL:     "https://example.com/entry/1",
		},
		Body: "body\n",
	}
	p := filepath.Join(dir, "test.md")
	if err := article.Write(p, a); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDelete_Confirm_Deletes verifies that confirming the prompt issues a DELETE request
// and prints a success message.
func TestDelete_Confirm_Deletes(t *testing.T) {
	deleteCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deleteCalled = true
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/1"
	path := setupDeleteTest(t, editURL)

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newDeleteCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete failed: %v\noutput: %s", err, out.String())
	}

	if !deleteCalled {
		t.Fatal("expected DELETE request")
	}
	if !strings.Contains(out.String(), "Deleted:") {
		t.Errorf("expected 'Deleted:' in output, got: %s", out.String())
	}
}

// TestDelete_Abort_NoRequest verifies that answering 'N' aborts without issuing a DELETE.
func TestDelete_Abort_NoRequest(t *testing.T) {
	deleteCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/2"
	path := setupDeleteTest(t, editURL)

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newDeleteCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("N\n"))
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if deleteCalled {
		t.Error("expected no DELETE request after aborting")
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected 'Aborted' in output, got: %s", out.String())
	}
}

// TestDelete_YesFlag_SkipsPrompt verifies that --yes skips the confirmation prompt.
func TestDelete_YesFlag_SkipsPrompt(t *testing.T) {
	deleteCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/3", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/3"
	path := setupDeleteTest(t, editURL)

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newDeleteCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("")) // no stdin — would block if prompt is shown
	cmd.SetArgs([]string{"--yes", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deleteCalled {
		t.Error("expected DELETE request with --yes flag")
	}
}

// TestDelete_RemoveLocal_RemovesFile verifies that --remove-local deletes the local file
// after a successful remote deletion.
func TestDelete_RemoveLocal_RemovesFile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/4", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/4"
	path := setupDeleteTest(t, editURL)

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newDeleteCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--yes", "--remove-local", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected local file to be removed, but it still exists")
	}
	if !strings.Contains(out.String(), "Removed local:") {
		t.Errorf("expected 'Removed local:' in output, got: %s", out.String())
	}
}

// TestDelete_NoEditURL_ReturnsError verifies that a file with no edit_url returns an error.
func TestDelete_NoEditURL_ReturnsError(t *testing.T) {
	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	dir := t.TempDir()
	a := &article.Article{
		Frontmatter: article.Frontmatter{
			Title: "No editUrl",
			Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			// EditURL intentionally empty
		},
		Body: "body\n",
	}
	p := filepath.Join(dir, "no-edit-url.md")
	if err := article.Write(p, a); err != nil {
		t.Fatal(err)
	}

	cmd := newDeleteCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{p})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for file with no edit_url, got nil")
	}
	if !strings.Contains(fmt.Sprint(err), "edit_url") {
		t.Errorf("expected error to mention edit_url, got: %v", err)
	}
}
