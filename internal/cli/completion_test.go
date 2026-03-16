package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletion_Bash(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected bash completion output, got empty")
	}
}

func TestCompletion_Zsh(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected zsh completion output, got empty")
	}
}

func TestCompletion_Fish(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "fish"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected fish completion output, got empty")
	}
}

func TestCompletion_InvalidShell(t *testing.T) {
	root := NewRootCmd()
	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	root.SetArgs([]string{"completion", "invalidshell"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid shell, got nil")
	}
	if !strings.Contains(err.Error(), "invalid argument") {
		t.Errorf("expected 'invalid argument' in error, got: %v", err)
	}
}
