package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

// TestWatch_MutuallyExclusive verifies that providing both a file argument and --dir errors.
func TestWatch_MutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "a.md", "---\ntitle: A\ndate: 2026-01-01T00:00:00Z\ndraft: false\n---\nbody\n")

	cmd := newWatchCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--dir", dir, filepath.Join(dir, "a.md")})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when both --dir and file argument are given")
	}
}

// TestWatch_NoMDFiles verifies that an empty directory prints a message and exits cleanly.
func TestWatch_NoMDFiles(t *testing.T) {
	dir := t.TempDir()

	cmd := newWatchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No .md files found") {
		t.Errorf("expected 'No .md files found' message, got: %s", out.String())
	}
}

// TestWatch_FileChange_TriggersPush verifies that modifying a watched file triggers pushFileFunc.
func TestWatch_FileChange_TriggersPush(t *testing.T) {
	dir := t.TempDir()
	fm := article.Frontmatter{
		Title: "Watch Test",
		Date:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Draft: true,
	}
	a := &article.Article{Frontmatter: fm, Body: "original\n"}
	path := filepath.Join(dir, "watch_test.md")
	if err := article.Write(path, a); err != nil {
		t.Fatal(err)
	}

	// Track push calls via a channel.
	pushed := make(chan string, 1)
	origPushFileFunc := pushFileFunc
	t.Cleanup(func() { pushFileFunc = origPushFileFunc })
	pushFileFunc = func(_ *cobra.Command, p string) error {
		pushed <- p
		return nil
	}

	debounce := 50 * time.Millisecond
	paths := []string{path}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := newWatchCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	// Run watch in a goroutine; cancel context to stop it after the test.
	done := make(chan error, 1)
	go func() {
		done <- runWatch(ctx, cmd, paths, debounce)
	}()

	// Give the watcher time to set up.
	time.Sleep(100 * time.Millisecond)

	// Modify the file to trigger a push.
	if err := os.WriteFile(path, []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for push or timeout.
	select {
	case p := <-pushed:
		abs, _ := filepath.Abs(path)
		if p != abs {
			t.Errorf("expected push for %q, got %q", abs, p)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for push after file change")
	}

	cancel()
	<-done
}
