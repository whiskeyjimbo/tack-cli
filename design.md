# CLI - Design Document

## Overview

**CLI** is a general-purpose infrastructure inspection tool that leverages WASM plugins to provide a unified interface for querying cloud resources, network services, and system state.

It reuses the same plugin ecosystem as Reglet but exposes operations as interactive CLI commands rather than compliance checks.

## Goals

1. **Unified interface**: One CLI for AWS, DNS, HTTP, certs, etc.
2. **Schema-driven**: Auto-generate subcommands, flags, and completions from plugin schemas
3. **Plugin ecosystem**: Install plugins from registry, same WASM format as Reglet
4. **Developer-friendly**: Excellent tab completion, helpful error messages, multiple output formats
5. **Scriptable**: JSON output mode for piping to jq, scripts, etc.

## Non-Goals

- Compliance checking (that's Reglet's job)
- Write operations / remediation (read-only inspection)
- State management or caching

## Plugin Design Philosophy

Plugins are **pure data fetchers**. They execute a single operation and return structured data. Validation and compliance checking is done externally:

- **CLI**: Displays data to users in table/JSON/YAML format
- **Reglet**: Validates data against `expect` expressions using expr-lang

This means plugins don't need separate "validate_X" operations. A single `resolve` operation in the DNS plugin serves both:
- `cli dns resolve --hostname example.com` → displays records
- Reglet profile with `expect: ['"1.2.3.4" in data.records']` → validates records

---

## Usage Examples

### Basic Operations

```bash
# AWS operations
cli aws iam get_account_summary
cli aws ec2 describe_security_groups --region us-east-1
cli aws s3 list_buckets --output json

# DNS lookups
cli dns resolve --hostname api.example.com
cli dns resolve --hostname example.com --record-type MX

# HTTP requests
cli http request --url https://api.example.com/health
cli http request --url https://example.com --method POST --body '{"key":"value"}'

# TCP connectivity
cli tcp connect --host db.internal --port 5432 --timeout-ms 5000

# Command execution
cli command execute --run "systemctl is-active nginx"
```

### Plugin Management

```bash
# List installed plugins
cli plugin list

# Install from registry
cli plugin install aws
cli plugin install aws@1.2.0

# Install from local file
cli plugin install ./my-custom-plugin.wasm

# Update plugins
cli plugin update aws
cli plugin update --all

# Remove plugins
cli plugin remove my-custom-plugin

# Search registry
cli plugin search cloud
```

### Output Formats

```bash
# Default: human-readable table
cli aws ec2 describe_security_groups

# JSON for scripting
cli aws ec2 describe_security_groups --output json

# YAML
cli aws ec2 describe_security_groups --output yaml

# Quiet mode (just status)
cli aws ec2 describe_security_groups --quiet
```

### Help with Examples

Operations include examples that appear in `--help` output:

```
$ cli dns resolve --help
Resolve hostname and return DNS records

Usage:
  cli dns resolve [flags]

Examples:
  # Resolve A record for a hostname
  cli dns resolve --hostname "example.com" --record-type "A"

  # Resolve MX records for email routing
  cli dns resolve --hostname "example.com" --record-type "MX"

Flags:
  -h, --help              help for resolve
      --hostname string   Target hostname to resolve (required)
      --nameserver string Custom nameserver to use
      --record-type string DNS record type to query (default "A")

Global Flags:
      --output string   Output format: table, json, yaml (default "table")
```

### Shell Completion

```bash
# Generate completion scripts
cli completion bash > /etc/bash_completion.d/cli
cli completion zsh > ~/.zsh/completions/_cli
cli completion fish > ~/.config/fish/completions/cli.fish

# Completions are dynamic based on installed plugins
cli <TAB>           # aws, dns, http, tcp, plugin, completion, help
cli aws <TAB>       # iam, ec2, s3, rds, vpc, lambda
cli aws ec2 <TAB>   # describe_security_groups, describe_instances_metadata
cli aws ec2 describe_security_groups --<TAB>  # --region, --filters, --output
```

---

## Architecture

```
cli/
├── cmd/cli/
│   └── main.go                 # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go             # Root command
│   │   ├── dynamic.go          # Dynamic command generation from manifests
│   │   │                       # - Uses input_fields for per-operation flags
│   │   │                       # - Uses examples for help text
│   │   └── completion.go       # Shell completion logic
│   ├── plugin/
│   │   ├── manager.go          # Plugin lifecycle (install, update, remove)
│   │   ├── registry.go         # Registry client
│   │   ├── loader.go           # WASM plugin loading
│   │   └── schema.go           # Schema parsing and validation
│   ├── runtime/
│   │   ├── executor.go         # Plugin execution
│   │   └── hostfuncs.go        # Host functions (reuse from reglet-host-sdk)
│   ├── output/
│   │   ├── table.go            # Human-readable tables (uses output_schema for columns)
│   │   ├── json.go             # JSON output
│   │   └── yaml.go             # YAML output
│   └── config/
│       └── config.go           # User configuration (~/.cli/config.yaml)
├── go.mod
└── go.sum
```

