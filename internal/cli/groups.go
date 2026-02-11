package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/whiskeyjimb/tack-cli/internal/config"
	pluginpkg "github.com/whiskeyjimb/tack-cli/internal/plugin"
)

// registerGroups creates group commands and nests plugin commands under them.
// Returns the set of plugin names that are in the "top" group (for root-level registration).
// The "top" group is special - its plugins appear at root level, not under a "top" command.
func registerGroups(
	root *cobra.Command,
	groups map[string]config.GroupConfig,
	discovered []pluginpkg.DiscoveredPlugin,
	generateFn func(pluginpkg.DiscoveredPlugin) *cobra.Command,
) map[string]bool {
	// Build lookup: plugin name -> DiscoveredPlugin
	pluginMap := make(map[string]pluginpkg.DiscoveredPlugin)
	for _, dp := range discovered {
		pluginMap[dp.Manifest.Name] = dp
	}

	// Track which plugins are in the "top" group
	topGroupPlugins := make(map[string]bool)

	for groupName, groupCfg := range groups {
		// Handle "top" group specially - just track its plugins, don't create a command
		if groupName == "top" {
			for _, pluginName := range groupCfg.Plugins {
				if _, ok := pluginMap[pluginName]; ok {
					topGroupPlugins[pluginName] = true
				}
			}
			continue
		}

		// Regular group - create a command for it
		groupCmd := &cobra.Command{
			Use:   groupName,
			Short: groupCfg.Description,
		}

		var pluginNames []string
		for _, pluginName := range groupCfg.Plugins {
			dp, ok := pluginMap[pluginName]
			if !ok {
				fmt.Fprintf(os.Stderr, "Warning: group %q references plugin %q which is not installed\n", groupName, pluginName)
				continue
			}

			pluginCmd := generateFn(dp)
			groupCmd.AddCommand(pluginCmd)
			pluginNames = append(pluginNames, pluginName)
		}

		// Only add the group if it has at least one valid plugin
		if len(pluginNames) > 0 {
			groupCmd.Long = fmt.Sprintf("%s\n\nPlugins: %s", groupCfg.Description, strings.Join(pluginNames, ", "))
			root.AddCommand(groupCmd)
		}
	}

	return topGroupPlugins
}
