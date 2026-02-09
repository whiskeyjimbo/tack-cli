package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	internalcli "github.com/whiskeyjimb/tack-cli/internal/cli"
	"github.com/whiskeyjimb/tack-cli/internal/config"
	"github.com/whiskeyjimb/tack-cli/internal/meta"
	"github.com/whiskeyjimb/tack-cli/internal/plugin"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Load config
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: config error: %v\n", err)
		cfg = config.DefaultConfig()
	}
	cfg.ApplyEnvOverrides()

	// Initialize plugin service stack
	stack, err := plugin.NewPluginStack(plugin.PluginServiceConfig{
		RequireSigning: cfg.RequireSigning,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize plugin service: %v\n", err)
		// We continue without the stack (OCI fallback will be disabled)
	}

	root := internalcli.NewRootCommand(cfg, stack)

	// Discover and register plugin commands
	outputFormat := cfg.Output
	verbose := false
	trustPlugins := false
	// Find --output, --verbose, and --trust-plugins in args (simple scan before cobra parsing)
	for i, arg := range os.Args {
		if arg == "--output" && i+1 < len(os.Args) {
			outputFormat = os.Args[i+1]
		}
		if arg == "--verbose" || arg == "-v" {
			verbose = true
		}
		if arg == "--trust-plugins" {
			trustPlugins = true
		}
	}
	_ = internalcli.RegisterPluginCommands(root, &outputFormat, &verbose, &trustPlugins, cfg, stack)

	if err := root.ExecuteContext(ctx); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "unknown command") {
			parts := strings.Split(msg, "\"")
			if len(parts) >= 2 {
				unknownCmd := parts[1]
				fmt.Fprintf(os.Stderr, "Error: plugin %q not found\n", unknownCmd)

				// Find installed plugins
				var installed []string
				for _, cmd := range root.Commands() {
					if cmd.Name() != "completion" && cmd.Name() != "help" && cmd.Name() != "plugin" && cmd.Name() != "version" {
						installed = append(installed, cmd.Name())
					}
				}
				if len(installed) > 0 {
					fmt.Fprintf(os.Stderr, "  Installed plugins: %s\n", strings.Join(installed, ", "))
				}
				fmt.Fprintf(os.Stderr, "  To install: %s plugin install %s\n", meta.AppName, unknownCmd)
				os.Exit(1)
			}
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
