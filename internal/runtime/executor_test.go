package runtime_test

import (
	"context"
	"os"
	"testing"

	"github.com/reglet-dev/cli/internal/runtime"
)

// testDNSWASMPath returns the path to the DNS plugin WASM binary.
// Skip the test if the binary is not found.
func testDNSWASMPath(t *testing.T) []byte {
	t.Helper()

	// Try relative path from project root (when running via go test ./...)
	paths := []string{
		"../../../reglet-plugins/plugins/dns/dns.wasm", // from internal/runtime/
		"../../reglet-plugins/plugins/dns/dns.wasm",    // from internal/
		"../reglet-plugins/plugins/dns/dns.wasm",       // project root relative to cli/
		"reglet-plugins/plugins/dns/dns.wasm",          // project root
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return data
		}
	}

	t.Skip("DNS WASM binary not found; build it with: make -C reglet-plugins/plugins/dns build")
	return nil
}

func TestPluginRunner_LoadPlugin(t *testing.T) {
	wasmBytes := testDNSWASMPath(t)
	ctx := context.Background()

	runner, err := runtime.NewPluginRunner(ctx)
	if err != nil {
		t.Fatalf("NewPluginRunner: %v", err)
	}
	defer runner.Close(ctx)

	plugin, err := runner.LoadPlugin(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}

	// Verify manifest
	if plugin.Manifest.Name != "dns" {
		t.Errorf("expected manifest name 'dns', got %q", plugin.Manifest.Name)
	}
	if len(plugin.Manifest.Services) == 0 {
		t.Error("expected at least one service in manifest")
	}

	// Verify the dns service exists
	svc, ok := plugin.Manifest.Services["dns"]
	if !ok {
		t.Fatal("expected 'dns' service in manifest")
	}
	if len(svc.Operations) == 0 {
		t.Error("expected at least one operation in dns service")
	}
}

func TestPluginRunner_Check(t *testing.T) {
	wasmBytes := testDNSWASMPath(t)
	ctx := context.Background()

	runner, err := runtime.NewPluginRunner(ctx)
	if err != nil {
		t.Fatalf("NewPluginRunner: %v", err)
	}
	defer runner.Close(ctx)

	plugin, err := runner.LoadPlugin(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}

	// Execute a DNS resolve
	result, err := plugin.Check(ctx, map[string]any{
		"hostname":    "example.com",
		"record_type": "A",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	// The result should have a status
	if result.Status == "" {
		t.Error("expected non-empty result status")
	}

	// Data should contain hostname echo
	if result.Data == nil {
		t.Fatal("expected non-nil result data")
	}
	if result.Data["hostname"] != "example.com" {
		t.Errorf("expected hostname 'example.com' in data, got %v", result.Data["hostname"])
	}
}
