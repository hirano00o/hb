package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hirano00o/hb/article"
)

func TestRunSchedule(t *testing.T) {
	t.Run("sets scheduledAt with RFC3339 format", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "article.md")
		writeMD(t, dir, "article.md", "---\ntitle: Test\ndate: 2026-01-01T00:00:00Z\ndraft: true\n---\nbody\n")

		if err := runSchedule(path, "2026-04-01T12:00:00Z"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a, err := article.Read(path)
		if err != nil {
			t.Fatalf("read after schedule: %v", err)
		}
		if a.Frontmatter.ScheduledAt == nil {
			t.Fatal("expected scheduledAt to be set, got nil")
		}
		got := a.Frontmatter.ScheduledAt.UTC().Format("2006-01-02T15:04:05Z")
		if got != "2026-04-01T12:00:00Z" {
			t.Errorf("expected scheduledAt=2026-04-01T12:00:00Z, got %s", got)
		}
	})

	t.Run("sets scheduledAt with space-separated format", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "article.md")
		writeMD(t, dir, "article.md", "---\ntitle: Test\ndate: 2026-01-01T00:00:00Z\ndraft: true\n---\nbody\n")

		if err := runSchedule(path, "2026-04-01 12:00:00"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a, err := article.Read(path)
		if err != nil {
			t.Fatalf("read after schedule: %v", err)
		}
		if a.Frontmatter.ScheduledAt == nil {
			t.Fatal("expected scheduledAt to be set, got nil")
		}
		got := a.Frontmatter.ScheduledAt.UTC().Format("2006-01-02T15:04:05Z")
		if got != "2026-04-01T12:00:00Z" {
			t.Errorf("expected scheduledAt=2026-04-01T12:00:00Z, got %s", got)
		}
	})

	t.Run("returns error for invalid datetime format", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "article.md")
		writeMD(t, dir, "article.md", "---\ntitle: Test\ndate: 2026-01-01T00:00:00Z\ndraft: true\n---\nbody\n")

		err := runSchedule(path, "not-a-date")
		if err == nil {
			t.Fatal("expected error for invalid datetime, got nil")
		}
		if !strings.Contains(err.Error(), "invalid datetime") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("works on article without editUrl", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "article.md")
		// no editUrl field
		writeMD(t, dir, "article.md", "---\ntitle: No EditURL\ndate: 2026-02-01T00:00:00Z\ndraft: true\n---\ncontent\n")

		if err := runSchedule(path, "2026-05-01T09:00:00Z"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a, err := article.Read(path)
		if err != nil {
			t.Fatalf("read after schedule: %v", err)
		}
		if a.Frontmatter.ScheduledAt == nil {
			t.Fatal("expected scheduledAt to be set")
		}
		if a.Frontmatter.EditURL != "" {
			t.Errorf("expected editUrl to remain empty, got %q", a.Frontmatter.EditURL)
		}
	})
}

func TestRunUnschedule(t *testing.T) {
	t.Run("clears scheduledAt", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "article.md")
		writeMD(t, dir, "article.md", "---\ntitle: Test\ndate: 2026-01-01T00:00:00Z\ndraft: true\nscheduledAt: 2026-04-01T12:00:00Z\n---\nbody\n")

		if err := runUnschedule(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a, err := article.Read(path)
		if err != nil {
			t.Fatalf("read after unschedule: %v", err)
		}
		if a.Frontmatter.ScheduledAt != nil {
			t.Errorf("expected scheduledAt to be nil, got %v", a.Frontmatter.ScheduledAt)
		}
	})

	t.Run("works when scheduledAt is already nil", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "article.md")
		writeMD(t, dir, "article.md", "---\ntitle: Test\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\nbody\n")

		if err := runUnschedule(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a, err := article.Read(path)
		if err != nil {
			t.Fatalf("read after unschedule: %v", err)
		}
		if a.Frontmatter.ScheduledAt != nil {
			t.Errorf("expected scheduledAt to remain nil, got %v", a.Frontmatter.ScheduledAt)
		}
	})
}

func TestScheduleCmd_Integration(t *testing.T) {
	t.Run("schedule command updates file via CLI", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "article.md")
		if err := os.WriteFile(path, []byte("---\ntitle: CLI Test\ndate: 2026-01-01T00:00:00Z\ndraft: true\n---\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		root := NewRootCmd()
		var outBuf bytes.Buffer
		root.SetOut(&outBuf)
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"schedule", path, "2026-04-15T08:00:00Z"})

		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a, err := article.Read(path)
		if err != nil {
			t.Fatalf("read after CLI schedule: %v", err)
		}
		if a.Frontmatter.ScheduledAt == nil {
			t.Fatal("expected scheduledAt to be set")
		}
	})

	t.Run("unschedule command clears scheduledAt via CLI", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "article.md")
		if err := os.WriteFile(path, []byte("---\ntitle: CLI Test\ndate: 2026-01-01T00:00:00Z\ndraft: true\nscheduledAt: 2026-04-15T08:00:00Z\n---\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		root := NewRootCmd()
		var outBuf bytes.Buffer
		root.SetOut(&outBuf)
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"unschedule", path})

		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a, err := article.Read(path)
		if err != nil {
			t.Fatalf("read after CLI unschedule: %v", err)
		}
		if a.Frontmatter.ScheduledAt != nil {
			t.Errorf("expected scheduledAt to be nil, got %v", a.Frontmatter.ScheduledAt)
		}
	})
}
