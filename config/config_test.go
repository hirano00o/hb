package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hirano00o/hb/config"
)

func TestLoad_OK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("hatena_id: user\nblog_id: blog\napi_key: key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HatenaID != "user" || cfg.BlogID != "blog" || cfg.APIKey != "key" {
		t.Errorf("got %+v", cfg)
	}
}

func TestLoad_NotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(":\tinvalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")
	cfg := &config.Config{HatenaID: "u", BlogID: "b", APIKey: "k"}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.HatenaID != "u" || loaded.BlogID != "b" || loaded.APIKey != "k" {
		t.Errorf("got %+v", loaded)
	}
}

func TestMerge(t *testing.T) {
	global := &config.Config{HatenaID: "g_user", BlogID: "g_blog", APIKey: "g_key"}
	project := &config.Config{BlogID: "p_blog"}
	merged := config.Merge(global, project)
	if merged.HatenaID != "g_user" {
		t.Errorf("HatenaID should come from global, got %s", merged.HatenaID)
	}
	if merged.BlogID != "p_blog" {
		t.Errorf("BlogID should be overridden by project, got %s", merged.BlogID)
	}
	if merged.APIKey != "g_key" {
		t.Errorf("APIKey should come from global, got %s", merged.APIKey)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr string
	}{
		{"ok", &config.Config{HatenaID: "u", BlogID: "b", APIKey: "k"}, ""},
		{"no hatena_id", &config.Config{BlogID: "b", APIKey: "k"}, "hatena_id"},
		{"no blog_id", &config.Config{HatenaID: "u", APIKey: "k"}, "blog_id"},
		{"no api_key", &config.Config{HatenaID: "u", BlogID: "b"}, "api_key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.Validate(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGlobalConfigPath(t *testing.T) {
	path, err := config.GlobalConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("hb", "config.yaml")) {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestGlobalConfigPath_XDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path, err := config.GlobalConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "hb", "config.yaml")
	if path != want {
		t.Errorf("want %s, got %s", want, path)
	}
}