### Dependencies

```go
require (
    github.com/reglet-dev/reglet-host-sdk   // WASM runtime, host functions, capabilities
    github.com/reglet-dev/reglet-abi        // Shared ABI types (Manifest, Result, GrantSet)
    github.com/spf13/cobra                  // CLI framework
    github.com/charmbracelet/lipgloss       // Terminal styling
    github.com/olekukonko/tablewriter       // Table output
)
```

---

## Plugin Interface

### WASM Plugin Methods

Each plugin exposes two methods via the WASM interface:

```go
// Manifest returns plugin metadata, services, operations, and config schema
Manifest(ctx context.Context) (abi.Manifest, error)

// Check executes an operation with the given config
Check(ctx context.Context, config map[string]any) (abi.Result, error)
```

### Manifest Structure (Go Types)

```go
// Manifest is returned by plugin.Manifest()
// See: reglet-abi/manifest.go
type Manifest struct {
    Name           string                        `json:"name"`
    Version        string                        `json:"version"`
    Description    string                        `json:"description"`
    SDKVersion     string                        `json:"sdk_version"`
    MinHostVersion string                        `json:"min_host_version,omitempty"`
    Services       map[string]ServiceManifest    `json:"services"`
    ConfigSchema   json.RawMessage               `json:"config_schema"`   // JSON Schema
    Capabilities   hostfunc.GrantSet             `json:"capabilities"`
}

// ServiceManifest groups related operations
type ServiceManifest struct {
    Name        string              `json:"name"`
    Description string              `json:"description"`
    Operations  []OperationManifest `json:"operations"`
}

// OperationManifest is a single executable action
type OperationManifest struct {
    Name         string             `json:"name"`
    Description  string             `json:"description"`
    InputFields  []string           `json:"input_fields,omitempty"`  // Config fields used by this operation
    OutputSchema json.RawMessage    `json:"output_schema,omitempty"` // JSON Schema for output data
    Examples     []OperationExample `json:"examples,omitempty"`      // Usage examples (also used for tests)
}

// OperationExample documents a usage scenario
type OperationExample struct {
    Name           string          `json:"name"`
    Description    string          `json:"description,omitempty"`
    Input          json.RawMessage `json:"input"`
    ExpectedOutput json.RawMessage `json:"expected_output,omitempty"`
    ExpectedError  string          `json:"expected_error,omitempty"`
}

// GrantSet declares what permissions the plugin needs (replaces flat []Capability).
// See: reglet-abi/hostfunc/grant.go, reglet-abi/hostfunc/capability.go
type GrantSet struct {
    Network *NetworkCapability     `json:"network,omitempty"`   // dns_lookup, tcp_connect, http_request
    FS      *FileSystemCapability  `json:"fs,omitempty"`        // file read/write
    Env     *EnvironmentCapability `json:"env,omitempty"`       // environment variable access
    Exec    *ExecCapability        `json:"exec,omitempty"`      // exec_command
    KV      *KeyValueCapability    `json:"kv,omitempty"`        // key-value store access
}
```

### Result Structure (Go Types)

```go
// Result is returned by plugin.Check()
// See: reglet-abi/result.go
type Result struct {
    Timestamp time.Time              `json:"timestamp"`
    Status    ResultStatus           `json:"status"`   // "success", "failure", "error"
    Message   string                 `json:"message,omitempty"`
    Data      map[string]any         `json:"data,omitempty"`     // Operation-specific output
    Metadata  *RunMetadata           `json:"metadata,omitempty"` // Execution timing, SDK version
    Error     *hostfunc.ErrorDetail  `json:"error,omitempty"`    // Structured error with Type, Code, wrapped errors
}

type ResultStatus string

const (
    ResultStatusSuccess ResultStatus = "success"
    ResultStatusFailure ResultStatus = "failure"
    ResultStatusError   ResultStatus = "error"
)

// RunMetadata contains execution metadata (see reglet-abi/metadata.go)
// Fields: StartTime, EndTime, Duration, SDKVersion, PluginID

// ErrorDetail provides structured error information (see reglet-abi/hostfunc/error.go)
// Fields: Type, Code, Message, Wrapped, Details, Stack, IsTimeout, IsNotFound
// Error types: "network", "timeout", "config", "panic", "capability", "validation", "internal"
```

