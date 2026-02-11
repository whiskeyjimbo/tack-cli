package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/whiskeyjimb/tack-cli/internal/config"
)

// newGroupCommand creates the "group" management command.
func newGroupCommand(cfg *config.Config, configPath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage plugin groups",
	}

	cmd.AddCommand(
		newGroupListCommand(cfg),
		newGroupCreateCommand(cfg, configPath),
		newGroupDeleteCommand(cfg, configPath),
		newGroupAddCommand(cfg, configPath),
		newGroupRemoveCommand(cfg, configPath),
	)

	return cmd
}

// newGroupListCommand creates the "group list" command.
func newGroupListCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all plugin groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if len(cfg.Groups) == 0 {
				_, _ = fmt.Fprintln(out, "No groups configured.")
				return nil
			}

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "GROUP\tDESCRIPTION\tPLUGINS")
			for name, group := range cfg.Groups {
				plugins := strings.Join(group.Plugins, ", ")
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", name, group.Description, plugins)
			}
			return w.Flush()
		},
	}
}

// newGroupCreateCommand creates the "group create" command.
func newGroupCreateCommand(cfg *config.Config, configPath string) *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new plugin group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if cfg.Groups == nil {
				cfg.Groups = make(map[string]config.GroupConfig)
			}

			if _, exists := cfg.Groups[name]; exists {
				return fmt.Errorf("group %q already exists", name)
			}

			// Check for reserved names (from config package)
			if name == "" {
				return fmt.Errorf("group name cannot be empty")
			}
			reservedCommands := map[string]bool{
				"completion": true,
				"version":    true,
				"plugin":     true,
				"group":      true,
				"help":       true,
			}
			if reservedCommands[name] {
				return fmt.Errorf("group name %q conflicts with built-in command", name)
			}

			cfg.Groups[name] = config.GroupConfig{
				Description: description,
				Plugins:     []string{},
			}

			if err := cfg.Save(configPath); err != nil {
				delete(cfg.Groups, name)
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created group %q\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Description for the group")
	return cmd
}

// newGroupDeleteCommand creates the "group delete" command.
func newGroupDeleteCommand(cfg *config.Config, configPath string) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a plugin group",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Prevent deletion of the special "top" group
			if name == "top" {
				return fmt.Errorf("cannot delete the 'top' group - it controls which plugins appear at the root level")
			}

			if _, exists := cfg.Groups[name]; !exists {
				return fmt.Errorf("group %q not found", name)
			}

			old := cfg.Groups[name]
			delete(cfg.Groups, name)

			if err := cfg.Save(configPath); err != nil {
				cfg.Groups[name] = old
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted group %q\n", name)
			return nil
		},
	}
}

// newGroupAddCommand creates the "group add" command.
func newGroupAddCommand(cfg *config.Config, configPath string) *cobra.Command {
	return &cobra.Command{
		Use:   "add <group> <plugin>...",
		Short: "Add plugins to a group",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupName := args[0]
			pluginNames := args[1:]

			group, exists := cfg.Groups[groupName]
			if !exists {
				return fmt.Errorf("group %q not found", groupName)
			}

			// Build set of existing plugins for dedup
			existing := make(map[string]bool)
			for _, p := range group.Plugins {
				existing[p] = true
			}

			var added []string
			for _, p := range pluginNames {
				if existing[p] {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %q is already in group %q, skipping\n", p, groupName)
					continue
				}
				group.Plugins = append(group.Plugins, p)
				existing[p] = true
				added = append(added, p)
			}

			if len(added) == 0 {
				return nil
			}

			cfg.Groups[groupName] = group

			if err := cfg.Save(configPath); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Added %s to group %q\n", strings.Join(added, ", "), groupName)
			return nil
		},
	}
}

// newGroupRemoveCommand creates the "group remove" command.
func newGroupRemoveCommand(cfg *config.Config, configPath string) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <group> <plugin>...",
		Short: "Remove plugins from a group",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupName := args[0]
			pluginNames := args[1:]

			group, exists := cfg.Groups[groupName]
			if !exists {
				return fmt.Errorf("group %q not found", groupName)
			}

			toRemove := make(map[string]bool)
			for _, p := range pluginNames {
				toRemove[p] = true
			}

			// Check all requested plugins exist in the group
			existing := make(map[string]bool)
			for _, p := range group.Plugins {
				existing[p] = true
			}
			for _, p := range pluginNames {
				if !existing[p] {
					return fmt.Errorf("plugin %q is not in group %q", p, groupName)
				}
			}

			// If removing from "top" group, ensure plugins exist in at least one other group
			if groupName == "top" {
				for _, pluginName := range pluginNames {
					foundInOtherGroup := false
					for otherGroupName, otherGroup := range cfg.Groups {
						if otherGroupName == "top" {
							continue
						}
						for _, p := range otherGroup.Plugins {
							if p == pluginName {
								foundInOtherGroup = true
								break
							}
						}
						if foundInOtherGroup {
							break
						}
					}
					if !foundInOtherGroup {
						return fmt.Errorf("cannot remove %q from 'top' group: it is not in any other group and would become inaccessible", pluginName)
					}
				}
			}

			// Filter out removed plugins
			var remaining []string
			for _, p := range group.Plugins {
				if !toRemove[p] {
					remaining = append(remaining, p)
				}
			}
			group.Plugins = remaining
			cfg.Groups[groupName] = group

			if err := cfg.Save(configPath); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from group %q\n", strings.Join(pluginNames, ", "), groupName)
			return nil
		},
	}
}
