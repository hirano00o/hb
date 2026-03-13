package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

// newTestStatusCmd creates a cobra.Command suitable for runStatus tests.
func newTestStatusCmd(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "status"}
	cmd.SetContext(t.Context())
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	return cmd, &out, &errBuf
}

// writeStatusMD writes a .md file with frontmatter for status tests.
func writeStatusMD(t *testing.T, dir, name string, fm article.Frontmatter, body string) string {
	t.Helper()
	a := &article.Article{Frontmatter: fm, Body: body}
	path := filepath.Join(dir, name)
	if err := article.Write(path, a); err != nil {
		t.Fatalf("writeStatusMD: %v", err)
	}
	return path
}

// buildStatusFeedXML builds a minimal Atom feed with the given entries for status tests.
// Each entry has editURL, title, content, and date.
type statusEntry struct {
	id      string
	title   string
	content string
	draft   bool
}

func buildStatusFeedXML(base string, entries []statusEntry) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	sb.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">` + "\n")
	for _, e := range entries {
		draftStr := "no"
		if e.draft {
			draftStr = "yes"
		}
		sb.WriteString(`  <entry>` + "\n")
		sb.WriteString(`    <link rel="edit" href="` + base + `/user/blog/atom/entry/` + e.id + `"/>` + "\n")
		sb.WriteString(`    <title>` + e.title + `</title>` + "\n")
		sb.WriteString(`    <updated>2026-03-01T12:00:00Z</updated>` + "\n")
		sb.WriteString(`    <published>2026-03-01T12:00:00Z</published>` + "\n")
		sb.WriteString(`    <content type="text/x-markdown">` + e.content + `</content>` + "\n")
		sb.WriteString(`    <app:control><app:draft>` + draftStr + `</app:draft></app:control>` + "\n")
		sb.WriteString(`  </entry>` + "\n")
	}
	sb.WriteString(`</feed>`)
	return sb.String()
}