---

## Schema-to-CLI Mapping

Plugins expose their schema and operations via `Manifest()`. CLI parses this to generate commands dynamically.

### Plugin Manifest Examples

#### Multi-Service Plugin (AWS)

```json
{
  "name": "aws",
  "version": "1.0.0",
  "description": "AWS infrastructure inspection and compliance checks",
  "sdk_version": "0.1.0",
  "services": {
    "iam": {
      "name": "iam",
      "description": "IAM identity and access management checks",
      "operations": [
        {"name": "get_account_summary", "description": "Check if root account has MFA enabled"},
        {"name": "get_account_password_policy", "description": "Verify IAM password policy meets requirements"},
        {"name": "list_access_keys_with_usage", "description": "Find access keys unused for 90+ days"}
      ]
    },
    "ec2": {
      "name": "ec2",
      "description": "EC2 compute instance security checks",
      "operations": [
        {"name": "describe_security_groups", "description": "Find security groups with open SSH/RDP to 0.0.0.0/0"},
        {"name": "describe_instances_metadata", "description": "Verify IMDSv2 enforcement on EC2 instances"}
      ]
    }
  },
  "config_schema": {
    "type": "object",
    "properties": {
      "service": {
        "type": "string",
        "enum": ["iam", "ec2", "s3", "rds", "vpc", "lambda"],
        "description": "AWS service to query"
      },
      "operation": {
        "type": "string",
        "description": "Service operation to perform"
      },
      "region": {
        "type": "string",
        "description": "AWS region"
      },
      "timeout_seconds": {
        "type": "integer",
        "default": 30,
        "description": "Request timeout in seconds"
      },
      "filters": {
        "type": "object",
        "additionalProperties": {
          "type": "array",
          "items": {"type": "string"}
        },
        "description": "AWS API filters"
      }
    },
    "required": ["service", "operation"]
  },
  "capabilities": {
    "network": {
      "rules": [{"hosts": ["*"], "ports": ["443", "80"]}]
    }
  }
}
```

#### Single-Service Plugin (DNS)

```json
{
  "name": "dns",
  "version": "1.0.0",
  "description": "DNS resolution and record lookup",
  "sdk_version": "0.1.0",
  "services": {
    "dns": {
      "name": "dns",
      "description": "DNS resolution and record lookup",
      "operations": [
        {
          "name": "resolve",
          "description": "Resolve hostname and return DNS records",
          "input_fields": ["hostname", "record_type", "nameserver"],
          "output_schema": {
            "type": "object",
            "properties": {
              "hostname": {"type": "string", "description": "Queried hostname"},
              "record_type": {"type": "string", "description": "DNS record type queried"},
              "records": {"type": "array", "items": {"type": "string"}, "description": "Resolved DNS records"},
              "ttl": {"type": "integer", "description": "Record TTL in seconds"}
            }
          },
          "examples": [
            {
              "name": "basic_a",
              "description": "Resolve A record for a hostname",
              "input": {"hostname": "example.com", "record_type": "A"},
              "expected_output": {"hostname": "example.com", "record_type": "A", "records": ["93.184.216.34"]}
            },
            {
              "name": "mx_records",
              "description": "Resolve MX records for email routing",
              "input": {"hostname": "example.com", "record_type": "MX"}
            }
          ]
        }
      ]
    }
  },
  "config_schema": {
    "type": "object",
    "properties": {
      "hostname": {
        "type": "string",
        "description": "Hostname to resolve"
      },
      "record_type": {
        "type": "string",
        "enum": ["A", "AAAA", "MX", "TXT", "CNAME", "NS"],
        "default": "A",
        "description": "DNS record type to query"
      },
      "nameserver": {
        "type": "string",
        "description": "Custom nameserver to use"
      }
    },
    "required": ["hostname"]
  },
  "capabilities": {
    "network": {
      "rules": [{"hosts": ["*"], "ports": ["53"]}]
    }
  }
}
```

> **Note**: Validation (e.g., "A record matches expected IP") is now done via `expect` expressions in Reglet profiles using expr-lang, not separate plugin operations. Plugins are pure data fetchers.

---

## CLI Command Generation Algorithm

### Step 1: Load Plugin and Get Manifest

```go
func loadPlugin(wasmPath string) (*Manifest, error) {
    // 1. Load WASM module using wazero runtime
    // 2. Call plugin's Manifest() method
    // 3. Parse and return Manifest struct
}
```

### Step 2: Determine Command Structure

For each plugin, determine if it's multi-service or single-service:

