package cli_test

import (
	"bytes"
	"testing"

	"github.com/hirano00o/hb/internal/cli"
)

func TestVersionFlag(t *testing.T) {
	cmd := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// version defaults to "dev" unless injected via -ldflags at build time.
	want := "hb version dev\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
