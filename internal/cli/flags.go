package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// schemaProperty represents a single property from a JSON Schema.
type schemaProperty struct {
	Type        string `json:"type"`
	Enum        []any  `json:"enum"`
	Default     any    `json:"default"`
	Description string `json:"description"`
}

// parsedSchema holds the parsed config schema.
type parsedSchema struct {
	Properties map[string]schemaProperty `json:"properties"`
	Required   []string                  `json:"required"`
}

// parseConfigSchema parses a JSON Schema from raw bytes.
func parseConfigSchema(raw json.RawMessage) (*parsedSchema, error) {
	if len(raw) == 0 {
		return &parsedSchema{}, nil
	}
	var s parsedSchema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parsing config schema: %w", err)
	}
	return &s, nil
}

// addFlagsForOperation adds cobra flags based on the config schema,
// filtered to only the fields listed in inputFields.
//
// Flag naming: snake_case JSON fields become kebab-case flags.
// Example: "record_type" -> "--record-type"
//
// Fields named "service" or "operation" are skipped because they are
// determined by the command path (e.g., "cli aws ec2 describe_security_groups"
// implies service=ec2 and operation=describe_security_groups).
//
// defaults: optional map of flag defaults (e.g., from config file).
func addFlagsForOperation(cmd *cobra.Command, schema *parsedSchema, inputFields []string, defaults map[string]string) {
	if schema == nil {
		return
	}

	// Build lookup set for input fields
	inputSet := make(map[string]bool, len(inputFields))
	for _, f := range inputFields {
		inputSet[f] = true
	}

	// Track which fields are required
	requiredSet := make(map[string]bool, len(schema.Required))
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	for name, prop := range schema.Properties {
		// Skip service/operation - determined by command path
		if name == "service" || name == "operation" {
			continue
		}

		// Skip fields not in this operation's input_fields (if input_fields is specified)
		if len(inputFields) > 0 && !inputSet[name] {
			continue
		}

		// Convert snake_case to kebab-case for CLI flags
		flagName := strings.ReplaceAll(name, "_", "-")

		// Check for user-defined default
		userDefault, hasUserDefault := defaults[flagName]

		switch prop.Type {
		case "string":
			defaultVal := ""
			if hasUserDefault {
				defaultVal = userDefault
			} else if prop.Default != nil {
				defaultVal = fmt.Sprintf("%v", prop.Default)
			}
			cmd.Flags().String(flagName, defaultVal, prop.Description)

			// Register completion for enum values
			if len(prop.Enum) > 0 {
				enumStrs := make([]string, len(prop.Enum))
				for i, e := range prop.Enum {
					enumStrs[i] = fmt.Sprintf("%v", e)
				}
				_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
					return enumStrs, cobra.ShellCompDirectiveNoFileComp
				})
			}

		case "integer":
			defaultVal := 0
			if hasUserDefault {
				fmt.Sscanf(userDefault, "%d", &defaultVal)
			} else if prop.Default != nil {
				if f, ok := prop.Default.(float64); ok {
					defaultVal = int(f)
				}
			}
			cmd.Flags().Int(flagName, defaultVal, prop.Description)

		case "boolean":
			defaultVal := false
			if hasUserDefault {
				defaultVal = strings.ToLower(userDefault) == "true"
			} else if prop.Default != nil {
				if b, ok := prop.Default.(bool); ok {
					defaultVal = b
				}
			}
			cmd.Flags().Bool(flagName, defaultVal, prop.Description)

		case "array":
			// Note: User defaults for arrays/objects not currently supported via config map[string]string
			cmd.Flags().StringSlice(flagName, nil, prop.Description)

		case "object":
			cmd.Flags().StringToString(flagName, nil, prop.Description)
		}

		// Mark required flags
		if requiredSet[name] {
			_ = cmd.MarkFlagRequired(flagName)
		}
	}
}

// buildConfigFromFlags constructs the plugin config map from cobra flags.
// It sets "service" and "operation" from the command path, then adds all
// user-provided flag values (converting kebab-case back to snake_case).
func buildConfigFromFlags(cmd *cobra.Command, serviceName, operationName string) map[string]any {
	config := map[string]any{
		"service":   serviceName,
		"operation": operationName,
	}

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed {
			return
		}

		// Convert kebab-case flag name back to snake_case for JSON
		jsonName := strings.ReplaceAll(f.Name, "-", "_")

		// Skip internal flags
		if jsonName == "output" || jsonName == "plugin_path" || jsonName == "quiet" {
			return
		}

		switch f.Value.Type() {
		case "string":
			val, _ := cmd.Flags().GetString(f.Name)
			config[jsonName] = val
		case "int":
			val, _ := cmd.Flags().GetInt(f.Name)
			config[jsonName] = val
		case "bool":
			val, _ := cmd.Flags().GetBool(f.Name)
			config[jsonName] = val
		case "stringSlice":
			val, _ := cmd.Flags().GetStringSlice(f.Name)
			config[jsonName] = val
		case "stringToString":
			val, _ := cmd.Flags().GetStringToString(f.Name)
			config[jsonName] = val
		}
	})

	return config
}
