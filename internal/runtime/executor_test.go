package runtime_test

import (
	"context"
	"os"
	"testing"

	"github.com/whiskeyjimb/tack-cli/internal/runtime"
)

// testWASMPath returns the path to the test fixture WASM binary.
func testWASMPath(t *testing.T) []byte {
	t.Helper()

	path := "testdata/fixture.wasm"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Test fixture not found at %s. Build it in reglet/internal/infrastructure/wasm/testdata/fixture first.", path)
	}
	return data
}

func TestPluginRunner_LoadPlugin(t *testing.T) {
	wasmBytes := testWASMPath(t)
	ctx := context.Background()

	runner, err := runtime.NewPluginRunner(ctx, runtime.WithTrustPlugins(true))
	if err != nil {
		t.Fatalf("NewPluginRunner: %v", err)
	}
	defer func() { _ = runner.Close(ctx) }()

	plugin, err := runner.LoadPlugin(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}

	// Verify manifest
	if plugin.Manifest.Name != "fixture" {
		t.Errorf("expected manifest name 'fixture', got %q", plugin.Manifest.Name)
	}
}

func TestPluginRunner_Check(t *testing.T) {
	wasmBytes := testWASMPath(t)
	ctx := context.Background()

	runner, err := runtime.NewPluginRunner(ctx, runtime.WithTrustPlugins(true))
	if err != nil {
		t.Fatalf("NewPluginRunner: %v", err)
	}
	defer func() { _ = runner.Close(ctx) }()

	plugin, err := runner.LoadPlugin(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}

	// Execute a check
	result, err := plugin.Check(ctx, map[string]any{
		"action": "echo_test",
		"input":  "hello world",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	// The result should have a status
	if result.Status == "" {
		t.Error("expected non-empty result status")
	}

	// Data should contain echoed input
	if result.Data == nil {
		t.Fatal("expected non-nil result data")
	}
	if result.Data["echo"] != "hello world" {
		t.Errorf("expected echo 'hello world' in data, got %v", result.Data["echo"])
	}
}
