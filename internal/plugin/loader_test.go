package plugin

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_LoadLocalPlugins(t *testing.T) {
	// This test requires a real WASM binary
	wasmPaths := []string{
		"../reglet-plugins/plugins/dns/dns.wasm",
		"../../reglet-plugins/plugins/dns/dns.wasm",
		"/home/jrose/src/all-reglet/reglet-plugins/plugins/dns/dns.wasm",
	}

	var wasmData []byte
	for _, p := range wasmPaths {
		data, err := os.ReadFile(p)
		if err == nil {
			wasmData = data
			break
		}
	}
	if wasmData == nil {
		t.Skip("DNS WASM binary not found")
	}

	// Set up a temp plugins dir with the WASM file
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "dns.wasm"), wasmData, 0o644)

	loader := NewLoader(embed.FS{}, dir, nil, "")
	plugins, err := loader.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}

	if len(plugins) == 0 {
		t.Fatal("expected at least 1 discovered plugin")
	}

	found := false
	for _, p := range plugins {
		if p.Manifest.Name == "dns" {
			found = true
			if p.Source != "local" {
				t.Errorf("expected source 'local', got %q", p.Source)
			}
		}
	}
	if !found {
		t.Error("DNS plugin not found in discovered plugins")
	}
}

func TestDefaultPluginsDir(t *testing.T) {
	dir := DefaultPluginsDir()
	if dir == "" {
		t.Error("expected non-empty plugins dir")
	}
}
