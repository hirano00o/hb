package cli

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
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

// TestConfigInit_Project_Creates verifies that `config init` (without -g) creates .hb/config.yaml
// and that empty-Enter inputs result in no fields written to the file.
func TestConfigInit_Project_Creates(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	cmd := newConfigInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// Hatena ID (empty), Blog ID (empty), API Key (empty)
	cmd.SetIn(strings.NewReader("\n\n\n"))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfgPath := filepath.Join(dir, ".hb", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("expected .hb/config.yaml to exist: %v", err)
	}
	if !strings.Contains(out.String(), "Project config created at") {
		t.Errorf("unexpected output: %q", out.String())
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	for _, field := range []string{"hatena_id", "blog_id", "api_key"} {
		if strings.Contains(content, field) {
			t.Errorf("expected %s not to appear in config, got:\n%s", field, content)
		}
	}
}

// TestConfigInit_Project_OverwriteYes verifies that an existing project config is overwritten when user confirms.
func TestConfigInit_Project_OverwriteYes(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	if err := os.MkdirAll(".hb", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".hb/config.yaml", []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newConfigInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// confirm overwrite=y, then Hatena ID (empty), Blog ID (empty), API Key (empty)
	cmd.SetIn(strings.NewReader("y\n\n\n\n"))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "Aborted.") {
		t.Errorf("should not abort, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "Project config created at") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

// TestConfigInit_Project_OverwriteNo verifies that an existing project config is not overwritten when user declines.
func TestConfigInit_Project_OverwriteNo(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	if err := os.MkdirAll(".hb", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".hb/config.yaml", []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newConfigInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("n\n"))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Aborted.") {
		t.Errorf("expected Aborted., got: %q", out.String())
	}
}

// TestConfigInit_Project_WithInput verifies that interactive input values are written to the project config.
func TestConfigInit_Project_WithInput(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	cmd := newConfigInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// Hatena ID, Blog ID, API Key
	cmd.SetIn(strings.NewReader("myid\nmyblog.hateblo.jp\nmyapikey\n"))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfgPath := filepath.Join(dir, ".hb", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	for _, want := range []string{"hatena_id: myid", "blog_id: myblog.hateblo.jp", "api_key: myapikey"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in config, got:\n%s", want, content)
		}
	}
}

// TestConfigInit_Project_WithFlags verifies that --hatena-id and --blog-id flags are written to the project config.
func TestConfigInit_Project_WithFlags(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	cmd := newConfigInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// API Key (empty) — only API Key is prompted when flags are provided
	cmd.SetIn(strings.NewReader("\n"))

	if err := cmd.Flags().Set("hatena-id", "flagid"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("blog-id", "flagblog.hateblo.jp"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfgPath := filepath.Join(dir, ".hb", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	for _, want := range []string{"hatena_id: flagid", "blog_id: flagblog.hateblo.jp"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in config, got:\n%s", want, content)
		}
	}
	if strings.Contains(content, "api_key") {
		t.Errorf("expected api_key not to appear in config (empty input), got:\n%s", content)
	}
}

// TestConfigInit_Global_Creates verifies that `config init -g` creates the global config via interactive input.
func TestConfigInit_Global_Creates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cmd := newConfigInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// Hatena ID, Blog ID, API Key (non-terminal path)
	cmd.SetIn(strings.NewReader("myid\nmyblog.hateblo.jp\nmyapikey\n"))

	if err := cmd.Flags().Set("global", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "hb", "config.yaml")); err != nil {
		t.Errorf("expected global config to exist: %v", err)
	}
	if !strings.Contains(out.String(), "Global config saved to") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

// TestConfigInit_Global_OverwriteYes verifies that an existing global config is overwritten when user confirms.
func TestConfigInit_Global_OverwriteYes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "hb")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newConfigInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// confirm overwrite=y, then Hatena ID, Blog ID, API Key
	cmd.SetIn(strings.NewReader("y\nmyid\nmyblog.hateblo.jp\nmyapikey\n"))

	if err := cmd.Flags().Set("global", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "Aborted.") {
		t.Errorf("should not abort, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "Global config saved to") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

// TestConfigInit_Global_OverwriteNo verifies that an existing global config is not overwritten when user declines.
func TestConfigInit_Global_OverwriteNo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "hb")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newConfigInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("n\n"))

	if err := cmd.Flags().Set("global", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Aborted.") {
		t.Errorf("expected Aborted., got: %q", out.String())
	}
}
