// Package config handles user configuration for the CLI.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/whiskeyjimb/tack-cli/internal/meta"
	"gopkg.in/yaml.v3"
)

// Config holds user configuration loaded from ~/.tack/config.yaml.
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

	// Indexes lists additional plugin search indexes.
	Indexes []IndexSource `yaml:"indexes"`

	// Groups maps group names to their configuration.
	// Plugins in a group are accessed as: tack <group> <plugin> <operation>
	Groups map[string]GroupConfig `yaml:"groups,omitempty"`
}

// IndexSource defines a plugin index location.
type IndexSource struct {
	URL  string `yaml:"url"`
	Name string `yaml:"name"`
}

// GroupConfig defines a named plugin group.
type GroupConfig struct {
	// Description is the help text shown for the group command.
	Description string `yaml:"description"`

	// Plugins lists plugin names that belong to this group.
	Plugins []string `yaml:"plugins"`
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
// ~/.tack/config.yaml
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "."+meta.AppName, "config.yaml")
	}
	return filepath.Join(home, "."+meta.AppName, "config.yaml")
}

// DefaultConfigDir returns the default config directory.
// ~/.tack/
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "."+meta.AppName)
	}
	return filepath.Join(home, "."+meta.AppName)
}

// ApplyEnvOverrides applies environment variable overrides to the config.
//
// Environment variables (higher priority than config file):
//   - TACK_OUTPUT: default output format
//   - TACK_TIMEOUT: default timeout
//   - TACK_DEFAULT_REGISTRY: OCI registry prefix
func (c *Config) ApplyEnvOverrides() {
	prefix := strings.ToUpper(meta.AppName) + "_"
	if v := os.Getenv(prefix + "OUTPUT"); v != "" {
		c.Output = v
	}
	if v := os.Getenv(prefix + "TIMEOUT"); v != "" {
		c.Timeout = v
	}
	if v := os.Getenv(prefix + "DEFAULT_REGISTRY"); v != "" {
		c.DefaultRegistry = v
	}
}

// reservedCommands lists built-in command names that cannot be used as group names.
var reservedCommands = map[string]bool{
	"completion": true,
	"version":    true,
	"plugin":     true,
	"group":      true,
	"help":       true,
}

// ValidateGroups checks group configuration for errors.
// Only checks for critical errors (empty name, reserved name).
// Empty plugin lists are allowed since groups may be in the process of being configured.
func (c *Config) ValidateGroups() error {
	for name := range c.Groups {
		if name == "" {
			return fmt.Errorf("group name cannot be empty")
		}
		if reservedCommands[name] {
			return fmt.Errorf("group name %q conflicts with built-in command", name)
		}
	}
	return nil
}

// Save writes the config to the given path as YAML.
// Creates parent directories if they don't exist.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
