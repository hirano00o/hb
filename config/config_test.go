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

func intPtr(v int) *int { return &v }

func TestMerge_Concurrency(t *testing.T) {
	global := &config.Config{HatenaID: "u", BlogID: "b", APIKey: "k", Concurrency: intPtr(3)}
	project := &config.Config{Concurrency: intPtr(10)}
	merged := config.Merge(global, project)
	if merged.Concurrency == nil || *merged.Concurrency != 10 {
		t.Errorf("Concurrency should be overridden by project, got %v", merged.Concurrency)
	}

	// nil in project must not override global.
	project2 := &config.Config{}
	merged2 := config.Merge(global, project2)
	if merged2.Concurrency == nil || *merged2.Concurrency != 3 {
		t.Errorf("Concurrency should be kept from global when project is nil, got %v", merged2.Concurrency)
	}

	// Zero in project must override global (explicitly set to 0 is meaningful for MaxPages,
	// but Concurrency=0 means "use default", so this tests that nil is the sentinel for unset).
	project3 := &config.Config{Concurrency: intPtr(0)}
	merged3 := config.Merge(global, project3)
	if merged3.Concurrency == nil || *merged3.Concurrency != 0 {
		t.Errorf("Concurrency should be overridden to 0 when project explicitly sets it, got %v", merged3.Concurrency)
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

func TestLoadMerged_GlobalOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	globalDir := filepath.Join(dir, "hb")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"),
		[]byte("hatena_id: guser\nblog_id: gblog\napi_key: gkey\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Change to a directory without a project config.
	t.Chdir(t.TempDir())

	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HatenaID != "guser" || cfg.BlogID != "gblog" || cfg.APIKey != "gkey" {
		t.Errorf("got %+v", cfg)
	}
}

func TestLoadMerged_BothExist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	globalDir := filepath.Join(dir, "hb")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"),
		[]byte("hatena_id: guser\nblog_id: gblog\napi_key: gkey\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Create a project directory with a project config that overrides blog_id.
	projectDir := t.TempDir()
	hbDir := filepath.Join(projectDir, ".hb")
	if err := os.MkdirAll(hbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hbDir, "config.yaml"),
		[]byte("blog_id: pblog\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(projectDir)

	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HatenaID != "guser" {
		t.Errorf("HatenaID should be global, got %s", cfg.HatenaID)
	}
	if cfg.BlogID != "pblog" {
		t.Errorf("BlogID should be overridden by project, got %s", cfg.BlogID)
	}
	if cfg.APIKey != "gkey" {
		t.Errorf("APIKey should be global, got %s", cfg.APIKey)
	}
}

func TestProjectConfigPath_Found(t *testing.T) {
	root := t.TempDir()
	hbDir := filepath.Join(root, ".hb")
	if err := os.MkdirAll(hbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hbDir, "config.yaml"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	// Change to a child directory — ProjectConfigPath should walk up to find .hb/config.yaml.
	child := filepath.Join(root, "sub", "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)

	got, err := config.ProjectConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, ".hb", "config.yaml")
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}

func TestLoadMerged_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	globalDir := filepath.Join(dir, "hb")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"),
		[]byte("hatena_id: guser\nblog_id: gblog\napi_key: gkey\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	t.Setenv("HB_HATENA_ID", "envuser")
	t.Setenv("HB_BLOG_ID", "envblog")
	t.Setenv("HB_API_KEY", "envkey")

	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HatenaID != "envuser" {
		t.Errorf("HatenaID should be overridden by env, got %s", cfg.HatenaID)
	}
	if cfg.BlogID != "envblog" {
		t.Errorf("BlogID should be overridden by env, got %s", cfg.BlogID)
	}
	if cfg.APIKey != "envkey" {
		t.Errorf("APIKey should be overridden by env, got %s", cfg.APIKey)
	}
}

func TestProjectConfigPath_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := config.ProjectConfigPath()
	if err == nil {
		t.Fatal("expected error when project config not found")
	}
}

func TestLoadMerged_ConcurrencyEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	globalDir := filepath.Join(dir, "hb")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"),
		[]byte("hatena_id: u\nblog_id: b\napi_key: k\nconcurrency: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	t.Setenv("HB_CONCURRENCY", "8")
	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Concurrency == nil || *cfg.Concurrency != 8 {
		t.Errorf("Concurrency should be 8 from env, got %v", cfg.Concurrency)
	}
}

func TestMerge_MaxPages(t *testing.T) {
	global := &config.Config{HatenaID: "u", BlogID: "b", APIKey: "k", MaxPages: intPtr(3)}
	project := &config.Config{MaxPages: intPtr(10)}
	merged := config.Merge(global, project)
	if merged.MaxPages == nil || *merged.MaxPages != 10 {
		t.Errorf("MaxPages should be overridden by project, got %v", merged.MaxPages)
	}

	// nil in project must not override global.
	project2 := &config.Config{}
	merged2 := config.Merge(global, project2)
	if merged2.MaxPages == nil || *merged2.MaxPages != 3 {
		t.Errorf("MaxPages should be kept from global when project is nil, got %v", merged2.MaxPages)
	}

	// Zero in project must override global (0 = no limit).
	project3 := &config.Config{MaxPages: intPtr(0)}
	merged3 := config.Merge(global, project3)
	if merged3.MaxPages == nil || *merged3.MaxPages != 0 {
		t.Errorf("MaxPages should be overridden to 0 when project explicitly sets it, got %v", merged3.MaxPages)
	}
}

func TestLoadMerged_MaxPagesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	globalDir := filepath.Join(dir, "hb")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"),
		[]byte("hatena_id: u\nblog_id: b\napi_key: k\nmax_pages: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	t.Setenv("HB_MAX_PAGES", "5")
	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxPages == nil || *cfg.MaxPages != 5 {
		t.Errorf("MaxPages should be 5 from env, got %v", cfg.MaxPages)
	}
}

func TestLoadMerged_MaxPagesEnvZero(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	globalDir := filepath.Join(dir, "hb")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"),
		[]byte("hatena_id: u\nblog_id: b\napi_key: k\nmax_pages: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	// 0 is valid for MaxPages (means no limit).
	t.Setenv("HB_MAX_PAGES", "0")
	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxPages == nil || *cfg.MaxPages != 0 {
		t.Errorf("MaxPages should be 0 from env, got %v", cfg.MaxPages)
	}
}

func TestLoadMerged_MaxPagesEnvInvalid(t *testing.T) {
	setupGlobalConfig := func(t *testing.T) {
		t.Helper()
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		if err := os.MkdirAll(filepath.Join(dir, "hb"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "hb", "config.yaml"),
			[]byte("hatena_id: u\nblog_id: b\napi_key: k\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(t.TempDir())
	}

	tests := []struct {
		name  string
		value string
	}{
		{"non-integer", "invalid"},
		{"negative", "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupGlobalConfig(t)
			t.Setenv("HB_MAX_PAGES", tt.value)
			_, err := config.LoadMerged()
			if err == nil {
				t.Fatalf("expected error for HB_MAX_PAGES=%q", tt.value)
			}
		})
	}
}

func TestLoadMerged_ConcurrencyEnvInvalid(t *testing.T) {
	setupGlobalConfig := func(t *testing.T) {
		t.Helper()
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		if err := os.MkdirAll(filepath.Join(dir, "hb"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "hb", "config.yaml"),
			[]byte("hatena_id: u\nblog_id: b\napi_key: k\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(t.TempDir())
	}

	tests := []struct {
		name  string
		value string
	}{
		{"non-integer", "invalid"},
		{"zero", "0"},
		{"negative", "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupGlobalConfig(t)
			t.Setenv("HB_CONCURRENCY", tt.value)
			_, err := config.LoadMerged()
			if err == nil {
				t.Fatalf("expected error for HB_CONCURRENCY=%q", tt.value)
			}
		})
	}
}
