package cli

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

// mockNoOpEditor simulates an editor that exits without changing the file.
func mockNoOpEditor(name string, args ...string) *exec.Cmd {
	return exec.Command("true")
}

// mockModifyEditorFor returns a mock editor function that overwrites the target file with newContent.
func mockModifyEditorFor(newContent string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		if len(args) == 0 {
			return exec.Command("true")
		}
		script := "printf '%s' '" + strings.ReplaceAll(newContent, "'", "'\\''") + "' > " + args[0]
		return exec.Command("sh", "-c", script)
	}
}

type editTestCmd struct {
	cmd *cobra.Command
	out *bytes.Buffer
	err *bytes.Buffer
}

func newEditTestCmd(t *testing.T, in *strings.Reader) editTestCmd {
	t.Helper()
	cmd := &cobra.Command{Use: "edit"}
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if in != nil {
		cmd.SetIn(in)
	} else {
		cmd.SetIn(strings.NewReader(""))
	}
	cmd.SetContext(t.Context())
	return editTestCmd{cmd: cmd, out: &out, err: &errBuf}
}

func TestEdit_NoChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	a := &article.Article{
		Frontmatter: article.Frontmatter{
			Title: "Test Article",
			Date:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		Body: "hello\n",
	}
	if err := article.Write(path, a); err != nil {
		t.Fatal(err)
	}

	origExec := execCommand
	execCommand = mockNoOpEditor
	t.Cleanup(func() { execCommand = origExec })

	tc := newEditTestCmd(t, nil)
	if err := runEdit(tc.cmd, path, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(tc.out.String(), "No changes.") {
		t.Errorf("expected 'No changes.' in output, got %q", tc.out.String())
	}
}

func TestEdit_FileNotFound(t *testing.T) {
	origExec := execCommand
	execCommand = mockNoOpEditor
	t.Cleanup(func() { execCommand = origExec })

	tc := newEditTestCmd(t, nil)
	err := runEdit(tc.cmd, "/nonexistent/missing.md", false)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestEdit_ChangesDetected_Abort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	a := &article.Article{
		Frontmatter: article.Frontmatter{
			Title: "Test",
			Date:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		Body: "original\n",
	}
	if err := article.Write(path, a); err != nil {
		t.Fatal(err)
	}

	origExec := execCommand
	execCommand = mockModifyEditorFor("---\ntitle: Test\ndate: 2026-03-01T00:00:00Z\n---\nmodified\n")
	t.Cleanup(func() { execCommand = origExec })

	// Stub newClientFromConfig; article has no editUrl so GetEntry is not called.
	origClient := newClientFromConfig
	t.Cleanup(func() { newClientFromConfig = origClient })
	newClientFromConfig = func() (*hatena.Client, error) {
		return hatena.NewClient("user", "blog", "key"), nil
	}

	tc := newEditTestCmd(t, strings.NewReader("N\n"))
	if err := runEdit(tc.cmd, path, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(tc.out.String(), "Aborted.") {
		t.Errorf("expected 'Aborted.' after N response, got %q", tc.out.String())
	}
}

func TestEdit_AutoPushFlag_CallsPush(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	a := &article.Article{
		Frontmatter: article.Frontmatter{
			Title: "Test",
			Date:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		Body: "original\n",
	}
	if err := article.Write(path, a); err != nil {
		t.Fatal(err)
	}

	origExec := execCommand
	execCommand = mockModifyEditorFor("---\ntitle: Test\ndate: 2026-03-01T00:00:00Z\n---\nmodified\n")
	t.Cleanup(func() { execCommand = origExec })

	pushCalled := false
	origClient := newClientFromConfig
	t.Cleanup(func() { newClientFromConfig = origClient })
	newClientFromConfig = func() (*hatena.Client, error) {
		pushCalled = true
		// Return a dummy client; push will fail but we just verify push path is reached.
		return hatena.NewClient("user", "blog", "key"), nil
	}

	tc := newEditTestCmd(t, nil)
	// Error is expected (no real server) but push path must be reached.
	_ = runEdit(tc.cmd, path, true)
	if !pushCalled {
		t.Error("expected newClientFromConfig to be called (push path reached)")
	}
}
