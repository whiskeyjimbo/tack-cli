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
		"../runtime/testdata/fixture.wasm", // relative to internal/plugin
		"../../internal/runtime/testdata/fixture.wasm",
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
		if p.Manifest.Name == "fixture" {
			found = true
			if p.Source != "local" {
				t.Errorf("expected source 'local', got %q", p.Source)
			}
		}
	}
	if !found {
		t.Error("Fixture plugin not found in discovered plugins")
	}
}

func TestDefaultPluginsDir(t *testing.T) {
	dir := DefaultPluginsDir()
	if dir == "" {
		t.Error("expected non-empty plugins dir")
	}
}

func TestLoader_OnDemandLoading(t *testing.T) {
	// This test verifies that the loader reads WASM bytes on-demand rather than
	// holding them in memory from discovery time (memory bloat fix).
	wasmPaths := []string{
		"../runtime/testdata/fixture.wasm",
		"../../internal/runtime/testdata/fixture.wasm",
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
		t.Skip("Fixture WASM binary not found")
	}

	// Set up a temp plugins dir with the WASM file
	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "test.wasm")
	if err := os.WriteFile(wasmPath, wasmData, 0o644); err != nil {
		t.Fatalf("Failed to write test plugin: %v", err)
	}

	loader := NewLoader(embed.FS{}, dir, nil, "")
	plugins, err := loader.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}

	if len(plugins) == 0 {
		t.Fatal("expected at least 1 discovered plugin")
	}

	// Get the first plugin
	plugin := plugins[0]

	// Call the Loader function to read the WASM bytes
	loadedBytes, err := plugin.Loader()
	if err != nil {
		t.Fatalf("Loader() failed: %v", err)
	}

	// Verify the loaded bytes match the original data
	if len(loadedBytes) != len(wasmData) {
		t.Errorf("Expected loaded bytes length %d, got %d", len(wasmData), len(loadedBytes))
	}

	// Verify we can call the loader multiple times (idempotent)
	loadedBytes2, err := plugin.Loader()
	if err != nil {
		t.Fatalf("Second Loader() call failed: %v", err)
	}

	if len(loadedBytes2) != len(wasmData) {
		t.Errorf("Second load: expected bytes length %d, got %d", len(wasmData), len(loadedBytes2))
	}

	// Now modify the file to verify it reads from disk each time
	modifiedData := append([]byte{0x00}, wasmData[1:]...)
	if err := os.WriteFile(wasmPath, modifiedData, 0o644); err != nil {
		t.Fatalf("Failed to modify test plugin: %v", err)
	}

	// Read again - should get the modified data
	loadedBytes3, err := plugin.Loader()
	if err != nil {
		t.Fatalf("Third Loader() call failed: %v", err)
	}

	if loadedBytes3[0] != 0x00 {
		t.Error("Loader did not read modified file from disk (still using cached bytes)")
	}
}
