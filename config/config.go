// Package config manages hb configuration files.
// It supports a global config and a project-local config that overrides the global.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds the credentials and settings for Hatena Blog API access.
type Config struct {
	HatenaID    string `yaml:"hatena_id,omitempty"`
	BlogID      string `yaml:"blog_id,omitempty"`
	APIKey      string `yaml:"api_key,omitempty"`
	Concurrency int    `yaml:"concurrency,omitempty"`
}

// GlobalConfigPath returns the path to the global config file.
// Uses $XDG_CONFIG_HOME if set, otherwise defaults to ~/.config.
func GlobalConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "hb", "config.yaml"), nil
}

// ProjectConfigPath walks from the current directory up to the root looking for .hb/config.yaml.
func ProjectConfigPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for {
		candidate := filepath.Join(dir, ".hb", "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("project config not found: .hb/config.yaml")
}

// Load reads a Config from the given YAML file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes cfg to the given YAML file path, creating parent directories as needed.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// Merge returns a new Config with global as the base, overridden by any non-empty fields from project.
func Merge(global, project *Config) *Config {
	merged := *global
	if project.HatenaID != "" {
		merged.HatenaID = project.HatenaID
	}
	if project.BlogID != "" {
		merged.BlogID = project.BlogID
	}
	if project.APIKey != "" {
		merged.APIKey = project.APIKey
	}
	if project.Concurrency != 0 {
		merged.Concurrency = project.Concurrency
	}
	return &merged
}

// LoadMerged loads the global config, optionally merges the project config if found,
// then applies environment variable overrides (HB_HATENA_ID, HB_BLOG_ID, HB_API_KEY, HB_CONCURRENCY).
// Priority: env vars > project config > global config.
func LoadMerged() (*Config, error) {
	globalPath, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}
	global, err := Load(globalPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if global == nil {
		global = &Config{}
	}

	var merged *Config
	projectPath, err := ProjectConfigPath()
	if err != nil {
		// no project config; use global only
		merged = global
	} else {
		project, err := Load(projectPath)
		if err != nil {
			return nil, err
		}
		merged = Merge(global, project)
	}

	if v := os.Getenv("HB_HATENA_ID"); v != "" {
		merged.HatenaID = v
	}
	if v := os.Getenv("HB_BLOG_ID"); v != "" {
		merged.BlogID = v
	}
	if v := os.Getenv("HB_API_KEY"); v != "" {
		merged.APIKey = v
	}
	if v := os.Getenv("HB_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("HB_CONCURRENCY must be a positive integer, got %q", v)
		}
		merged.Concurrency = n
	}
	return merged, nil
}

// Validate returns an error if any required field is empty.
func Validate(cfg *Config) error {
	if cfg.HatenaID == "" {
		return errors.New("hatena_id is required")
	}
	if cfg.BlogID == "" {
		return errors.New("blog_id is required")
	}
	if cfg.APIKey == "" {
		return errors.New("api_key is required")
	}
	return nil
}
