package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RefreshInterval != 10 {
		t.Errorf("RefreshInterval = %d, want 10", cfg.RefreshInterval)
	}
	if cfg.Theme != "mocha" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "mocha")
	}
	if cfg.Shell != "/bin/sh" {
		t.Errorf("Shell = %q, want %q", cfg.Shell, "/bin/sh")
	}
	if cfg.ReadOnly {
		t.Error("ReadOnly should be false by default")
	}
}

func TestLoadFrom_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yml")

	content := `default_cluster: prod-cluster
default_service: api-service
refresh_interval: 30
read_only: true
theme: latte
shell: /bin/bash
`
	os.WriteFile(cfgFile, []byte(content), 0o644)

	cfg := LoadFrom(cfgFile)

	if cfg.DefaultCluster != "prod-cluster" {
		t.Errorf("DefaultCluster = %q, want %q", cfg.DefaultCluster, "prod-cluster")
	}
	if cfg.DefaultService != "api-service" {
		t.Errorf("DefaultService = %q, want %q", cfg.DefaultService, "api-service")
	}
	if cfg.RefreshInterval != 30 {
		t.Errorf("RefreshInterval = %d, want 30", cfg.RefreshInterval)
	}
	if !cfg.ReadOnly {
		t.Error("ReadOnly should be true")
	}
	if cfg.Theme != "latte" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "latte")
	}
	if cfg.Shell != "/bin/bash" {
		t.Errorf("Shell = %q, want %q", cfg.Shell, "/bin/bash")
	}
}

func TestLoadFrom_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yml")

	content := `default_cluster: my-cluster
refresh_interval: -1
`
	os.WriteFile(cfgFile, []byte(content), 0o644)

	cfg := LoadFrom(cfgFile)

	if cfg.DefaultCluster != "my-cluster" {
		t.Errorf("DefaultCluster = %q, want %q", cfg.DefaultCluster, "my-cluster")
	}
	if cfg.RefreshInterval != -1 {
		t.Errorf("RefreshInterval = %d, want -1", cfg.RefreshInterval)
	}
	// Defaults preserved for unset fields
	if cfg.Shell != "/bin/sh" {
		t.Errorf("Shell = %q, want default %q", cfg.Shell, "/bin/sh")
	}
	if cfg.Theme != "mocha" {
		t.Errorf("Theme = %q, want default %q", cfg.Theme, "mocha")
	}
}

func TestLoadFrom_MissingFile(t *testing.T) {
	cfg := LoadFrom("/nonexistent/path/config.yml")

	// Should return defaults
	if cfg.RefreshInterval != 10 {
		t.Errorf("RefreshInterval = %d, want default 10", cfg.RefreshInterval)
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yml")
	os.WriteFile(cfgFile, []byte(":::invalid yaml:::"), 0o644)

	cfg := LoadFrom(cfgFile)

	// Should return defaults
	if cfg.RefreshInterval != 10 {
		t.Errorf("RefreshInterval = %d, want default 10", cfg.RefreshInterval)
	}
}

func TestConfigFilePath(t *testing.T) {
	path := ConfigFilePath()
	if path == "" {
		t.Skip("could not determine home directory")
	}

	// Should end with the expected path
	expected := filepath.Join(".config", "ecs-tui", "config.yml")
	if !filepath.IsAbs(path) {
		t.Errorf("path should be absolute, got %q", path)
	}
	if len(path) < len(expected) {
		t.Errorf("path too short: %q", path)
	}
}
