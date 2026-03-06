package cli

import (
	"bytes"
	"fmt"
	"io"
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

// fixedNow is the time injected in new command tests.
var fixedNow = time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)

// newCmdOpts holds optional overrides for runNewCmd.
type newCmdOpts struct {
	stdin       string // content to feed as stdin (empty means no stdin override)
	stdinIsPipe bool   // stub isStdinPipe to return true
}

// runNewCmd executes newNewCmdIn(dir) with the given args.
// Returns stdout+stderr, the first created .md file path, and any error.
func runNewCmd(t *testing.T, dir string, args []string, opts ...newCmdOpts) (string, string, error) {
	t.Helper()

	origNow := timeNow
	timeNow = func() time.Time { return fixedNow }
	t.Cleanup(func() { timeNow = origNow })

	var opt newCmdOpts
	if len(opts) > 0 {
		opt = opts[0]
	}
	// Always stub isStdinPipe to prevent real pipe state from affecting tests.
	orig := isStdinPipe
	pipeResult := opt.stdinIsPipe
	isStdinPipe = func() bool { return pipeResult }
	t.Cleanup(func() { isStdinPipe = orig })

	cmd := newNewCmdIn(dir)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if opt.stdin != "" {
		cmd.SetIn(strings.NewReader(opt.stdin))
	}
	cmd.SetArgs(args)
	runErr := cmd.Execute()

	// Find created .md file in dir (if any).
	var created string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			created = filepath.Join(dir, e.Name())
			break
		}
	}
	return out.String(), created, runErr
}

// TestNew_BasicCreate verifies basic file creation with correct filename and frontmatter.
func TestNew_BasicCreate(t *testing.T) {
	dir := t.TempDir()
	out, path, err := runNewCmd(t, dir, []string{"-t", "My First Post"})
	if err != nil {
		t.Fatalf("new failed: %v\noutput: %s", err, out)
	}

	wantName := "20260306_My-First-Post.md"
	if filepath.Base(path) != wantName {
		t.Errorf("filename = %q, want %q", filepath.Base(path), wantName)
	}

	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if a.Frontmatter.Title != "My First Post" {
		t.Errorf("title = %q, want %q", a.Frontmatter.Title, "My First Post")
	}
	if a.Frontmatter.Draft {
		t.Error("expected draft=false by default")
	}
	if !a.Frontmatter.Date.Equal(fixedNow) {
		t.Errorf("date = %v, want %v", a.Frontmatter.Date, fixedNow)
	}
}

// TestNew_Draft verifies --draft flag adds draft_ prefix and sets draft: true.
func TestNew_Draft(t *testing.T) {
	dir := t.TempDir()
	out, path, err := runNewCmd(t, dir, []string{"--draft", "-t", "Draft Post"})
	if err != nil {
		t.Fatalf("new failed: %v\noutput: %s", err, out)
	}

	wantName := "draft_20260306_Draft-Post.md"
	if filepath.Base(path) != wantName {
		t.Errorf("filename = %q, want %q", filepath.Base(path), wantName)
	}

	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if !a.Frontmatter.Draft {
		t.Error("expected draft=true with --draft flag")
	}
}

