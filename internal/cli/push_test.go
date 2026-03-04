package cli

import (
	"bytes"
	"encoding/xml"
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

// capturedEntry holds the XML body sent by the push command for inspection.
type capturedEntry struct {
	XMLName xml.Name `xml:"entry"`
	Control struct {
		Draft string `xml:"draft"`
	} `xml:"http://www.w3.org/2007/app control"`
}

func setupPushTest(t *testing.T, editURL string, fm article.Frontmatter, body string) (path string, cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	a := &article.Article{Frontmatter: fm, Body: body}
	p := filepath.Join(dir, "test.md")
	if err := article.Write(p, a); err != nil {
		t.Fatal(err)
	}
	return p, func() {}
}

// stubClient replaces newClientFromConfig with a function that returns c, restoring the original on cleanup.
func stubClient(t *testing.T, c *hatena.Client) {
	t.Helper()
	orig := newClientFromConfig
	t.Cleanup(func() { newClientFromConfig = orig })
	newClientFromConfig = func() (*hatena.Client, error) { return c, nil }
}

// TestPush_NewEntry_Create verifies that a file with no editUrl triggers a POST and
// updates the local file with the assigned editUrl and url.
func TestPush_NewEntry_Create(t *testing.T) {
	postCalled := false
	mux := http.NewServeMux()
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
  <title>New Entry</title>
  <content type="text/x-markdown">new body
</content>
  <published>2026-03-01T12:00:00Z</published>
  <updated>2026-03-01T12:00:00Z</updated>
  <link rel="edit" href="%s/user/example.hateblo.jp/atom/entry/99"/>
  <link rel="alternate" href="https://example.com/entry/99"/>
  <app:control><app:draft>no</app:draft></app:control>
</entry>`, r.Host)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	fm := article.Frontmatter{
		Title: "New Entry",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: false,
		// EditURL is empty → triggers POST
	}
	path, _ := setupPushTest(t, "", fm, "new body\n")

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("push failed: %v\noutput: %s", err, out.String())
	}

	if !postCalled {
		t.Fatal("expected POST request for new entry")
	}
	if !strings.Contains(out.String(), "Created:") {
		t.Errorf("expected 'Created:' in output, got: %s", out.String())
	}

	// Verify the file was updated with editUrl and url.
	updated, err := article.Read(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if updated.Frontmatter.EditURL == "" {
		t.Error("expected EditURL to be set after create")
	}
	if updated.Frontmatter.URL == "" {
		t.Error("expected URL to be set after create")
	}
}

// TestPush_Draft_FlagOverridesFrontmatter verifies that --draft overrides frontmatter draft=false
// and that the request body contains app:draft yes.
func TestPush_Draft_FlagOverridesFrontmatter(t *testing.T) {
	var receivedBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// return remote entry with draft=false
			writeEntryXMLFull(w, "Title", "body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/1", r.Host),
				"https://example.com/entry/1")
		case http.MethodPut:
			receivedBody, _ = io.ReadAll(r.Body)
			// echo back
			w.Write(receivedBody)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/1"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false, // frontmatter says not draft
		EditURL: editURL,
		URL:     "https://example.com/entry/1",
	}
	path, _ := setupPushTest(t, editURL, fm, "body\n")

	// Override the client base URL by pointing to our test server.
	// We rely on env-var config so no config file is needed.
	// The mock returns the same entry for GET, so push will detect no changes
	// unless --draft changes the draft field.

	// We need to make the client use our test server. Since newClientFromConfig
	// uses env vars for credentials, we patch the base URL by embedding the test
	// server URL directly into the editUrl frontmatter (already done above).
	// However the collection URL uses the real base URL. We only use GET+PUT on
	// the entry URL, which is already pointing at our test server.

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("y\n")) // confirm push
	cmd.SetArgs([]string{"--draft", "--yes", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("push failed: %v\noutput: %s", err, out.String())
	}

	if len(receivedBody) == 0 {
		t.Fatal("no PUT request was made")
	}

	var entry capturedEntry
	if err := xml.Unmarshal(receivedBody, &entry); err != nil {
		t.Fatalf("failed to parse request body: %v\nbody: %s", err, receivedBody)
	}
	if entry.Control.Draft != "yes" {
		t.Errorf("expected app:draft=yes in PUT body, got %q", entry.Control.Draft)
	}
}

// TestPush_Draft_Conflict_Aborted verifies that when frontmatter and --draft differ
// and the user declines, push is aborted.
func TestPush_Draft_Conflict_Aborted(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/2", func(w http.ResponseWriter, r *http.Request) {
		writeEntryXMLFull(w, "Title", "body\n", false,
			fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/2", r.Host),
			"https://example.com/entry/2")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/2"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/2",
	}
	path, _ := setupPushTest(t, editURL, fm, "body\n")

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("N\n")) // decline draft conflict prompt
	cmd.SetArgs([]string{"--draft", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected Aborted in output, got: %s", out.String())
	}
}

// TestPush_ConfirmPrompt_Confirm verifies that answering 'y' to the confirmation prompt
// causes the push to proceed and issue a PUT request.
func TestPush_ConfirmPrompt_Confirm(t *testing.T) {
	putCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/3", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeEntryXMLFull(w, "Title", "remote body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/3", r.Host),
				"https://example.com/entry/3") // different from local
		case http.MethodPut:
			putCalled = true
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/3"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/3",
	}
	path, _ := setupPushTest(t, editURL, fm, "local body\n")

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	if !putCalled {
		t.Error("expected PUT request after confirming with 'y'")
	}
}

// TestPush_ConfirmPrompt_Abort verifies that answering 'N' to the confirmation prompt
// aborts the push without issuing a PUT request.
func TestPush_ConfirmPrompt_Abort(t *testing.T) {
	putCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/4", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeEntryXMLFull(w, "Title", "remote body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/4", r.Host),
				"https://example.com/entry/4")
		case http.MethodPut:
			putCalled = true
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/4"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/4",
	}
	path, _ := setupPushTest(t, editURL, fm, "local body\n")

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("N\n"))
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	if putCalled {
		t.Error("expected no PUT request after aborting")
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected 'Aborted' in output, got: %s", out.String())
	}
}

// TestPush_YesFlag_SkipsPrompt verifies that --yes skips the confirmation prompt
// and the push proceeds without reading from stdin.
func TestPush_YesFlag_SkipsPrompt(t *testing.T) {
	putCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/5", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeEntryXMLFull(w, "Title", "remote body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/5", r.Host),
				"https://example.com/entry/5")
		case http.MethodPut:
			putCalled = true
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/5"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/5",
	}
	path, _ := setupPushTest(t, editURL, fm, "local body\n")

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// No stdin input: if the prompt is shown it would block or fail.
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"--yes", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	if !putCalled {
		t.Error("expected PUT request with --yes flag")
	}
}

// TestPush_DiffDirection_LocalAsFrom verifies that the unified diff shows
// local content as "from" (---) and remote content as "to" (+++).
func TestPush_DiffDirection_LocalAsFrom(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/6", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeEntryXMLFull(w, "Title", "remote body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/6", r.Host),
				"https://example.com/entry/6")
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/6"
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/6",
	}
	path, _ := setupPushTest(t, editURL, fm, "local body\n")

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("N\n")) // abort after seeing diff
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	diffOut := out.String()
	// "---" line must appear before "+++" line, and local content must be removed (-),
	// remote content must be added (+) in the diff output.
	minusIdx := strings.Index(diffOut, "-local body")
	plusIdx := strings.Index(diffOut, "+remote body")
	if minusIdx < 0 {
		t.Errorf("expected '-local body' in diff output, got:\n%s", diffOut)
	}
	if plusIdx < 0 {
		t.Errorf("expected '+remote body' in diff output, got:\n%s", diffOut)
	}
}

// writeEntryXMLFull writes a minimal Atom entry XML to w with explicit edit and alternate hrefs.
func writeEntryXMLFull(w http.ResponseWriter, title, content string, draft bool, editHref, alternateHref string) {
	draftStr := "no"
	if draft {
		draftStr = "yes"
	}
	w.Header().Set("Content-Type", "application/atom+xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">
  <title>%s</title>
  <content type="text/x-markdown">%s</content>
  <published>2026-03-01T12:00:00Z</published>
  <updated>2026-03-01T12:00:00Z</updated>
  <link rel="edit" href="%s"/>
  <link rel="alternate" href="%s"/>
  <app:control><app:draft>%s</app:draft></app:control>
</entry>`, title, content, editHref, alternateHref, draftStr)
}

