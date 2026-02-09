package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reglet-dev/cli/internal/config"
	pluginpkg "github.com/reglet-dev/cli/internal/plugin"
)

func TestPluginCommand_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	stack, _ := pluginpkg.NewPluginStack(pluginpkg.PluginServiceConfig{CacheDir: dir})
	root := NewRootCommand(config.DefaultConfig(), stack)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"plugin", "list"})

	cmd := newPluginListCommand(stack)
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No plugins installed") {
		t.Errorf("expected empty list message, got: %s", output)
	}
}

func TestPluginCommand_InstallLocal(t *testing.T) {
	pluginsDir := t.TempDir()
	stack, _ := pluginpkg.NewPluginStack(pluginpkg.PluginServiceConfig{CacheDir: pluginsDir})

	// Create a fake WASM file
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "testplugin.wasm")
	_ = os.WriteFile(srcPath, []byte("fake wasm"), 0o644)

	cmd := newPluginInstallCommand(stack, "ghcr.io/reglet-dev")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{srcPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Verify it was installed in the structured cache
	if _, err := os.Stat(filepath.Join(pluginsDir, "testplugin", "plugin.wasm")); err != nil {
		t.Errorf("plugin not installed in structured cache: %v", err)
	}
}

func TestPluginCommand_Remove(t *testing.T) {
	pluginsDir := t.TempDir()
	stack, _ := pluginpkg.NewPluginStack(pluginpkg.PluginServiceConfig{CacheDir: pluginsDir})

	// Create a fake WASM file
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "testplugin.wasm")
	_ = os.WriteFile(srcPath, []byte("fake wasm"), 0o644)

	// Install it first so we can remove it
	installCmd := newPluginInstallCommand(stack, "")
	installCmd.SetArgs([]string{srcPath})
	if err := installCmd.Execute(); err != nil {
		t.Fatalf("failed to install for remove test: %v", err)
	}

	cmd := newPluginRemoveCommand(stack)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"testplugin"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Verify it was removed
	if _, err := os.Stat(filepath.Join(pluginsDir, "testplugin", "plugin.wasm")); !os.IsNotExist(err) {
		t.Error("plugin not removed")
	}
}
