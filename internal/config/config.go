// Package config handles user configuration for the CLI.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds user configuration loaded from ~/.cli/config.yaml.
type Config struct {
	// Output is the default output format (table, json, yaml).
	Output string `yaml:"output"`

	// Timeout is the default operation timeout.
	Timeout string `yaml:"timeout"`

	// DefaultRegistry is the OCI registry prefix for plugin references.
	// When a user runs "cli plugin install dns", this prefix is prepended
	// to form the full OCI reference: "ghcr.io/reglet-dev/reglet-plugins/dns:latest"
	DefaultRegistry string `yaml:"default_registry"`

	// RequireSigning controls whether plugins must have valid cosign signatures.
	RequireSigning bool `yaml:"require_signing"`

	// Quiet suppresses all output except exit code.
	Quiet bool `yaml:"quiet"`

	// Aliases maps short names to full command strings.
	// Example: {"sg": "aws ec2 describe_security_groups"}
	Aliases map[string]string `yaml:"aliases"`

	// PluginDefaults holds per-plugin default flag values.
	// Example: {"aws": {"region": "us-east-1"}}
	PluginDefaults map[string]map[string]string `yaml:"plugin_defaults"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Output:          "table",
		Timeout:         "30s",
		DefaultRegistry: "ghcr.io/reglet-dev/reglet-plugins",
	}
}

// Load reads configuration from the given path.
// Returns DefaultConfig if the file doesn't exist.
// Returns an error only if the file exists but is malformed.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}

// DefaultConfigPath returns the default config file path.
// ~/.cli/config.yaml
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".cli", "config.yaml")
	}
	return filepath.Join(home, ".cli", "config.yaml")
}

// DefaultConfigDir returns the default config directory.
// ~/.cli/
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".cli")
	}
	return filepath.Join(home, ".cli")
}

// ApplyEnvOverrides applies environment variable overrides to the config.
//
// Environment variables (higher priority than config file):
//   - CLI_OUTPUT: default output format
//   - CLI_TIMEOUT: default timeout
//   - CLI_DEFAULT_REGISTRY: OCI registry prefix
func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("CLI_OUTPUT"); v != "" {
		c.Output = v
	}
	if v := os.Getenv("CLI_TIMEOUT"); v != "" {
		c.Timeout = v
	}
	if v := os.Getenv("CLI_DEFAULT_REGISTRY"); v != "" {
		c.DefaultRegistry = v
	}
}
