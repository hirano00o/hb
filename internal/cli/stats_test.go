package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newTestStatsCmd(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "stats"}
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	return cmd, &out, &errBuf
}

func writeStatsMD(t *testing.T, dir, name string, fm article.Frontmatter) string {
	t.Helper()
	a := &article.Article{Frontmatter: fm, Body: "body\n"}
	path := dir + "/" + name
	if err := article.Write(path, a); err != nil {
		t.Fatalf("writeStatsMD: %v", err)
	}
	return path
}

func TestRunStats_Totals(t *testing.T) {
	dir := t.TempDir()

	writeStatsMD(t, dir, "pub1.md", article.Frontmatter{
		Title: "Pub 1",
		Date:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Draft: false,
	})
	writeStatsMD(t, dir, "pub2.md", article.Frontmatter{
		Title: "Pub 2",
		Date:  time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		Draft: false,
	})
	writeStatsMD(t, dir, "draft1.md", article.Frontmatter{
		Title: "Draft 1",
		Date:  time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		Draft: true,
	})
	sched := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	writeStatsMD(t, dir, "sched1.md", article.Frontmatter{
		Title:       "Scheduled 1",
		Date:        time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		Draft:       true,
		ScheduledAt: &sched,
	})

	cmd, out, _ := newTestStatsCmd(t)
	if err := runStats(cmd, dir, 6, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := out.String()
	if !strings.Contains(s, "Total:     4 articles") {
		t.Errorf("expected total 4, got %q", s)
	}
	if !strings.Contains(s, "Published: 2") {
		t.Errorf("expected Published: 2, got %q", s)
	}
	if !strings.Contains(s, "Drafts:    1") {
		t.Errorf("expected Drafts:    1, got %q", s)
	}
	if !strings.Contains(s, "Scheduled: 1") {
		t.Errorf("expected Scheduled: 1, got %q", s)
	}
}

func TestRunStats_CategoryCount(t *testing.T) {
	dir := t.TempDir()

	writeStatsMD(t, dir, "go1.md", article.Frontmatter{
		Title:    "Go 1",
		Date:     time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Category: []string{"Go"},
	})
	writeStatsMD(t, dir, "go2.md", article.Frontmatter{
		Title:    "Go 2",
		Date:     time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		Category: []string{"Go"},
	})
	writeStatsMD(t, dir, "blog1.md", article.Frontmatter{
		Title:    "Blog 1",
		Date:     time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		Category: []string{"Blog"},
	})
	writeStatsMD(t, dir, "none1.md", article.Frontmatter{
		Title: "No Category",
		Date:  time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC),
	})

	cmd, out, _ := newTestStatsCmd(t)
	if err := runStats(cmd, dir, 6, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := out.String()
	if !strings.Contains(s, "By category:") {
		t.Errorf("expected 'By category:' header, got %q", s)
	}
	if !strings.Contains(s, "Go:") {
		t.Errorf("expected Go category, got %q", s)
	}
	if !strings.Contains(s, "Blog:") {
		t.Errorf("expected Blog category, got %q", s)
	}
	if !strings.Contains(s, "(none):") {
		t.Errorf("expected (none) category, got %q", s)
	}

	// Go (2) should appear before Blog (1) and (none) (1)
	goIdx := strings.Index(s, "Go:")
	blogIdx := strings.Index(s, "Blog:")
	if goIdx < 0 || blogIdx < 0 {
		t.Fatalf("missing category in output: %q", s)
	}
	if goIdx > blogIdx {
		t.Errorf("expected Go before Blog (descending count), got %q", s)
	}
}

func TestRunStats_MonthCount(t *testing.T) {
	dir := t.TempDir()

	dates := []time.Time{
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	for i, d := range dates {
		writeStatsMD(t, dir, "art"+string(rune('0'+i))+".md", article.Frontmatter{
			Title: "Article",
			Date:  d,
		})
	}

	cmd, out, _ := newTestStatsCmd(t)
	if err := runStats(cmd, dir, 6, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := out.String()
	if !strings.Contains(s, "By month") {
		t.Errorf("expected 'By month' header, got %q", s)
	}
	if !strings.Contains(s, "2026-03:") {
		t.Errorf("expected 2026-03 in output, got %q", s)
	}
	if !strings.Contains(s, "2026-02:") {
		t.Errorf("expected 2026-02 in output, got %q", s)
	}
	// 2025-06 is older than 6 months from 2026-03, should not appear
	if strings.Contains(s, "2025-06:") {
		t.Errorf("expected 2025-06 to be excluded with --months 6, got %q", s)
	}
}

func TestRunStats_MonthsZeroShowsAll(t *testing.T) {
	dir := t.TempDir()

	dates := []time.Time{
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for i, d := range dates {
		writeStatsMD(t, dir, "a"+string(rune('0'+i))+".md", article.Frontmatter{
			Title: "Article",
			Date:  d,
		})
	}

	cmd, out, _ := newTestStatsCmd(t)
	if err := runStats(cmd, dir, 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := out.String()
	if !strings.Contains(s, "2026-03:") {
		t.Errorf("expected 2026-03 in all output, got %q", s)
	}
	if !strings.Contains(s, "2024-01:") {
		t.Errorf("expected 2024-01 in all output, got %q", s)
	}
	if !strings.Contains(s, "By month (recent all):") {
		t.Errorf("expected 'By month (recent all):' header, got %q", s)
	}
}