// TestNew_FileExists_Error verifies that creating a file that already exists returns an error.
func TestNew_FileExists_Error(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "20260306_Existing-Post.md")
	if err := os.WriteFile(existing, []byte("---\ntitle: Existing Post\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runNewCmd(t, dir, []string{"-t", "Existing Post"})
	if err == nil {
		t.Fatal("expected error for existing file, got nil")
	}
	// Error message should suggest renaming.
	if !strings.Contains(out, "rename") && !strings.Contains(err.Error(), "rename") {
		t.Errorf("expected rename hint in output or error, got output=%q err=%v", out, err)
	}
}

// TestNew_Body_Argument verifies -b "hello\nworld" converts literal \n to newline.
func TestNew_Body_Argument(t *testing.T) {
	dir := t.TempDir()
	out, path, err := runNewCmd(t, dir, []string{"-b", `hello\nworld`, "-t", "Body Post"})
	if err != nil {
		t.Fatalf("new failed: %v\noutput: %s", err, out)
	}

	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if !strings.Contains(a.Body, "hello\nworld") {
		t.Errorf("body = %q, want literal newline between hello and world", a.Body)
	}
}

// TestNew_Body_FlagTakesPrecedenceOverPipe verifies that -b value is used even when stdin is also piped.
func TestNew_Body_FlagTakesPrecedenceOverPipe(t *testing.T) {
	dir := t.TempDir()
	out, path, err := runNewCmd(t, dir, []string{"-b", `flag\nvalue`, "-t", "Flag Wins"}, newCmdOpts{
		stdin:       "pipe content",
		stdinIsPipe: true,
	})
	if err != nil {
		t.Fatalf("new failed: %v\noutput: %s", err, out)
	}

	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	// -b value must win over piped stdin; \n converted.
	if !strings.Contains(a.Body, "flag\nvalue") {
		t.Errorf("expected -b value in body, got: %q", a.Body)
	}
	if strings.Contains(a.Body, "pipe content") {
		t.Errorf("pipe content should be ignored when -b is given, got: %q", a.Body)
	}
}

// TestNew_Body_Pipe verifies piped stdin is used as-is (no \n conversion) even without -b.
func TestNew_Body_Pipe(t *testing.T) {
	dir := t.TempDir()
	out, path, err := runNewCmd(t, dir, []string{"-t", "Pipe Post"}, newCmdOpts{
		stdin:       `hello\nworld`,
		stdinIsPipe: true,
	})
	if err != nil {
		t.Fatalf("new failed: %v\noutput: %s", err, out)
	}

	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	// Pipe input must NOT have \n converted.
	if strings.Contains(a.Body, "hello\nworld") {
		t.Errorf("pipe body should not convert literal \\n, got: %q", a.Body)
	}
	if !strings.Contains(a.Body, `hello\nworld`) {
		t.Errorf("body = %q, want literal backslash-n preserved", a.Body)
	}
}

// TestNew_Push verifies --push flag POSTs to the API and writes back editUrl/url/date.
func TestNew_Push(t *testing.T) {
	postCalled := false
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		postCalled = true
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">
  <title>Push Post</title>
  <content type="text/x-markdown">body
</content>
  <published>2026-03-06T00:00:00Z</published>
  <updated>2026-03-06T00:00:00Z</updated>
  <link rel="edit" href="%s/user/example.hateblo.jp/atom/entry/42"/>
  <link rel="alternate" href="https://example.com/entry/42"/>
  <app:control><app:draft>no</app:draft></app:control>
</entry>`, srv.URL)
	})
	srv.Start()
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	dir := t.TempDir()
	out, path, err := runNewCmd(t, dir, []string{"--push", "-t", "Push Post"})
	if err != nil {
		t.Fatalf("new --push failed: %v\noutput: %s", err, out)
	}

	if !postCalled {
		t.Fatal("expected POST request")
	}
	if !strings.Contains(out, "Saved:") {
		t.Errorf("expected 'Saved:' in output for local file, got: %s", out)
	}
	if !strings.Contains(out, "Created:") {
		t.Errorf("expected 'Created:' in output for remote URL, got: %s", out)
	}

	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if a.Frontmatter.EditURL == "" {
		t.Error("expected EditURL to be written back")
	}
	if a.Frontmatter.URL == "" {
		t.Error("expected URL to be written back")
	}
	if a.Frontmatter.Date.IsZero() {
		t.Error("expected Date to be written back")
	}
}

// TestNew_Push_FileExists_Error verifies that push is not attempted when file already exists.
func TestNew_Push_FileExists_Error(t *testing.T) {
	postCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		postCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	dir := t.TempDir()
	existing := filepath.Join(dir, "20260306_Exists-Push.md")
	if err := os.WriteFile(existing, []byte("---\ntitle: Exists Push\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runNewCmd(t, dir, []string{"--push", "-t", "Exists Push"})
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if postCalled {
		t.Error("POST should not be called when file already exists")
	}
}

// TestNew_NoArgs verifies that omitting the required --title flag returns an error.
func TestNew_NoArgs(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runNewCmd(t, dir, []string{})
	if err == nil {
		t.Fatal("expected error when no title given")
	}
}

// TestNew_ReadFromStdin_WithoutPipe verifies that stdin is not consumed when not piped.
func TestNew_ReadFromStdin_WithoutPipe(t *testing.T) {
	dir := t.TempDir()
	out, path, err := runNewCmd(t, dir, []string{"-t", "No Body Post"})
	if err != nil {
		t.Fatalf("new failed: %v\noutput: %s", err, out)
	}
	a, err := article.Read(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if a.Body != "" {
		t.Errorf("expected empty body without pipe, got: %q", a.Body)
	}
}

// TestNew_Push_WithBody verifies that --push with -b sends body content.
func TestNew_Push_WithBody(t *testing.T) {
	var receivedBody []byte
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">
  <title>Body Push</title>
  <content type="text/x-markdown">hello
world
</content>
  <published>2026-03-06T00:00:00Z</published>
  <updated>2026-03-06T00:00:00Z</updated>
  <link rel="edit" href="%s/user/example.hateblo.jp/atom/entry/43"/>
  <link rel="alternate" href="https://example.com/entry/43"/>
  <app:control><app:draft>no</app:draft></app:control>
</entry>`, srv.URL)
	})

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	dir := t.TempDir()
	out, _, err := runNewCmd(t, dir, []string{"--push", "-b", `hello\nworld`, "-t", "Body Push"})
	if err != nil {
		t.Fatalf("new --push -b failed: %v\noutput: %s", err, out)
	}

	// The newline may appear as a raw newline or as the XML entity &#xA; in the body.
	body := string(receivedBody)
	if !strings.Contains(body, "hello\nworld") && !strings.Contains(body, "hello&#xA;world") {
		t.Errorf("expected body with newline in POST, got: %s", body)
	}
}
