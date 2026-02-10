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
