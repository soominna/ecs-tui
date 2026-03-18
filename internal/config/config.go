package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the settings from ~/.config/ecs-tui/config.yml.
type Config struct {
	DefaultCluster  string `yaml:"default_cluster"`
	DefaultService  string `yaml:"default_service"`
	RefreshInterval int    `yaml:"refresh_interval"` // seconds, -1 disables auto-refresh
	ReadOnly        bool   `yaml:"read_only"`
	Theme           string `yaml:"theme"`  // "mocha" or "latte"
	Shell           string `yaml:"shell"`  // shell for exec (default: /bin/sh)
}

// DefaultConfig returns the default configuration values.
func DefaultConfig() *Config {
	return &Config{
		RefreshInterval: 10,
		Theme:           "mocha",
		Shell:           "/bin/sh",
	}
}

// ConfigFilePath returns the config file path.
func ConfigFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "ecs-tui", "config.yml")
}

// Load reads config.yml and returns a Config.
// Returns defaults if the file doesn't exist or fails to parse.
func Load() *Config {
	cfg := DefaultConfig()
	path := ConfigFilePath()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return DefaultConfig()
	}

	return cfg
}

// LoadFrom reads a config from the specified path.
func LoadFrom(path string) *Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return DefaultConfig()
	}

	return cfg
}
