package cli

import (
	"testing"

	abi "github.com/reglet-dev/reglet-abi"
	"github.com/spf13/cobra"
	"github.com/whiskeyjimb/tack-cli/internal/config"
	pluginpkg "github.com/whiskeyjimb/tack-cli/internal/plugin"
)

// fakeDiscoveredPlugin creates a minimal DiscoveredPlugin for testing.
func fakeDiscoveredPlugin(name string) pluginpkg.DiscoveredPlugin {
	return pluginpkg.DiscoveredPlugin{
		Manifest: abi.Manifest{
			Name:        name,
			Description: name + " plugin",
			Services: map[string]abi.ServiceManifest{
				name: {
					Name: name,
					Operations: []abi.OperationManifest{
						{Name: "check", Description: "Run check"},
					},
				},
			},
		},
		Loader: func() ([]byte, error) { return nil, nil },
	}
}

// fakeGenerateFn creates a simple cobra command from a DiscoveredPlugin for testing.
func fakeGenerateFn(dp pluginpkg.DiscoveredPlugin) *cobra.Command {
	cmd := &cobra.Command{
		Use:   dp.Manifest.Name,
		Short: dp.Manifest.Description,
	}
	for _, svc := range dp.Manifest.Services {
		for _, op := range svc.Operations {
			cmd.AddCommand(&cobra.Command{
				Use:   op.Name,
				Short: op.Description,
			})
		}
	}
	return cmd
}

func TestRegisterGroups_Basic(t *testing.T) {
	root := &cobra.Command{Use: "tack"}

	discovered := []pluginpkg.DiscoveredPlugin{
		fakeDiscoveredPlugin("dns"),
		fakeDiscoveredPlugin("http"),
		fakeDiscoveredPlugin("aws"),
	}

	groups := map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns", "http"}},
		"cloud":   {Description: "Cloud tools", Plugins: []string{"aws"}},
	}

	topPlugins := registerGroups(root, groups, discovered, fakeGenerateFn)

	// Since there's no "top" group, topPlugins should be empty
	if len(topPlugins) != 0 {
		t.Errorf("expected 0 top-level plugins, got %d", len(topPlugins))
	}

	// Root should have 2 subcommands (network, cloud)
	cmds := root.Commands()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 group commands, got %d", len(cmds))
	}

	// Find "network" group and check it has dns, http
	var networkCmd *cobra.Command
	for _, c := range cmds {
		if c.Name() == "network" {
			networkCmd = c
		}
	}
	if networkCmd == nil {
		t.Fatal("expected 'network' group command")
	}
	if len(networkCmd.Commands()) != 2 {
		t.Errorf("expected 2 plugins in network, got %d", len(networkCmd.Commands()))
	}
}

func TestRegisterGroups_MissingPlugin(t *testing.T) {
	root := &cobra.Command{Use: "tack"}

	discovered := []pluginpkg.DiscoveredPlugin{
		fakeDiscoveredPlugin("dns"),
	}

	groups := map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns", "nonexistent"}},
	}

	topPlugins := registerGroups(root, groups, discovered, fakeGenerateFn)

	// No "top" group, so topPlugins should be empty
	if len(topPlugins) != 0 {
		t.Errorf("expected 0 top-level plugins, got %d", len(topPlugins))
	}

	// Group should still be added with 1 plugin
	if len(root.Commands()) != 1 {
		t.Fatalf("expected 1 group command, got %d", len(root.Commands()))
	}
}

func TestRegisterGroups_AllPluginsMissing(t *testing.T) {
	root := &cobra.Command{Use: "tack"}

	groups := map[string]config.GroupConfig{
		"empty": {Description: "Empty group", Plugins: []string{"nonexistent"}},
	}

	topPlugins := registerGroups(root, groups, nil, fakeGenerateFn)

	if len(topPlugins) != 0 {
		t.Errorf("expected no top-level plugins, got %d", len(topPlugins))
	}

	// Group should NOT be added
	if len(root.Commands()) != 0 {
		t.Errorf("expected 0 commands, got %d", len(root.Commands()))
	}
}

func TestRegisterGroups_PluginInMultipleGroups(t *testing.T) {
	root := &cobra.Command{Use: "tack"}

	discovered := []pluginpkg.DiscoveredPlugin{
		fakeDiscoveredPlugin("dns"),
	}

	groups := map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns"}},
		"debug":   {Description: "Debug tools", Plugins: []string{"dns"}},
	}

	topPlugins := registerGroups(root, groups, discovered, fakeGenerateFn)

	// No "top" group
	if len(topPlugins) != 0 {
		t.Errorf("expected 0 top-level plugins, got %d", len(topPlugins))
	}

	// Both groups should exist
	if len(root.Commands()) != 2 {
		t.Fatalf("expected 2 group commands, got %d", len(root.Commands()))
	}

	// Each should have its own dns command (distinct instances)
	for _, groupCmd := range root.Commands() {
		if len(groupCmd.Commands()) != 1 {
			t.Errorf("group %q: expected 1 plugin, got %d", groupCmd.Name(), len(groupCmd.Commands()))
		}
	}
}

func TestRegisterGroups_TopGroup(t *testing.T) {
	root := &cobra.Command{Use: "tack"}

	discovered := []pluginpkg.DiscoveredPlugin{
		fakeDiscoveredPlugin("dns"),
		fakeDiscoveredPlugin("http"),
	}

	groups := map[string]config.GroupConfig{
		"top":     {Description: "Top-level plugins", Plugins: []string{"dns"}},
		"network": {Description: "Network tools", Plugins: []string{"dns", "http"}},
	}

	topPlugins := registerGroups(root, groups, discovered, fakeGenerateFn)

	// Only "dns" should be marked for top-level
	if len(topPlugins) != 1 || !topPlugins["dns"] {
		t.Errorf("expected only dns at top level, got %v", topPlugins)
	}

	// Only "network" group command should exist (not "top")
	if len(root.Commands()) != 1 {
		t.Fatalf("expected 1 group command (not including top), got %d", len(root.Commands()))
	}
}
