// Package cli implements the command-line interface for Reglet.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/reglet-dev/cli/internal/config"
	pluginpkg "github.com/reglet-dev/cli/internal/plugin"
	"github.com/spf13/cobra"
)

// NewRootCommand creates the top-level CLI command with dynamic plugin loading.
func NewRootCommand(cfg *config.Config, stack *pluginpkg.PluginStack) *cobra.Command {
	var (
		outputFormat string
		verbose      bool
		quiet        bool
	)

	root := &cobra.Command{
		Use:   "cli",
		Short: "Infrastructure inspection tool powered by WASM plugins",
		Long: `CLI is a general-purpose infrastructure inspection tool that leverages
WASM plugins to provide a unified interface for querying cloud resources,
network services, and system state.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Flags with defaults from config
	root.PersistentFlags().StringVar(&outputFormat, "output", cfg.Output, "Output format: table, json, yaml")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging from plugins")
	root.PersistentFlags().BoolVar(&quiet, "quiet", cfg.Quiet, "Suppress output; exit code indicates result")

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
		root.AddCommand(newPluginCommand(stack, cfg.DefaultRegistry))
	}

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
func RegisterPluginCommands(root *cobra.Command, outputFormat *string, verbose *bool, cfg *config.Config, stack *pluginpkg.PluginStack) error {
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

	for _, dp := range discovered {
		dp := dp // capture for closure
		var defaults map[string]string
		if cfg != nil && cfg.PluginDefaults != nil {
			defaults = cfg.PluginDefaults[dp.Manifest.Name]
		}

		pluginCmd := generatePluginCommand(dp.Manifest, dp.Loader, outputFormat, verbose, defaults)
		root.AddCommand(pluginCmd)
	}

	return nil
}
