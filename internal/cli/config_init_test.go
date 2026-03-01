package cli

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestReadPassword_NonTerminal_Input verifies that readPassword returns the typed text
// when stdin is not a terminal (pipe fallback path).
func TestReadPassword_NonTerminal_Input(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("mysecret\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	scanner := bufio.NewScanner(cmd.InOrStdin())
	got, err := readPassword(cmd, scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "mysecret" {
		t.Errorf("got %q, want %q", got, "mysecret")
	}
}

// TestReadPassword_NonTerminal_EOF verifies that readPassword returns io.ErrUnexpectedEOF
// when stdin closes without providing any input.
func TestReadPassword_NonTerminal_EOF(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	var out bytes.Buffer
	cmd.SetOut(&out)

	scanner := bufio.NewScanner(cmd.InOrStdin())
	_, err := readPassword(cmd, scanner)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("got %v, want io.ErrUnexpectedEOF", err)
	}
}

// TestReadPassword_NonTerminal_Whitespace verifies that leading/trailing whitespace
// is trimmed from the input.
func TestReadPassword_NonTerminal_Whitespace(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("  trimmed  \n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	scanner := bufio.NewScanner(cmd.InOrStdin())
	got, err := readPassword(cmd, scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "trimmed" {
		t.Errorf("got %q, want %q", got, "trimmed")
	}
}