```go
func generateCommands(manifest *Manifest) *cobra.Command {
    pluginCmd := &cobra.Command{
        Use:   manifest.Name,
        Short: manifest.Description,
    }

    // Check number of services
    if len(manifest.Services) == 1 {
        // Single-service: operations become direct subcommands
        // cli dns resolve
        // cli dns validate_a
        for _, svc := range manifest.Services {
            for _, op := range svc.Operations {
                pluginCmd.AddCommand(createOperationCommand(manifest, svc.Name, op))
            }
        }
    } else {
        // Multi-service: services become subcommands, operations nested under
        // cli aws iam get_account_summary
        // cli aws ec2 describe_security_groups
        for svcName, svc := range manifest.Services {
            svcCmd := &cobra.Command{
                Use:   svcName,
                Short: svc.Description,
            }
            for _, op := range svc.Operations {
                svcCmd.AddCommand(createOperationCommand(manifest, svcName, op))
            }
            pluginCmd.AddCommand(svcCmd)
        }
    }

    return pluginCmd
}
```

### Step 3: Generate Operation Commands with Flags

```go
func createOperationCommand(manifest *Manifest, serviceName string, op OperationManifest) *cobra.Command {
    cmd := &cobra.Command{
        Use:   op.Name,
        Short: op.Description,
        RunE: func(cmd *cobra.Command, args []string) error {
            // 1. Build config JSON from flags
            config := buildConfigFromFlags(cmd, serviceName, op.Name)

            // 2. Call plugin.Check(ctx, config)
            result, err := executePlugin(manifest.Name, config)
            if err != nil {
                return err
            }

            // 3. Format and print result (uses op.OutputSchema for table columns)
            return outputResult(cmd, result, op.OutputSchema)
        },
    }

    // Add flags only for this operation's input fields
    addFlagsForOperation(cmd, manifest.ConfigSchema, op.InputFields)

    // Add examples to help text
    if len(op.Examples) > 0 {
        cmd.Example = formatExamplesForHelp(manifest.Name, serviceName, op)
    }

    return cmd
}
```

### Step 4: Map JSON Schema to Cobra Flags (Per-Operation)

Each operation declares which config fields it uses via `input_fields`. This ensures operations only show relevant flags:

```go
// addFlagsForOperation adds flags for specific input fields only.
func addFlagsForOperation(cmd *cobra.Command, configSchema json.RawMessage, inputFields []string) {
    var schema struct {
        Properties map[string]struct {
            Type        string   `json:"type"`
            Enum        []any    `json:"enum"`
            Default     any      `json:"default"`
            Description string   `json:"description"`
        } `json:"properties"`
        Required []string `json:"required"`
    }
    if err := json.Unmarshal(configSchema, &schema); err != nil {
        return
    }
    props := schema.Properties
    required := schema.Required

    // Build set of input fields for quick lookup
    inputSet := make(map[string]bool)
    for _, f := range inputFields {
        inputSet[f] = true
    }

    for name, prop := range props {
        // Skip service/operation - determined by command path
        if name == "service" || name == "operation" {
            continue
        }

        // Skip fields not in this operation's input_fields
        if len(inputFields) > 0 && !inputSet[name] {
            continue
        }

        desc := prop.Description
        defaultVal := prop.Default

        // Convert snake_case to kebab-case for flags
        flagName := strings.ReplaceAll(name, "_", "-")

        switch prop.Type {
        case "string":
            if len(prop.Enum) > 0 {
                // Enum: add completion for valid values
                cmd.Flags().String(flagName, toString(defaultVal), desc)
                cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
                    return toStringSlice(prop.Enum), cobra.ShellCompDirectiveNoFileComp
                })
            } else {
                cmd.Flags().String(flagName, toString(defaultVal), desc)
            }

        case "integer":
            cmd.Flags().Int(flagName, toInt(defaultVal), desc)

        case "boolean":
            cmd.Flags().Bool(flagName, toBool(defaultVal), desc)

        case "array":
            cmd.Flags().StringSlice(flagName, nil, desc)

        case "object":
            cmd.Flags().StringToString(flagName, nil, desc)
        }

        // Mark required
        if contains(required, name) {
            cmd.MarkFlagRequired(flagName)
        }
    }
}

// formatExamplesForHelp converts operation examples to CLI help text.
func formatExamplesForHelp(pluginName, svcName string, op OperationManifest) string {
    var sb strings.Builder
    for _, ex := range op.Examples {
        if ex.ExpectedError != "" {
            continue // Skip error examples in help
        }
        if ex.Description != "" {
            sb.WriteString(fmt.Sprintf("  # %s\n", ex.Description))
        }
        flags := inputJSONToFlags(ex.Input)
        sb.WriteString(fmt.Sprintf("  cli %s %s %s\n\n", pluginName, op.Name, flags))
    }
    return strings.TrimSuffix(sb.String(), "\n")
}
```

