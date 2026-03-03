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
	"github.com/spf13/cobra"
)

func TestParseFilterDate(t *testing.T) {
	want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2026-01-15", false},
		{"2026/01/15", false},
		{"20260115", false},
		{"2026.01.15", true},
		{"invalid", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFilterDate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if !got.Equal(want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestFilterEntriesByDate(t *testing.T) {
	mkEntry := func(year int, month time.Month, day int) *hatena.Entry {
		return &hatena.Entry{
			Date:    time.Date(year, month, day, 12, 0, 0, 0, time.FixedZone("JST", 9*3600)),
			EditURL: "https://example.com/" + time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format("20060102"),
		}
	}

	entries := []*hatena.Entry{
		mkEntry(2025, 12, 31),
		mkEntry(2026, 1, 1),
		mkEntry(2026, 1, 15),
		mkEntry(2026, 3, 1),
		mkEntry(2026, 3, 2),
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		from, to time.Time
		wantLen  int
		wantURLs []string
	}{
		{
			name:     "no filter",
			wantLen:  5,
		},
		{
			name:     "from only",
			from:     from,
			wantLen:  4,
			wantURLs: []string{"20260101", "20260115", "20260301", "20260302"},
		},
		{
			name:     "to only",
			to:       to,
			wantLen:  4,
			wantURLs: []string{"20251231", "20260101", "20260115", "20260301"},
		},
		{
			name:     "from and to",
			from:     from,
			to:       to,
			wantLen:  3,
			wantURLs: []string{"20260101", "20260115", "20260301"},
		},
		{
			name:     "from equals to (single day)",
			from:     from,
			to:       from,
			wantLen:  1,
			wantURLs: []string{"20260101"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterEntriesByDate(entries, tt.from, tt.to)
			if len(got) != tt.wantLen {
				t.Errorf("len=%d, want %d", len(got), tt.wantLen)
			}
			for i, wantURL := range tt.wantURLs {
				if i >= len(got) {
					break
				}
				if !strings.Contains(got[i].EditURL, wantURL) {
					t.Errorf("entry[%d] EditURL=%q, want suffix %q", i, got[i].EditURL, wantURL)
				}
			}
		})
	}
}

// buildFeedXML generates a minimal Atom feed XML with the given entries and optional next-page URL.
// base is the scheme+host of the test server (e.g. "http://127.0.0.1:PORT"), derived from r.Host at request time.
func buildFeedXML(base string, entries []struct{ id, title string }, nextURL string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	sb.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">` + "\n")
	if nextURL != "" {
		sb.WriteString(`  <link rel="next" href="` + nextURL + `"/>` + "\n")
	}
	for _, e := range entries {
		sb.WriteString(`  <entry>` + "\n")
		sb.WriteString(`    <link rel="edit" href="` + base + `/user/blog/atom/entry/` + e.id + `"/>` + "\n")
		sb.WriteString(`    <title>` + e.title + `</title>` + "\n")
		sb.WriteString(`    <updated>2026-03-01T12:00:00+09:00</updated>` + "\n")
		sb.WriteString(`    <published>2026-03-01T12:00:00+09:00</published>` + "\n")
		sb.WriteString(`    <content type="text/x-markdown">body of ` + e.title + `</content>` + "\n")
		sb.WriteString(`    <app:control><app:draft>no</app:draft></app:control>` + "\n")
		sb.WriteString(`  </entry>` + "\n")
	}
	sb.WriteString(`</feed>`)
	return sb.String()
}

// newTestPullCmd creates a cobra.Command whose output is captured in the returned buffer.
func newTestPullCmd(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "pull"}
	cmd.SetContext(t.Context())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	return cmd, &buf
}

func TestRunPull_Integration(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func(t *testing.T) *http.ServeMux
		setupDir    func(t *testing.T, dir, srvURL string)
		checkDir    func(t *testing.T, dir string)
		force       bool
		concurrency int
		maxPages    int
		wantFiles   int
	}{
		{
			name: "basic: multiple entries parallel download",
			setupServer: func(t *testing.T) *http.ServeMux {
				mux := http.NewServeMux()
				mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
					base := "http://" + r.Host
					entries := []struct{ id, title string }{{"1", "Alpha"}, {"2", "Beta"}, {"3", "Gamma"}}
					w.Write([]byte(buildFeedXML(base, entries, "")))
				})
				return mux
			},
			force:       true,
			concurrency: 2,
			wantFiles:   3,
		},
		{
			name: "pagination: two pages of entries",
			setupServer: func(t *testing.T) *http.ServeMux {
				mux := http.NewServeMux()
				mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
					base := "http://" + r.Host
					if r.URL.RawQuery == "page=2" {
						w.Write([]byte(buildFeedXML(base, []struct{ id, title string }{{"2", "Page2Entry"}}, "")))
						return
					}
					w.Write([]byte(buildFeedXML(base, []struct{ id, title string }{{"1", "Page1Entry"}}, base+"/user/blog/atom/entry?page=2")))
				})
				return mux
			},
			force:       true,
			concurrency: 2,
			wantFiles:   2,
		},
		{
			name: "skip existing entries by editURL",
			setupServer: func(t *testing.T) *http.ServeMux {
				mux := http.NewServeMux()
				mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
					base := "http://" + r.Host
					entries := []struct{ id, title string }{{"1", "ExistingEntry"}, {"2", "NewEntry"}}
					w.Write([]byte(buildFeedXML(base, entries, "")))
				})
				return mux
			},
			setupDir: func(t *testing.T, dir, srvURL string) {
				// Write a .md file with the editURL of entry 1 in its frontmatter.
				editURL := srvURL + "/user/blog/atom/entry/1"
				content := "---\ntitle: ExistingEntry\ndate: 2026-03-01T12:00:00+09:00\ndraft: false\neditUrl: " + editURL + "\n---\nold body\n"
				if err := os.WriteFile(filepath.Join(dir, "existing.md"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			force:       false,
			concurrency: 2,
			wantFiles:   2, // existing.md + one new file for entry 2
		},
		{
			name: "force=true: skip entries whose editURL already exists locally",
			setupServer: func(t *testing.T) *http.ServeMux {
				mux := http.NewServeMux()
				mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
					base := "http://" + r.Host
					entries := []struct{ id, title string }{{"1", "KnownEntry"}, {"2", "NewEntry"}}
					w.Write([]byte(buildFeedXML(base, entries, "")))
				})
				return mux
			},
			setupDir: func(t *testing.T, dir, srvURL string) {
				editURL := srvURL + "/user/blog/atom/entry/1"
				content := "---\ntitle: KnownEntry\ndate: 2026-03-01T12:00:00+09:00\ndraft: false\neditUrl: " + editURL + "\n---\nold body\n"
				if err := os.WriteFile(filepath.Join(dir, "existing.md"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			force:       true,
			concurrency: 2,
			wantFiles:   2, // existing.md (entry1 skipped) + one new file for entry2
			checkDir: func(t *testing.T, dir string) {
				t.Helper()
				b, err := os.ReadFile(filepath.Join(dir, "existing.md"))
				if err != nil {
					t.Fatalf("existing.md should not be removed: %v", err)
				}
				if !strings.Contains(string(b), "old body") {
					t.Errorf("existing.md should not be overwritten, got: %s", b)
				}
			},
		},
		{
			name: "force=true: auto-rename on filename collision",
			setupServer: func(t *testing.T) *http.ServeMux {
				mux := http.NewServeMux()
				mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
					base := "http://" + r.Host
					w.Write([]byte(buildFeedXML(base, []struct{ id, title string }{{"1", "SameTitle"}}, "")))
				})
				return mux
			},
			setupDir: func(t *testing.T, dir, _ string) {
				// Pre-place a file with the same generated name.
				if err := os.WriteFile(filepath.Join(dir, "20260301_SameTitle.md"), []byte("old"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			force:       true,
			concurrency: 2,
			wantFiles:   2, // original + renamed _1
		},
		{
			name: "maxPages limits pagination",
			setupServer: func(t *testing.T) *http.ServeMux {
				mux := http.NewServeMux()
				mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
					base := "http://" + r.Host
					switch r.URL.RawQuery {
					case "page=2":
						w.Write([]byte(buildFeedXML(base, []struct{ id, title string }{{"2", "Page2Entry"}}, base+"/user/blog/atom/entry?page=3")))
					case "page=3":
						w.Write([]byte(buildFeedXML(base, []struct{ id, title string }{{"3", "Page3Entry"}}, "")))
					default:
						w.Write([]byte(buildFeedXML(base, []struct{ id, title string }{{"1", "Page1Entry"}}, base+"/user/blog/atom/entry?page=2")))
					}
				})
				return mux
			},
			force:       true,
			concurrency: 2,
			maxPages:    1,
			wantFiles:   1, // only page 1
		},
		{
			name: "concurrency=1 serialises downloads",
			setupServer: func(t *testing.T) *http.ServeMux {
				mux := http.NewServeMux()
				mux.HandleFunc("/user/blog/atom/entry", func(w http.ResponseWriter, r *http.Request) {
					base := "http://" + r.Host
					entries := []struct{ id, title string }{{"1", "Alpha"}, {"2", "Beta"}}
					w.Write([]byte(buildFeedXML(base, entries, "")))
				})
				return mux
			},
			force:       true,
			concurrency: 1, // serialise: only one goroutine at a time
			wantFiles:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			srv := httptest.NewServer(tt.setupServer(t))
			t.Cleanup(srv.Close)

			if tt.setupDir != nil {
				tt.setupDir(t, dir, srv.URL)
			}

			c := hatena.NewClient("user", "blog", "key")
			c.SetBaseURL(srv.URL)

			cmd, _ := newTestPullCmd(t)
			if err := runPull(cmd, c, dir, tt.force, time.Time{}, time.Time{}, tt.concurrency, tt.maxPages); err != nil {
				t.Fatalf("runPull error: %v", err)
			}

			files, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != tt.wantFiles {
				names := make([]string, 0, len(files))
				for _, f := range files {
					names = append(names, f.Name())
				}
				t.Errorf("got %d files %v, want %d", len(files), names, tt.wantFiles)
			}
			if tt.checkDir != nil {
				tt.checkDir(t, dir)
			}
		})
	}
}

// TestPull_Parallel verifies that article.Write can be called concurrently
// without data races. End-to-end integration tests for the pull parallel loop
// are covered by TestRunPull_Integration.
func TestPull_Parallel(t *testing.T) {
	const entryCount = 10
	dir := t.TempDir()

	errs := make(chan error, entryCount)
	for i := 0; i < entryCount; i++ {
		i := i
		go func() {
			e := &hatena.Entry{
				Title:   fmt.Sprintf("Entry %d", i),
				Content: fmt.Sprintf("body %d\n", i),
				Date:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
				EditURL: fmt.Sprintf("https://blog.hatena.ne.jp/user/example.hateblo.jp/atom/entry/%d", i),
			}
			a := article.FromEntry(e)
			path := filepath.Join(dir, fmt.Sprintf("20260301_Entry_%d.md", i))
			errs <- article.Write(path, a)
		}()
	}
	for i := 0; i < entryCount; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent write failed: %v", err)
		}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != entryCount {
		t.Errorf("expected %d files, got %d", entryCount, len(files))
	}
}