// TestPush_LocalImage_Uploaded verifies that a local image in the article body is
// uploaded to Fotolife and the body sent to the blog API contains the hatena:syntax value.
func TestPush_LocalImage_Uploaded(t *testing.T) {
	// Set up Fotolife mock server
	fotolifeMux := http.NewServeMux()
	fotolifeMux.HandleFunc("/atom/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://purl.org/atom/ns#" xmlns:hatena="http://www.hatena.ne.jp/info/xmlns#">
  <title>photo.jpg</title>
  <hatena:syntax>[f:id:user:20260303120000j:image]</hatena:syntax>
</entry>`))
	})
	fotolifeSrv := httptest.NewServer(fotolifeMux)
	t.Cleanup(fotolifeSrv.Close)

	// Set up Blog API mock server
	var receivedBody []byte
	blogMux := http.NewServeMux()
	blogMux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">
  <title>Image Article</title>
  <content type="text/x-markdown">[f:id:user:20260303120000j:image]
</content>
  <published>2026-03-01T12:00:00Z</published>
  <updated>2026-03-01T12:00:00Z</updated>
  <link rel="edit" href="%s/user/example.hateblo.jp/atom/entry/10"/>
  <link rel="alternate" href="https://example.com/entry/10"/>
  <app:control><app:draft>no</app:draft></app:control>
</entry>`, r.Host)
	})
	blogSrv := httptest.NewServer(blogMux)
	t.Cleanup(blogSrv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	fm := article.Frontmatter{
		Title: "Image Article",
		Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft: false,
	}
	path, _ := setupPushTest(t, "", fm, "![alt](photo.jpg)\n")
	// Place the image in the same directory as the article file.
	imgPath := filepath.Join(filepath.Dir(path), "photo.jpg")
	if err := os.WriteFile(imgPath, []byte{0xFF, 0xD8, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(blogSrv.URL)
	c.SetFotolifeURL(fotolifeSrv.URL + "/atom/post")
	stubClient(t, c)

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("push failed: %v\noutput: %s", err, out.String())
	}

	if !strings.Contains(string(receivedBody), "[f:id:user:20260303120000j:image]") {
		t.Errorf("expected hatena:syntax in POST body, got: %s", receivedBody)
	}

	// Verify the local file still contains the original image reference (not rewritten)
	updated, err := article.Read(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if !strings.Contains(updated.Body, "photo.jpg") {
		t.Errorf("expected original body preserved in local file, got: %s", updated.Body)
	}
}

// TestPush_LocalImage_RePush_NoChanges verifies that pushing an article with a local image
// a second time (when the remote already has the hatena:syntax) reports "No changes".
// This guards against the bug where local.Body (with local paths) was compared to
// remoteArticle.Body (with hatena:syntax), which always produces a false diff.
func TestPush_LocalImage_RePush_NoChanges(t *testing.T) {
	const syntax = "[f:id:user:20260303120000j:image]"

	fotolifeMux := http.NewServeMux()
	fotolifeMux.HandleFunc("/atom/post", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://purl.org/atom/ns#" xmlns:hatena="http://www.hatena.ne.jp/info/xmlns#">
  <title>photo.jpg</title>
  <hatena:syntax>%s</hatena:syntax>
</entry>`, syntax)
	})
	fotolifeSrv := httptest.NewServer(fotolifeMux)
	t.Cleanup(fotolifeSrv.Close)

	blogMux := http.NewServeMux()
	blogMux.HandleFunc("/user/example.hateblo.jp/atom/entry/20", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			editHref := fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/20", r.Host)
			// Remote body already contains hatena:syntax from the first push.
			writeEntryXMLFull(w, "Image Article", syntax+"\n", false, editHref, "https://example.com/entry/20")
		} else {
			t.Errorf("unexpected %s request: expected no re-upload", r.Method)
		}
	})
	blogSrv := httptest.NewServer(blogMux)
	t.Cleanup(blogSrv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := blogSrv.URL + "/user/example.hateblo.jp/atom/entry/20"
	fm := article.Frontmatter{
		Title:   "Image Article",
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/20",
	}
	// Local file still contains the local image path (not replaced).
	path, _ := setupPushTest(t, editURL, fm, "![alt](photo.jpg)\n")
	imgPath := filepath.Join(filepath.Dir(path), "photo.jpg")
	if err := os.WriteFile(imgPath, []byte{0xFF, 0xD8, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(blogSrv.URL)
	c.SetFotolifeURL(fotolifeSrv.URL + "/atom/post")
	stubClient(t, c)

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("push failed: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "No changes") {
		t.Errorf("expected 'No changes' output on re-push, got: %s", out.String())
	}
}

// TestPush_HasChanges verifies that hasChanges detects differences in each field.
func TestPush_HasChanges(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	otherScheduledAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	date1 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)

	base := article.Article{
		Frontmatter: article.Frontmatter{
			Title:         "Title",
			Draft:         false,
			Category:      []string{"cat1"},
			CustomURLPath: "custom",
			Date:          date1,
		},
		Body: "body",
	}

	tests := []struct {
		name   string
		local  article.Article
		remote article.Article
		want   bool
	}{
		{
			name:   "no changes",
			local:  base,
			remote: base,
			want:   false,
		},
		{
			name:  "body differs",
			local: base,
			remote: func() article.Article {
				a := base
				a.Body = "other"
				return a
			}(),
			want: true,
		},
		{
			name:  "title differs",
			local: base,
			remote: func() article.Article {
				a := base
				a.Frontmatter.Title = "Other"
				return a
			}(),
			want: true,
		},
		{
			name:  "draft differs",
			local: base,
			remote: func() article.Article {
				a := base
				a.Frontmatter.Draft = true
				return a
			}(),
			want: true,
		},
		{
			name:  "category differs",
			local: base,
			remote: func() article.Article {
				a := base
				a.Frontmatter.Category = []string{"other"}
				return a
			}(),
			want: true,
		},
		{
			name:  "customUrlPath differs",
			local: base,
			remote: func() article.Article {
				a := base
				a.Frontmatter.CustomURLPath = "other"
				return a
			}(),
			want: true,
		},
		{
			name:  "date differs",
			local: base,
			remote: func() article.Article {
				a := base
				a.Frontmatter.Date = date2
				return a
			}(),
			want: true,
		},
		{
			name: "scheduledAt: local set remote nil",
			local: func() article.Article {
				a := base
				a.Frontmatter.ScheduledAt = &scheduledAt
				return a
			}(),
			remote: base,
			want:   true,
		},
		{
			name:  "scheduledAt: both nil",
			local: base,
			remote: base,
			want:  false,
		},
		{
			name: "scheduledAt: same value",
			local: func() article.Article {
				a := base
				a.Frontmatter.ScheduledAt = &scheduledAt
				return a
			}(),
			remote: func() article.Article {
				a := base
				a.Frontmatter.ScheduledAt = &scheduledAt
				return a
			}(),
			want: false,
		},
		{
			name: "scheduledAt: different value",
			local: func() article.Article {
				a := base
				a.Frontmatter.ScheduledAt = &scheduledAt
				return a
			}(),
			remote: func() article.Article {
				a := base
				a.Frontmatter.ScheduledAt = &otherScheduledAt
				return a
			}(),
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasChanges(&tc.local, &tc.remote)
			if got != tc.want {
				t.Errorf("hasChanges=%v, want %v", got, tc.want)
			}
		})
	}
}

// TestPush_ScheduledAt_NewEntry verifies that pushing a file with scheduledAt
// sends draft=yes and hatenablog:scheduled=yes in the POST body.
func TestPush_ScheduledAt_NewEntry(t *testing.T) {
	var receivedBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app"
       xmlns:hatenablog="http://www.hatena.ne.jp/info/xmlns#hatenablog">
  <title>Scheduled Entry</title>
  <content type="text/x-markdown">body
</content>
  <published>2026-04-01T12:00:00Z</published>
  <updated>2026-04-01T12:00:00Z</updated>
  <link rel="edit" href="%s/user/example.hateblo.jp/atom/entry/50"/>
  <link rel="alternate" href=""/>
  <app:control>
    <app:draft>yes</app:draft>
    <hatenablog:scheduled>yes</hatenablog:scheduled>
  </app:control>
</entry>`, r.Host)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	scheduledAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	fm := article.Frontmatter{
		Title:       "Scheduled Entry",
		Date:        time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:       false, // intentionally false; ToEntry must force draft=true
		ScheduledAt: &scheduledAt,
	}
	path, _ := setupPushTest(t, "", fm, "body\n")

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("push failed: %v\noutput: %s", err, out.String())
	}

	if len(receivedBody) == 0 {
		t.Fatal("no POST request was made")
	}

	type scheduledEntry struct {
		XMLName xml.Name `xml:"entry"`
		Control struct {
			Draft     string `xml:"draft"`
			Scheduled string `xml:"http://www.hatena.ne.jp/info/xmlns#hatenablog scheduled"`
		} `xml:"http://www.w3.org/2007/app control"`
	}
	var entry scheduledEntry
	if err := xml.Unmarshal(receivedBody, &entry); err != nil {
		t.Fatalf("parse request body: %v\nbody: %s", err, receivedBody)
	}
	if entry.Control.Draft != "yes" {
		t.Errorf("expected draft=yes, got %q", entry.Control.Draft)
	}
	if entry.Control.Scheduled != "yes" {
		t.Errorf("expected scheduled=yes, got %q", entry.Control.Scheduled)
	}
}

// TestPush_ScheduledAt_UpdateEntry verifies that updating an existing entry with scheduledAt
// sends draft=yes and hatenablog:scheduled=yes in the PUT body.
func TestPush_ScheduledAt_UpdateEntry(t *testing.T) {
	var receivedBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/51", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Remote has no scheduledAt so hasChanges returns true.
			writeEntryXMLFull(w, "Scheduled Entry", "body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/51", r.Host),
				"https://example.com/entry/51")
		case http.MethodPut:
			receivedBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/atom+xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app"
       xmlns:hatenablog="http://www.hatena.ne.jp/info/xmlns#hatenablog">
  <title>Scheduled Entry</title>
  <content type="text/x-markdown">body
</content>
  <published>2026-04-01T12:00:00Z</published>
  <updated>2026-04-01T12:00:00Z</updated>
  <link rel="edit" href="http://%s/user/example.hateblo.jp/atom/entry/51"/>
  <link rel="alternate" href="https://example.com/entry/51"/>
  <app:control>
    <app:draft>yes</app:draft>
    <hatenablog:scheduled>yes</hatenablog:scheduled>
  </app:control>
</entry>`, r.Host)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	scheduledAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/51"
	fm := article.Frontmatter{
		Title:       "Scheduled Entry",
		Date:        time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:       false, // ToEntry must force draft=true due to scheduledAt
		EditURL:     editURL,
		URL:         "https://example.com/entry/51",
		ScheduledAt: &scheduledAt,
	}
	path, _ := setupPushTest(t, editURL, fm, "body\n")

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--yes", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("push failed: %v\noutput: %s", err, out.String())
	}

	if len(receivedBody) == 0 {
		t.Fatal("no PUT request was made")
	}

	type scheduledEntry struct {
		XMLName xml.Name `xml:"entry"`
		Control struct {
			Draft     string `xml:"draft"`
			Scheduled string `xml:"http://www.hatena.ne.jp/info/xmlns#hatenablog scheduled"`
		} `xml:"http://www.w3.org/2007/app control"`
	}
	var entry scheduledEntry
	if err := xml.Unmarshal(receivedBody, &entry); err != nil {
		t.Fatalf("parse PUT body: %v\nbody: %s", err, receivedBody)
	}
	if entry.Control.Draft != "yes" {
		t.Errorf("expected draft=yes in PUT body, got %q", entry.Control.Draft)
	}
	if entry.Control.Scheduled != "yes" {
		t.Errorf("expected scheduled=yes in PUT body, got %q", entry.Control.Scheduled)
	}
}

// TestPush_NewEntry_Create_UpdatesDate verifies that a POST response's published date
// is written back to the local frontmatter Date field.
func TestPush_NewEntry_Create_UpdatesDate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">
  <title>New Entry</title>
  <content type="text/x-markdown">body
</content>
  <published>2026-03-01T12:00:00Z</published>
  <updated>2026-03-01T12:00:00Z</updated>
  <link rel="edit" href="%s/user/example.hateblo.jp/atom/entry/100"/>
  <link rel="alternate" href="https://example.com/entry/100"/>
  <app:control><app:draft>no</app:draft></app:control>
</entry>`, r.Host)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	// Local Date is zero value; remote published is 2026-03-01T12:00:00Z.
	fm := article.Frontmatter{
		Title: "New Entry",
		Date:  time.Time{},
		Draft: false,
	}
	path, _ := setupPushTest(t, "", fm, "body\n")

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("push failed: %v\noutput: %s", err, out.String())
	}

	updated, err := article.Read(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	want := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if !updated.Frontmatter.Date.Equal(want) {
		t.Errorf("expected Date=%v, got %v", want, updated.Frontmatter.Date)
	}
}