### Step 5: Build Config JSON for Plugin Execution

```go
func buildConfigFromFlags(cmd *cobra.Command, serviceName, opName string) map[string]any {
    config := map[string]any{
        "service":   serviceName,
        "operation": opName,
    }

    // Add all flag values to config
    cmd.Flags().VisitAll(func(f *pflag.Flag) {
        if f.Changed {
            // Convert kebab-case back to snake_case for JSON
            jsonName := strings.ReplaceAll(f.Name, "-", "_")
            config[jsonName] = getFlagValue(f)
        }
    })

    return config
}
```

---

## Generated CLI Structure Examples

### Multi-Service Plugin (AWS)

```
cli aws                              # Plugin command
├── iam                              # Service subcommand
│   ├── get_account_summary          # Operation (no extra flags needed)
│   ├── get_account_password_policy
│   └── list_access_keys_with_usage
├── ec2                              # Service subcommand
│   ├── describe_security_groups     # Operation
│   │   └── [--region] [--filters]   # Flags from config_schema
│   └── describe_instances_metadata
│       └── [--region] [--filters]
└── [global: --timeout, --output, --quiet]
```

### Single-Service Plugin (DNS)

```
cli dns                              # Plugin command
└── resolve                          # Single data-fetching operation
    └── [--hostname] [--record-type] [--nameserver]
```

> Validation like "A record matches expected IP" is done via expr-lang in Reglet profiles, not CLI operations.

### Single-Service Plugin (HTTP)

```
cli http
└── request                          # Single operation, method is a flag
    └── [--url] [--method] [--headers] [--body] [--timeout] [--follow-redirects]
```

---

## Config Schema to Flags Mapping

| JSON Schema Type | Cobra Flag Type | Example |
|------------------|-----------------|---------|
| `"type": "string"` | `StringVar` | `--region us-east-1` |
| `"type": "string"` with `"enum"` | `StringVar` + completion | `--record-type A` (completes: A, AAAA, MX...) |
| `"type": "integer"` | `IntVar` | `--timeout 30` |
| `"type": "boolean"` | `BoolVar` | `--follow-redirects` |
| `"type": "array"` | `StringSliceVar` | `--expected-values 1.2.3.4 --expected-values 5.6.7.8` |
| `"type": "object"` | `StringToStringVar` | `--filters vpc-id=vpc-123` |
| `"required": ["field"]` | `MarkFlagRequired` | Error if not provided |
| `"default": value` | Flag default | Used if flag not specified |
| `"description": "..."` | Flag usage text | Shown in `--help` |

### Flag Naming Convention

- JSON field `timeout_seconds` → CLI flag `--timeout-seconds`
- JSON field `expected_values` → CLI flag `--expected-values`
- Convert `snake_case` to `kebab-case` for flags
- Convert back when building config JSON

---

## Plugin Execution Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ User runs: cli aws ec2 describe_security_groups --region us-east-1
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Cobra parses command path: plugin=aws, service=ec2,         │
│    operation=describe_security_groups                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Build config JSON from flags:                                │
│    {                                                            │
│      "service": "ec2",                                          │
│      "operation": "describe_security_groups",                   │
│      "region": "us-east-1"                                      │
│    }                                                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Load WASM plugin (aws.wasm)                                  │
│ 4. Call plugin.Check(ctx, config)                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. Plugin executes operation internally:                        │
│    - Parses config                                              │
│    - Looks up handler via GetHandler("ec2", "describe_security_groups")
│    - Executes handler                                           │
│    - Returns Result                                             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. CLI formats Result based on --output flag:                   │
│    - table (default): Human-readable table                      │
│    - json: Raw JSON output                                      │
│    - yaml: YAML output                                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## Plugin Management

### Directory Structure

```
~/.cli/
├── config.yaml           # User configuration
├── plugins/              # Installed plugins
│   ├── aws@1.2.0.wasm
│   ├── dns@1.0.0.wasm
│   └── http@1.1.0.wasm
└── cache/                # Registry cache
    └── registry.json
```

### Registry Protocol

```bash
# Registry endpoint
GET https://plugins.reglet.dev/v1/plugins           # List all
GET https://plugins.reglet.dev/v1/plugins/aws       # Plugin metadata
GET https://plugins.reglet.dev/v1/plugins/aws/1.2.0 # Specific version
GET https://plugins.reglet.dev/v1/plugins/aws/1.2.0/download  # WASM binary

# Response format
{
  "name": "aws",
  "versions": ["1.0.0", "1.1.0", "1.2.0"],
  "latest": "1.2.0",
  "description": "AWS infrastructure inspection",
  "checksum": "sha256:abc123..."
}
```

