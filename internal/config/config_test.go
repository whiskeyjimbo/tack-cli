package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Default(t *testing.T) {
	// Load from non-existent file should return defaults
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output != "table" {
		t.Errorf("expected default output 'table', got %q", cfg.Output)
	}
	if cfg.Timeout != "30s" {
		t.Errorf("expected default timeout '30s', got %q", cfg.Timeout)
	}
}

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
output: json
timeout: 60s
default_registry: ghcr.io/custom
aliases:
  sg: aws ec2 describe_security_groups
  buckets: aws s3 list_buckets
plugin_defaults:
  aws:
    region: us-east-1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Output != "json" {
		t.Errorf("expected output 'json', got %q", cfg.Output)
	}
	if cfg.Timeout != "60s" {
		t.Errorf("expected timeout '60s', got %q", cfg.Timeout)
	}
	if cfg.DefaultRegistry != "ghcr.io/custom" {
		t.Errorf("expected custom registry, got %q", cfg.DefaultRegistry)
	}
	if cfg.Aliases["sg"] != "aws ec2 describe_security_groups" {
		t.Errorf("expected alias 'sg', got %q", cfg.Aliases["sg"])
	}
	if cfg.PluginDefaults["aws"]["region"] != "us-east-1" {
		t.Errorf("expected aws region default")
	}
}

func TestLoad_Malformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Invalid YAML
	if err := os.WriteFile(path, []byte("{{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for malformed config")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	cfg := DefaultConfig()

	t.Setenv("TACK_OUTPUT", "yaml")
	t.Setenv("TACK_TIMEOUT", "120s")

	cfg.ApplyEnvOverrides()

	if cfg.Output != "yaml" {
		t.Errorf("expected output 'yaml' from env, got %q", cfg.Output)
	}
	if cfg.Timeout != "120s" {
		t.Errorf("expected timeout '120s' from env, got %q", cfg.Timeout)
	}
}

func TestLoad_WithGroups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`output: json
groups:
  network:
    description: "Network inspection tools"
    plugins:
      - dns
      - http
  cloud:
    description: "Cloud provider tools"
    plugins:
      - aws
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(cfg.Groups))
	}

	net := cfg.Groups["network"]
	if net.Description != "Network inspection tools" {
		t.Errorf("expected network description, got %q", net.Description)
	}
	if len(net.Plugins) != 2 || net.Plugins[0] != "dns" || net.Plugins[1] != "http" {
		t.Errorf("expected [dns, http], got %v", net.Plugins)
	}

	cloud := cfg.Groups["cloud"]
	if len(cloud.Plugins) != 1 || cloud.Plugins[0] != "aws" {
		t.Errorf("expected [aws], got %v", cloud.Plugins)
	}
}

func TestValidateGroups_ReservedName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups = map[string]GroupConfig{
		"plugin": {Description: "test", Plugins: []string{"dns"}},
	}
	if err := cfg.ValidateGroups(); err == nil {
		t.Error("expected error for reserved group name")
	}
}

func TestValidateGroups_EmptyPlugins(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups = map[string]GroupConfig{
		"empty": {Description: "test", Plugins: []string{}},
	}
	// Empty plugin lists are allowed (groups may be in progress)
	if err := cfg.ValidateGroups(); err != nil {
		t.Errorf("unexpected error for empty plugins: %v", err)
	}
}

func TestValidateGroups_Valid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups = map[string]GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns", "http"}},
	}
	if err := cfg.ValidateGroups(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateGroups_NoGroups(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.ValidateGroups(); err != nil {
		t.Errorf("unexpected error with nil groups: %v", err)
	}
}

func TestSave_RoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups = map[string]GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns", "http"}},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := cfg.Save(path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if len(loaded.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(loaded.Groups))
	}

	net := loaded.Groups["network"]
	if net.Description != "Network tools" {
		t.Errorf("description mismatch: %q", net.Description)
	}
	if len(net.Plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(net.Plugins))
	}
}
