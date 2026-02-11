// Package cli implements the command-line interface for Reglet.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/whiskeyjimb/tack-cli/internal/config"
	"github.com/whiskeyjimb/tack-cli/internal/meta"
	pluginpkg "github.com/whiskeyjimb/tack-cli/internal/plugin"
)

// NewRootCommand creates the top-level CLI command with dynamic plugin loading.
func NewRootCommand(cfg *config.Config, stack *pluginpkg.PluginStack, configPath string) *cobra.Command {
	var (
		outputFormat string
		verbose      bool
		quiet        bool
		trustPlugins bool
	)

	root := &cobra.Command{
		Use:   meta.AppName,
		Short: "One CLI, many plugins",
		Long: fmt.Sprintf(`%s is a single CLI that grows with plugins. Each plugin brings its own
commands, flags, and completions. Output is consistent across all of them.
Plugins are sandboxed WASM modules, installable from OCI registries.`, "Tack"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Flags with defaults from config
	root.PersistentFlags().StringVar(&outputFormat, "output", cfg.Output, "Output format: table, json, yaml")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging from plugins")
	root.PersistentFlags().BoolVar(&quiet, "quiet", cfg.Quiet, "Suppress output; exit code indicates result")
	root.PersistentFlags().BoolVar(&trustPlugins, "trust-plugins", false, "Trust all plugins and automatically grant requested capabilities")

	// When quiet mode is enabled, override output format
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if quiet {
			outputFormat = "quiet"
		}
	}

	// Static commands
	root.AddCommand(newCompletionCommand())
	root.AddCommand(newVersionCommand())

	// Plugin management (uses host-sdk PluginService)
	if stack != nil {
		root.AddCommand(newPluginCommand(stack, cfg))
	}

	// Group management
	root.AddCommand(newGroupCommand(cfg, configPath))

	// Register flag completions
	registerOutputFormatCompletion(root)

	// Register aliases from config
	if len(cfg.Aliases) > 0 {
		registerAliases(root, cfg.Aliases)
	}

	return root
}

// RegisterPluginCommands discovers plugins and adds their commands to root.
// This is called from main.go after flag parsing.
func RegisterPluginCommands(root *cobra.Command, outputFormat *string, verbose *bool, trustPlugins *bool, cfg *config.Config, stack *pluginpkg.PluginStack) error {
	ctx := context.Background()

	loader := pluginpkg.NewLoader(
		pluginpkg.EmbeddedPlugins,
		pluginpkg.DefaultPluginsDir(),
		stack,
		cfg.DefaultRegistry,
	)
	discovered, err := loader.DiscoverAll(ctx)
	if err != nil {
		// Don't fail the CLI if plugin discovery fails
		fmt.Fprintf(os.Stderr, "Warning: plugin discovery failed: %v\n", err)
		return nil
	}

	// Helper to generate a plugin command for a given DiscoveredPlugin.
	makePluginCmd := func(dp pluginpkg.DiscoveredPlugin) *cobra.Command {
		var defaults map[string]string
		if cfg != nil && cfg.PluginDefaults != nil {
			defaults = cfg.PluginDefaults[dp.Manifest.Name]
		}
		return generatePluginCommand(dp.Manifest, dp.Loader, outputFormat, verbose, trustPlugins, defaults)
	}

	// Ensure "top" group exists with all plugins by default
	if cfg.Groups == nil {
		cfg.Groups = make(map[string]config.GroupConfig)
	}
	if _, exists := cfg.Groups["top"]; !exists {
		// Create default "top" group with all discovered plugins
		var allPluginNames []string
		for _, dp := range discovered {
			if dp.Manifest.Name != "" {
				allPluginNames = append(allPluginNames, dp.Manifest.Name)
			}
		}
		cfg.Groups["top"] = config.GroupConfig{
			Description: "Top-level plugins",
			Plugins:     allPluginNames,
		}
	}

	// Register groups (including the special "top" group)
	topGroupPlugins := registerGroups(root, cfg.Groups, discovered, makePluginCmd)

	// Register plugins at the top level if they're in the "top" group
	for _, dp := range discovered {
		if topGroupPlugins[dp.Manifest.Name] {
			pluginCmd := makePluginCmd(dp)
			root.AddCommand(pluginCmd)
		}
	}

	return nil
}
