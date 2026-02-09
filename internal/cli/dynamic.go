package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/reglet-dev/cli/internal/output"
	"github.com/reglet-dev/cli/internal/runtime"
	abi "github.com/reglet-dev/reglet-abi"
	"github.com/spf13/cobra"
)

// generatePluginCommand creates a cobra command tree from a plugin manifest.
//
// Single-service plugins (1 service): operations become direct subcommands.
//
//	cli dns resolve
//
// Multi-service plugins (2+ services): services become subcommands, operations nested.
//
//	cli aws iam get_account_summary
//	cli aws ec2 describe_security_groups
func generatePluginCommand(manifest abi.Manifest, wasmLoader func() ([]byte, error), outputFormat *string, verbose *bool, defaults map[string]string) *cobra.Command {
	pluginCmd := &cobra.Command{
		Use:   manifest.Name,
		Short: manifest.Description,
	}

	schema, err := parseConfigSchema(manifest.ConfigSchema)
	if err != nil {
		// If schema parsing fails, create a command that reports the error
		pluginCmd.RunE = func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("failed to parse plugin config schema: %w", err)
		}
		return pluginCmd
	}

	if len(manifest.Services) == 1 {
		// Single-service: operations are direct subcommands of plugin
		for _, svc := range manifest.Services {
			for _, op := range svc.Operations {
				pluginCmd.AddCommand(
					createOperationCommand(manifest.Name, svc.Name, op, schema, wasmLoader, outputFormat, verbose, defaults),
				)
			}
		}
	} else {
		// Multi-service: services are subcommands, operations nested under them
		for svcName, svc := range manifest.Services {
			svcCmd := &cobra.Command{
				Use:   svcName,
				Short: svc.Description,
			}
			for _, op := range svc.Operations {
				svcCmd.AddCommand(
					createOperationCommand(manifest.Name, svcName, op, schema, wasmLoader, outputFormat, verbose, defaults),
				)
			}
			pluginCmd.AddCommand(svcCmd)
		}
	}

	return pluginCmd
}

// createOperationCommand creates a cobra command for a single operation.
func createOperationCommand(
	pluginName, serviceName string,
	op abi.OperationManifest,
	schema *parsedSchema,
	wasmLoader func() ([]byte, error),
	outputFormat *string,
	verbose *bool,
	defaults map[string]string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   op.Name,
		Short: op.Description,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load WASM plugin
			wasmBytes, err := wasmLoader()
			if err != nil {
				return fmt.Errorf("loading plugin: %w", err)
			}

			// Create runtime
			runner, err := runtime.NewPluginRunner(ctx, runtime.WithVerbose(*verbose))
			if err != nil {
				return fmt.Errorf("creating runtime: %w", err)
			}
			defer runner.Close(ctx)

			plugin, err := runner.LoadPlugin(ctx, wasmBytes)
			if err != nil {
				return fmt.Errorf("loading plugin: %w", err)
			}

			// Build config from flags
			config := buildConfigFromFlags(cmd, serviceName, op.Name)

			// Execute
			result, err := plugin.Check(ctx, config)
			if err != nil {
				return fmt.Errorf("executing operation: %w", err)
			}

			// Report errors from result
			if result.IsError() && result.Error != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error.Message)
				if result.Error.Type != "" {
					fmt.Fprintf(os.Stderr, "  Type: %s\n", result.Error.Type)
				}
				if result.Error.Code != "" {
					fmt.Fprintf(os.Stderr, "  Code: %s\n", result.Error.Code)
				}
				os.Exit(1)
			}

			// Format output
			formatter, err := output.NewFormatter(*outputFormat)
			if err != nil {
				return err
			}

			if err := formatter.Format(os.Stdout, result, op.OutputSchema); err != nil {
				return fmt.Errorf("formatting output: %w", err)
			}

			// Set exit code for non-success results (Failed results)
			if !result.IsSuccess() {
				os.Exit(1)
			}

			return nil
		},
	}

	// Add operation-specific flags from schema
	addFlagsForOperation(cmd, schema, op.InputFields, defaults)

	// Add examples to help text
	if len(op.Examples) > 0 {
		cmd.Example = formatExamplesForHelp(pluginName, serviceName, op)
	}

	return cmd
}

// formatExamplesForHelp converts operation examples to CLI help text.
//
// For single-service plugins, the command format is:
//
//	cli <plugin> <operation> <flags>
//
// For multi-service plugins, the command format is:
//
//	cli <plugin> <service> <operation> <flags>
//
// Error examples (those with ExpectedError set) are skipped.
func formatExamplesForHelp(pluginName, serviceName string, op abi.OperationManifest) string {
	var sb strings.Builder

	for _, ex := range op.Examples {
		// Skip error examples in help text
		if ex.ExpectedError != "" {
			continue
		}

		if ex.Description != "" {
			sb.WriteString(fmt.Sprintf("  # %s\n", ex.Description))
		}

		flags := inputJSONToFlags(ex.Input)
		sb.WriteString(fmt.Sprintf("  cli %s %s %s\n\n", pluginName, op.Name, flags))
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// inputJSONToFlags converts a JSON input object to CLI flag strings.
//
// Example:
//
//	{"hostname": "example.com", "record_type": "A"}
//	-> `--hostname "example.com" --record-type "A"`
func inputJSONToFlags(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	var fields map[string]any
	if err := json.Unmarshal(input, &fields); err != nil {
		return ""
	}

	var parts []string
	for key, val := range fields {
		flagName := strings.ReplaceAll(key, "_", "-")
		parts = append(parts, fmt.Sprintf("--%s %q", flagName, fmt.Sprintf("%v", val)))
	}

	return strings.Join(parts, " ")
}
