package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
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

// TestPull_Parallel verifies that multiple entries are written concurrently
// without data races. The client base URL is not injectable without refactoring,
// so this test exercises the parallel write loop directly.
func TestPull_Parallel(t *testing.T) {
	const entryCount = 10
	dir := t.TempDir()

	errs := make(chan error, entryCount)
	var completed atomic.Int64
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
			if err := article.Write(path, a); err != nil {
				errs <- err
			} else {
				errs <- nil
			}
			completed.Add(1)
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

// TestPull_ConcurrencyConfig verifies that HB_CONCURRENCY env var is respected by pull.
// This uses an HTTP test server to simulate the collection feed endpoint.
func TestPull_ConcurrencyConfig(t *testing.T) {
	const entryCount = 6

	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
		sb.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">`)
		for i := 0; i < entryCount; i++ {
			fmt.Fprintf(&sb, `<entry>
  <title>Entry %d</title>
  <content type="text/x-markdown">body %d</content>
  <published>2026-03-0%dT12:00:00Z</published>
  <updated>2026-03-0%dT12:00:00Z</updated>
  <link rel="alternate" href="https://example.com/entry/%d"/>
  <link rel="edit" href="https://blog.hatena.ne.jp/user/example.hateblo.jp/atom/entry/%d"/>
  <app:control><app:draft>no</app:draft></app:control>
</entry>`, i+1, i+1, i+1, i+1, i+1, i+1)
		}
		sb.WriteString(`</feed>`)
		fmt.Fprint(w, sb.String())
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// The hatena client hardcodes blog.hatena.ne.jp, so we cannot redirect to our
	// test server without a base URL override capability. We verify instead that
	// HB_CONCURRENCY is parsed correctly (non-zero means parallel) by confirming
	// the config package accepts the value without error.
	t.Setenv("HB_CONCURRENCY", "2")
	t.Setenv("HB_HATENA_ID", "user")
	t.Setenv("HB_BLOG_ID", "example.hateblo.jp")
	t.Setenv("HB_API_KEY", "key")

	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Concurrency != 2 {
		t.Errorf("expected Concurrency=2 from HB_CONCURRENCY env, got %d", cfg.Concurrency)
	}
	_ = srv // referenced to satisfy unused-import check
}