### Embedded Plugins

Core plugins are embedded in the binary for offline use:

```go
//go:embed plugins/*.wasm
var embeddedPlugins embed.FS
```

**Embedded plugins** (work offline, bundled in binary):
- `dns` - DNS resolution and record validation
- `http` - HTTP endpoint health and security checks
- `tcp` - TCP connectivity and TLS validation
- `smtp` - SMTP connection testing
- `file` - File system checks (exists, permissions, checksum, content)
- `command` - Command execution and output validation

**Registry-only plugins** (downloaded on demand):
- `aws` - AWS infrastructure inspection
- `gcp` - Google Cloud inspection
- `azure` - Azure inspection
- `k8s` - Kubernetes inspection

### Plugin Loading Strategy

```go
func loadPlugin(ctx context.Context, executor *host.Executor, name string) (*host.PluginInstance, error) {
    // 1. Check embedded plugins first
    if data, err := embeddedPlugins.ReadFile("plugins/" + name + ".wasm"); err == nil {
        return executor.LoadPlugin(ctx, data)
    }

    // 2. Check local cache (~/.cli/plugins/)
    cachePath := filepath.Join(os.UserHomeDir(), ".cli", "plugins", name + ".wasm")
    if data, err := os.ReadFile(cachePath); err == nil {
        return executor.LoadPlugin(ctx, data)
    }

    // 3. Download from registry (if allowed)
    return downloadAndLoad(ctx, executor, name)
}
```

---

## Output Formatting

### Schema-Driven Table Output

Table columns are derived from the operation's `output_schema`. This ensures consistent formatting and allows the CLI to intelligently format different data types:

```go
// FormatTable renders result data as a table using the output schema.
func FormatTable(result *Result, outputSchema json.RawMessage) {
    columns := getColumnsFromSchema(outputSchema)  // Uses output_schema properties
    // ... render table with columns in schema order
}
```

### Table Output (Default)

```
$ cli dns resolve --hostname example.com

┌──────────────┬─────────────┬─────────────────────┬─────┐
│ Hostname     │ Record Type │ Records             │ TTL │
├──────────────┼─────────────┼─────────────────────┼─────┤
│ example.com  │ A           │ 93.184.216.34       │ 300 │
└──────────────┴─────────────┴─────────────────────┴─────┘

$ cli aws ec2 describe_security_groups --region us-east-1

Security Groups (5 total)
┌─────────────────────┬──────────────┬─────────────────┬─────────┐
│ Group ID            │ Name         │ VPC             │ Status  │
├─────────────────────┼──────────────┼─────────────────┼─────────┤
│ sg-0123456789abcdef │ web-servers  │ vpc-abc123      │ ✓ OK    │
│ sg-abcdef012345678  │ db-servers   │ vpc-abc123      │ ✓ OK    │
│ sg-fedcba987654321  │ legacy-app   │ vpc-abc123      │ ✗ WARN  │
└─────────────────────┴──────────────┴─────────────────┴─────────┘

Issues:
  • sg-fedcba987654321 (legacy-app): SSH open to 0.0.0.0/0
```

### JSON Output

```bash
$ cli aws ec2 describe_security_groups --output json | jq '.security_groups[].group_id'
"sg-0123456789abcdef"
"sg-abcdef012345678"
"sg-fedcba987654321"
```

### Quiet Mode

```bash
$ cli aws ec2 describe_security_groups --quiet
$ echo $?
1  # Non-zero = issues found
```

---

## Shell Completion

### Dynamic Completion

Completions are generated dynamically based on:
1. Installed plugins (subcommands)
2. Plugin schemas (flags and enum values)
3. Context-aware suggestions (e.g., AWS regions)

```bash
# Bash completion function (simplified)
_cli_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "${COMP_WORDS[1]}" in
        aws)
            # Query plugin schema for completions
            COMPREPLY=($(cli __complete aws "${COMP_WORDS[@]:2}"))
            ;;
        plugin)
            COMPREPLY=($(compgen -W "list install update remove search" -- "$cur"))
            ;;
        *)
            # List installed plugins
            COMPREPLY=($(cli __complete "" "$cur"))
            ;;
    esac
}
```

### Hidden Completion Command

```bash
# Internal command used by shell completion
cli __complete aws ec2 describe_security_groups --r
# Output: --region

cli __complete aws ec2 describe_security_groups --region
# Output: us-east-1 us-west-2 eu-west-1 ... (from AWS regions enum or API)
```

---

## Configuration

### User Config (~/.cli/config.yaml)

