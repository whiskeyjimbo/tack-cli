package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	internalplugin "github.com/reglet-dev/cli/internal/plugin"
	hostentities "github.com/reglet-dev/reglet-host-sdk/plugin/entities"
	hostvalues "github.com/reglet-dev/reglet-host-sdk/plugin/values"
	"github.com/spf13/cobra"
)

// newPluginCommand creates the "plugin" management command group.
func newPluginCommand(stack *internalplugin.PluginStack, defaultRegistry string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
	}

	cmd.AddCommand(
		newPluginListCommand(stack),
		newPluginInstallCommand(stack, defaultRegistry),
		newPluginRemoveCommand(stack),
		newPluginPruneCommand(stack),
		newPluginRefreshCommand(stack),
	)

	return cmd
}

// newPluginListCommand creates the "plugin list" command.
func newPluginListCommand(stack *internalplugin.PluginStack) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins, err := stack.Service.ListCachedPlugins(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(plugins) == 0 {
				fmt.Fprintln(out, "No plugins installed in local cache.")
				return nil
			}

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tDIGEST\tDESCRIPTION")
			for _, p := range plugins {
				meta := p.Metadata()
				digest := p.Digest().String()
				// Truncate digest for display
				if len(digest) > 19 {
					digest = digest[:19] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					meta.Name(), meta.Version(), digest, meta.Description())
			}
			return w.Flush()
		},
	}
}

// newPluginInstallCommand creates the "plugin install" command.
func newPluginInstallCommand(stack *internalplugin.PluginStack, defaultRegistry string) *cobra.Command {
	return &cobra.Command{
		Use:   "install <reference>",
		Short: "Install a plugin from an OCI registry or local file",
		Long: `Install a plugin from an OCI registry or a local .wasm file.

Examples:
  cli plugin install dns                                        # Install latest from default registry
  cli plugin install dns@1.2.0                                  # Install specific version
  cli plugin install ghcr.io/my-org/plugins/custom:1.0.0        # Install from custom registry
  cli plugin install ./custom.wasm                              # Install from local file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			out := cmd.OutOrStdout()
			ctx := cmd.Context()

			// Determine if target is a local file or OCI reference
			isLocal := strings.HasSuffix(target, ".wasm") ||
				strings.HasPrefix(target, "./") ||
				strings.HasPrefix(target, "../") ||
				filepath.IsAbs(target)

			if isLocal {
				return installFromLocalFile(ctx, stack, target, out)
			}

			// Build full OCI reference from short name or full reference
			ref := resolveOCIRef(target, defaultRegistry)

			fmt.Fprintf(out, "Pulling %s ...\n", ref)

			pluginRef, err := hostvalues.ParsePluginReference(ref)
			if err != nil {
				return fmt.Errorf("invalid plugin reference %q: %w", ref, err)
			}

			// Pull via OCI \u2014 this resolves, downloads, verifies, and caches
			artifact, err := stack.Service.Pull(ctx, pluginRef)
			if err != nil {
				return fmt.Errorf("pulling plugin: %w", err)
			}

			meta := artifact.Metadata()
			fmt.Fprintf(out, "Installed %s@%s\n", meta.Name(), meta.Version())

			return nil
		},
	}
}

// installFromLocalFile installs a .wasm file into the local cache.
func installFromLocalFile(ctx context.Context, stack *internalplugin.PluginStack, path string, out io.Writer) error {
	fmt.Fprintf(out, "Installing from local file: %s\n", path)

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening plugin file: %w", err)
	}
	defer f.Close()

	// Extract name from filename
	name := strings.TrimSuffix(filepath.Base(path), ".wasm")

	// Create a minimal plugin entity for local files
	ref, err := hostvalues.ParsePluginReference(name)
	if err != nil {
		return fmt.Errorf("invalid plugin name %q: %w", name, err)
	}

	digest, err := hostvalues.ComputeDigestSHA256(f)
	if err != nil {
		return fmt.Errorf("computing digest: %w", err)
	}

	// Re-open for storage
	f.Seek(0, 0)

	metadata := hostvalues.NewPluginMetadata(name, "local", "", nil)
	plugin := hostentities.NewPlugin(ref, digest, metadata)

	storedPath, err := stack.Repository.Store(ctx, plugin, f)
	if err != nil {
		return fmt.Errorf("storing plugin: %w", err)
	}

	fmt.Fprintf(out, "Installed %q to %s\n", name, storedPath)
	return nil
}

// newPluginRemoveCommand creates the "plugin remove" command.
func newPluginRemoveCommand(stack *internalplugin.PluginStack) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <reference>",
		Aliases: []string{"uninstall", "rm"},
		Short:   "Remove a plugin from local cache",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := hostvalues.ParsePluginReference(args[0])
			if err != nil {
				return fmt.Errorf("invalid plugin reference: %w", err)
			}

			if err := stack.Repository.Delete(cmd.Context(), ref); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed plugin %q\n", args[0])
			return nil
		},
	}
}

// newPluginPruneCommand creates the "plugin prune" command.
func newPluginPruneCommand(stack *internalplugin.PluginStack) *cobra.Command {
	var keepVersions int

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove old plugin versions from cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := stack.Service.PruneCache(cmd.Context(), keepVersions); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pruned plugin cache (keeping %d versions per plugin)\n", keepVersions)
			return nil
		},
	}

	cmd.Flags().IntVar(&keepVersions, "keep", 3, "Number of versions to keep per plugin")
	return cmd
}

// newPluginRefreshCommand creates the "plugin refresh" command.
func newPluginRefreshCommand(stack *internalplugin.PluginStack) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Rebuild the plugin discovery cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Refreshing plugin cache...")

			// Delete manifest discovery cache to force full rebuild
			cachePath := internalplugin.DefaultCachePath()
			_ = os.Remove(cachePath)

			// Re-run discovery (caller must reinvoke with new loader)
			fmt.Fprintln(out, "Discovery cache cleared. Restart the CLI to rebuild.")
			return nil
		},
	}
}

// resolveOCIRef builds a full OCI reference from a short name or full reference.
func resolveOCIRef(target, defaultRegistry string) string {
	if strings.Contains(target, "/") {
		return target
	}

	name, version := parseNameVersion(target)
	if version == "" {
		version = "latest"
	}

	return fmt.Sprintf("%s/%s:%s", defaultRegistry, name, version)
}

// parseNameVersion splits "aws@1.2.0" into ("aws", "1.2.0").
func parseNameVersion(s string) (name, version string) {
	if idx := strings.Index(s, "@"); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

// formatSize formats bytes into a human-readable string.
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
	)
	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