func TestRunStatus(t *testing.T) {
	t.Run("no files", func(t *testing.T) {
		dir := t.TempDir()
		cmd, out, _ := newTestStatusCmd(t)

		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c := hatena.NewClient("user", "blog", "key")
		c.SetBaseURL(srv.URL)

		if err := runStatus(cmd, c, dir, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.String() != "No articles found.\n" {
			t.Errorf("expected 'No articles found.\\n', got %q", out.String())
		}
	})

	t.Run("all untracked (no editUrl)", func(t *testing.T) {
		dir := t.TempDir()
		writeStatusMD(t, dir, "draft.md", article.Frontmatter{
			Title: "Draft Post",
			Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			Draft: true,
		}, "body\n")

		mux := http.NewServeMux()
		mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(buildStatusFeedXML("http://"+r.Host, nil)))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c := hatena.NewClient("user", "blog", "key")
		c.SetBaseURL(srv.URL)

		cmd, out, _ := newTestStatusCmd(t)
		if err := runStatus(cmd, c, dir, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Untracked (1):") {
			t.Errorf("expected Untracked (1), got %q", out.String())
		}
		if !strings.Contains(out.String(), "draft.md") {
			t.Errorf("expected draft.md in output, got %q", out.String())
		}
	})

	t.Run("modified file detected", func(t *testing.T) {
		dir := t.TempDir()
		var srvURL string

		mux := http.NewServeMux()
		mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
			base := "http://" + r.Host
			entries := []statusEntry{
				{id: "1", title: "My Post", content: "remote body\n"},
			}
			w.Write([]byte(buildStatusFeedXML(base, entries)))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		srvURL = srv.URL

		editURL := srvURL + "/user/blog/atom/entry/1"
		writeStatusMD(t, dir, "post.md", article.Frontmatter{
			Title:   "My Post",
			Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			EditURL: editURL,
		}, "local body\n")

		c := hatena.NewClient("user", "blog", "key")
		c.SetBaseURL(srvURL)

		cmd, out, _ := newTestStatusCmd(t)
		if err := runStatus(cmd, c, dir, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Modified (1):") {
			t.Errorf("expected Modified (1), got %q", out.String())
		}
		if !strings.Contains(out.String(), "post.md") {
			t.Errorf("expected post.md in modified, got %q", out.String())
		}
		if !strings.Contains(out.String(), "Up to date (0):") {
			t.Errorf("expected Up to date (0), got %q", out.String())
		}
	})

	t.Run("up to date file detected", func(t *testing.T) {
		dir := t.TempDir()
		var srvURL string

		mux := http.NewServeMux()
		mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
			base := "http://" + r.Host
			entries := []statusEntry{
				{id: "2", title: "Synced Post", content: "same body\n"},
			}
			w.Write([]byte(buildStatusFeedXML(base, entries)))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		srvURL = srv.URL

		editURL := srvURL + "/user/blog/atom/entry/2"
		writeStatusMD(t, dir, "synced.md", article.Frontmatter{
			Title:   "Synced Post",
			Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			EditURL: editURL,
		}, "same body\n")

		c := hatena.NewClient("user", "blog", "key")
		c.SetBaseURL(srvURL)

		cmd, out, _ := newTestStatusCmd(t)
		if err := runStatus(cmd, c, dir, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Up to date (1):") {
			t.Errorf("expected Up to date (1), got %q", out.String())
		}
		if !strings.Contains(out.String(), "synced.md") {
			t.Errorf("expected synced.md in up to date, got %q", out.String())
		}
		if !strings.Contains(out.String(), "Modified (0):") {
			t.Errorf("expected Modified (0), got %q", out.String())
		}
	})

	t.Run("mixed: modified, untracked, up to date", func(t *testing.T) {
		dir := t.TempDir()
		var srvURL string

		mux := http.NewServeMux()
		mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
			base := "http://" + r.Host
			entries := []statusEntry{
				{id: "1", title: "Modified Post", content: "remote body\n"},
				{id: "2", title: "Synced Post", content: "same body\n"},
			}
			w.Write([]byte(buildStatusFeedXML(base, entries)))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		srvURL = srv.URL

		writeStatusMD(t, dir, "modified.md", article.Frontmatter{
			Title:   "Modified Post",
			Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			EditURL: srvURL + "/user/blog/atom/entry/1",
		}, "local body\n")
		writeStatusMD(t, dir, "synced.md", article.Frontmatter{
			Title:   "Synced Post",
			Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			EditURL: srvURL + "/user/blog/atom/entry/2",
		}, "same body\n")
		writeStatusMD(t, dir, "new.md", article.Frontmatter{
			Title: "New Draft",
			Date:  time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC),
			Draft: true,
		}, "draft body\n")

		c := hatena.NewClient("user", "blog", "key")
		c.SetBaseURL(srvURL)

		cmd, out, _ := newTestStatusCmd(t)
		if err := runStatus(cmd, c, dir, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		output := out.String()
		if !strings.Contains(output, "Modified (1):") {
			t.Errorf("expected Modified (1), got %q", output)
		}
		if !strings.Contains(output, "Untracked (1):") {
			t.Errorf("expected Untracked (1), got %q", output)
		}
		if !strings.Contains(output, "Up to date (1):") {
			t.Errorf("expected Up to date (1), got %q", output)
		}
	})

	t.Run("editUrl not found in remote treated as untracked", func(t *testing.T) {
		dir := t.TempDir()

		mux := http.NewServeMux()
		mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
			// Remote feed is empty — editUrl in local file has no match.
			w.Write([]byte(buildStatusFeedXML("http://"+r.Host, nil)))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		writeStatusMD(t, dir, "ghost.md", article.Frontmatter{
			Title:   "Ghost Post",
			Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			EditURL: srv.URL + "/user/blog/atom/entry/999",
		}, "body\n")

		c := hatena.NewClient("user", "blog", "key")
		c.SetBaseURL(srv.URL)

		cmd, out, _ := newTestStatusCmd(t)
		if err := runStatus(cmd, c, dir, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Untracked (1):") {
			t.Errorf("expected Untracked (1) for missing remote entry, got %q", out.String())
		}
	})

	t.Run("no frontmatter warning is suppressed by default", func(t *testing.T) {
		dir := t.TempDir()
		writeMD(t, dir, "bare.md", "just a body without frontmatter\n")
		writeStatusMD(t, dir, "valid.md", article.Frontmatter{
			Title: "Valid Post",
			Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			Draft: true,
		}, "body\n")

		mux := http.NewServeMux()
		mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(buildStatusFeedXML("http://"+r.Host, nil)))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		c := hatena.NewClient("user", "blog", "key")
		c.SetBaseURL(srv.URL)

		cmd, _, errBuf := newTestStatusCmd(t)
		if err := runStatus(cmd, c, dir, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(errBuf.String(), "warning: skipping") {
			t.Errorf("expected no per-file warning without verbose, got %q", errBuf.String())
		}
	})

	t.Run("no frontmatter file is skipped with warning", func(t *testing.T) {
		dir := t.TempDir()
		// File with no frontmatter delimiters — article.Read succeeds but Title and Date are zero.
		writeMD(t, dir, "bare.md", "just a body without frontmatter\n")
		writeStatusMD(t, dir, "valid.md", article.Frontmatter{
			Title: "Valid Post",
			Date:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			Draft: true,
		}, "body\n")

		mux := http.NewServeMux()
		mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(buildStatusFeedXML("http://"+r.Host, nil)))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		c := hatena.NewClient("user", "blog", "key")
		c.SetBaseURL(srv.URL)

		cmd, out, errBuf := newTestStatusCmd(t)
		if err := runStatus(cmd, c, dir, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(errBuf.String(), "warning:") {
			t.Errorf("expected warning for bare.md in stderr, got %q", errBuf.String())
		}
		// valid.md has no editUrl → Untracked
		if !strings.Contains(out.String(), "Untracked (1):") {
			t.Errorf("expected Untracked (1) for valid.md, got %q", out.String())
		}
	})
}