// TestPush_Update_UpdatesDate verifies that a PUT response's published date
// is written back to the local frontmatter Date field.
func TestPush_Update_UpdatesDate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/200", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Remote body differs so hasChanges returns true.
			writeEntryXMLFull(w, "Title", "remote body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/200", r.Host),
				"https://example.com/entry/200")
		case http.MethodPut:
			// PUT response returns a new editUrl (entry/201) to simulate editUrl change.
			writeEntryXMLFull(w, "Title", "local body\n", false,
				fmt.Sprintf("http://%s/user/example.hateblo.jp/atom/entry/201", r.Host),
				"https://example.com/entry/200")
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	editURL := srv.URL + "/user/example.hateblo.jp/atom/entry/200"
	// Local Date differs from remote published to confirm write-back occurs.
	fm := article.Frontmatter{
		Title:   "Title",
		Date:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Draft:   false,
		EditURL: editURL,
		URL:     "https://example.com/entry/200",
	}
	path, _ := setupPushTest(t, editURL, fm, "local body\n")

	c := hatena.NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	stubClient(t, c)

	cmd := newPushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--yes", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("push failed: %v\noutput: %s", err, out.String())
	}

	updated, err := article.Read(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	want := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if !updated.Frontmatter.Date.Equal(want) {
		t.Errorf("expected Date=%v after PUT, got %v", want, updated.Frontmatter.Date)
	}
	// PUT response returned entry/201; verify the local file reflects the new editUrl.
	wantEditURL := fmt.Sprintf("%s/user/example.hateblo.jp/atom/entry/201", srv.URL)
	if updated.Frontmatter.EditURL != wantEditURL {
		t.Errorf("expected EditURL=%q after PUT, got %q", wantEditURL, updated.Frontmatter.EditURL)
	}
}


