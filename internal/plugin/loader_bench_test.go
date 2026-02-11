package plugin

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkLoader_DiscoveryMemory benchmarks the memory usage during plugin discovery.
// This benchmark helps verify that the on-demand loader fix reduces memory consumption
// during cold starts (e.g., displaying help or version info).
func BenchmarkLoader_DiscoveryMemory(b *testing.B) {
	// This test requires a real WASM binary
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
		b.Skip("Fixture WASM binary not found")
	}

	// Set up a temp plugins dir with multiple copies of the WASM file
	// to simulate a realistic scenario with many plugins
	dir := b.TempDir()
	for i := 0; i < 20; i++ {
		filename := filepath.Join(dir, "plugin"+string(rune('a'+i))+".wasm")
		if err := os.WriteFile(filename, wasmData, 0o644); err != nil {
			b.Fatalf("Failed to write test plugin: %v", err)
		}
	}

	loader := NewLoader(embed.FS{}, dir, nil, "")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		plugins, err := loader.DiscoverAll(context.Background())
		if err != nil {
			b.Fatalf("DiscoverAll: %v", err)
		}
		if len(plugins) == 0 {
			b.Fatal("expected at least 1 discovered plugin")
		}
		// Don't call the Loader() function - we're measuring discovery memory,
		// not execution memory
	}
}