```yaml
# Default output format
output: table

# Default timeout
timeout: 30s

# Plugin registry
registry: https://plugins.reglet.dev

# AWS defaults
aws:
  region: us-east-1
  profile: default

# Aliases
aliases:
  sg: aws ec2 describe_security_groups
  buckets: aws s3 list_buckets_with_encryption
```

### Environment Variables

```bash
CLI_OUTPUT=json             # Default output format
CLI_TIMEOUT=60s             # Default timeout
CLI_REGISTRY=https://...    # Custom registry
AWS_REGION=us-east-1        # AWS region (standard)
AWS_PROFILE=prod            # AWS profile (standard)
```

---

## Implementation Phases

### Phase 1: Core Runtime (MVP)

**Goal**: Prove the concept with a single plugin

**Deliverables**:
- [ ] CLI skeleton with cobra
- [ ] WASM runtime integration (from reglet-host-sdk)
- [ ] Load single embedded plugin (dns)
- [ ] Execute plugin and print JSON output
- [ ] Basic `cli dns resolve example.com`

### Phase 2: Schema-Driven CLI

**Goal**: Auto-generate commands from plugin schema

**Deliverables**:
- [ ] Parse plugin schema on startup
- [ ] Generate cobra commands dynamically
- [ ] Map schema properties to flags
- [ ] Help text from schema descriptions
- [ ] Multiple output formats (table, json, yaml)

### Phase 3: Shell Completion

**Goal**: Excellent tab completion experience

**Deliverables**:
- [ ] `cli completion bash/zsh/fish/powershell`
- [ ] Dynamic completions from schema enums
- [ ] Context-aware flag completions
- [ ] Hidden `__complete` command for shells

### Phase 4: Plugin Management

**Goal**: Install plugins from registry

**Deliverables**:
- [ ] `cli plugin list/install/update/remove`
- [ ] Registry client
- [ ] Plugin versioning and checksums
- [ ] Embedded core plugins
- [ ] `~/.cli/plugins/` directory

### Phase 5: Polish

**Goal**: Production-ready CLI

**Deliverables**:
- [ ] User configuration file
- [ ] Command aliases
- [ ] Error messages and diagnostics
- [ ] Man page generation
- [ ] Release binaries (goreleaser)

---

## Technical Decisions

### Why Cobra?

- Industry standard for Go CLIs
- Excellent completion support
- Easy dynamic command generation
- Familiar to contributors

### Why Separate Host-SDK and Plugin-SDK?

- CLI imports `reglet-host-sdk` + `reglet-abi`, never `reglet-plugin-sdk`
- `reglet-plugin-sdk` is for plugin authors (Op[I,O], service registration)
- `reglet-abi` provides shared wire types (Manifest, Result, GrantSet)
- Same plugins work in both CLI and Reglet because both use `reglet-host-sdk`

### Why Separate from Reglet?

- Different use case (interactive vs batch)
- Simpler, focused tool
- Separate release cycle
- Smaller binary (no compliance features)

### WASM Runtime Integration

The CLI uses `reglet-host-sdk` for WASM runtime integration. Key components:

```go
import (
    "github.com/reglet-dev/reglet-host-sdk/host"
    hostlib "github.com/reglet-dev/reglet-host-sdk"
)

// Create executor with host function registry
executor, _ := host.NewExecutor(ctx, host.WithHostFunctions(registry))

// Load and instantiate a plugin from WASM bytes
plugin, _ := executor.LoadPlugin(ctx, wasmBytes)

// Plugin methods return value types, not pointers
manifest, _ := plugin.Manifest(ctx)          // (abi.Manifest, error)
result, _ := plugin.Check(ctx, configMap)    // (abi.Result, error)
```

### Host Functions

Plugins call back to the host for I/O operations. The host-sdk provides a bundle-based registry:

```go
registry, _ := hostlib.NewRegistry(
    hostlib.WithBundle(hostlib.NetworkBundle()),    // dns_lookup, tcp_connect, http_request
    hostlib.WithBundle(hostlib.ExecBundle()),       // exec_command
    hostlib.WithBundle(hostlib.SMTPBundle()),       // smtp_connect
    hostlib.WithBundle(hostlib.NetfilterBundle()),  // ssrf_check
    hostlib.WithMiddleware(hostlib.PanicRecoveryMiddleware()),
    hostlib.WithMiddleware(hostlib.CapabilityMiddleware(checker)),
)
```

| Host Function | Bundle | Description |
|---------------|--------|-------------|
| `http_request` | `NetworkBundle` | HTTP requests |
| `dns_lookup` | `NetworkBundle` | DNS resolution |
| `tcp_connect` | `NetworkBundle` | TCP connections |
| `smtp_connect` | `SMTPBundle` | SMTP connections |
| `exec_command` | `ExecBundle` | Command execution |
| `ssrf_check` | `NetfilterBundle` | SSRF protection |
| `log_message` | (built-in) | Plugin logging |

