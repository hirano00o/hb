package cli

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

// TestPush_Draft_FlagOverridesFrontmatter verifies that --draft overrides frontmatter draft=false
// and that the request body contains app:draft yes.
func TestPush_Draft_FlagOverridesFrontmatter(t *testing.T) {
	var receivedBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// return remote entry with draft=false
			writeEntryXML(w, "Title", "body\n", false)
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
		writeEntryXML(w, "Title", "body\n", false)
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

// writeEntryXML writes a minimal Atom entry XML to w.
func writeEntryXML(w http.ResponseWriter, title, content string, draft bool) {
	draftStr := "no"
	if draft {
		draftStr = "yes"
	}
	e := &hatena.Entry{
		Title:   title,
		Content: content,
		Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Draft:   draft,
	}
	_ = draftStr
	_ = e

	// Write a minimal valid Atom entry XML.
	w.Header().Set("Content-Type", "application/atom+xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">
  <title>%s</title>
  <content type="text/x-markdown">%s</content>
  <published>2026-03-01T12:00:00Z</published>
  <updated>2026-03-01T12:00:00Z</updated>
  <link rel="edit" href="https://blog.hatena.ne.jp/user/example.hateblo.jp/atom/entry/1"/>
  <link rel="alternate" href="https://example.com/entry/1"/>
  <app:control><app:draft>%s</app:draft></app:control>
</entry>`, title, content, draftStr)
}