---

## Open Questions

1. **Final name**: `cli` is a placeholder - need a real name
2. **Registry**: Shared with Reglet, or separate?
3. **Authentication**: How to handle cloud credentials? Defer to env vars?
4. **Caching**: Cache plugin manifests for faster startup?
5. **Positional args**: Should required fields like `hostname` be positional instead of flags?

---

## Available Plugins Reference

### AWS Plugin (Multi-Service)

| Service | Operation | Description |
|---------|-----------|-------------|
| `iam` | `get_account_summary` | Check if root account has MFA enabled |
| `iam` | `get_account_password_policy` | Verify IAM password policy meets requirements |
| `iam` | `list_access_keys_with_usage` | Find access keys unused for 90+ days |
| `ec2` | `describe_security_groups` | Find security groups with open SSH/RDP |
| `ec2` | `describe_instances_metadata` | Verify IMDSv2 enforcement |

**Config fields**: `service`, `operation`, `region`, `timeout_seconds`, `filters`

### DNS Plugin (Single-Service)

| Operation | Description | Input Fields |
|-----------|-------------|--------------|
| `resolve` | Resolve hostname and return DNS records | `hostname`, `record_type`, `nameserver` |

**Output fields**: `hostname`, `record_type`, `records[]`, `ttl`, `nameserver`

> Validation (e.g., "A record matches expected IP") is done via expr-lang in Reglet profiles.

### HTTP Plugin (Single-Service)

| Operation | Description | Input Fields |
|-----------|-------------|--------------|
| `request` | Perform HTTP request and return response details | `url`, `method`, `headers`, `body`, `timeout_seconds`, `follow_redirects`, `insecure_skip_verify` |

**Output fields**: `url`, `method`, `status_code`, `status`, `headers`, `body`, `body_size`, `tls_version`, `tls_expiry`, `tls_days_until_expiry`, `response_time_ms`

### TCP Plugin (Single-Service)

| Operation | Description | Input Fields |
|-----------|-------------|--------------|
| `connect` | Test TCP connection and return details | `host`, `port`, `timeout_ms`, `tls` |

**Output fields**: `host`, `port`, `connected`, `tls_version`, `tls_cipher_suite`, `tls_expiry`, `response_time_ms`

### SMTP Plugin (Single-Service)

| Operation | Description | Input Fields |
|-----------|-------------|--------------|
| `connect` | Test SMTP connection and return capabilities | `host`, `port`, `timeout_ms`, `use_tls`, `use_starttls` |

**Output fields**: `host`, `port`, `banner`, `extensions[]`, `tls_version`, `supports_auth`

### File Plugin (Single-Service)

| Operation | Description | Input Fields |
|-----------|-------------|--------------|
| `check` | Check file properties based on operation type | `path`, `operation`, `permissions`, `algorithm`, `contains` |

**Output fields**: `path`, `exists`, `is_dir`, `size`, `permissions`, `mod_time`, `checksum`, `contains`

> The `operation` field determines what's checked: `exists`, `permissions`, `checksum`, or `content`.

### Command Plugin (Single-Service)

| Operation | Description | Input Fields |
|-----------|-------------|--------------|
| `execute` | Execute command and return output | `run` OR `command`+`args`, `dir`, `env`, `timeout_seconds` |

**Output fields**: `command`, `exit_code`, `stdout`, `stderr`, `duration_ms`

> Exit code and output validation is done via expr-lang in Reglet profiles.

---

## Success Criteria

1. **Fast startup**: < 100ms to first prompt with completions
2. **Intuitive**: New users can discover commands via tab completion
3. **Scriptable**: JSON output works with jq, scripts
4. **Extensible**: Easy to add new plugins
5. **Portable**: Single binary, no dependencies

---

## Related Documentation

- **Typed Operations Design**: See [`reglet-plugin-sdk/docs/design/typed-operations/`](../reglet-plugin-sdk/docs/design/typed-operations/README.md) for the SDK-level implementation of typed plugin operations, including:
  - Phase 1-4: SDK core types, manifest schema, handler wrapping, testing
  - Phase 5-6: Plugin conversions (DNS as reference, then HTTP/TCP/SMTP/File/Command)
  - Phase 7: CLI integration details (this document's implementation)
  - Note: The CLI consumes the manifest schema produced by typed operations but does not import `reglet-plugin-sdk` itself. It uses `reglet-host-sdk` for WASM runtime and `reglet-abi` for shared types.
