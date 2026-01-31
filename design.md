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

---

## Usage Examples

### Basic Operations

```bash
# AWS operations
cli aws iam get_account_summary
cli aws ec2 describe_security_groups --region us-east-1
cli aws s3 list_buckets --output json

# DNS lookups
cli dns resolve api.example.com
cli dns resolve example.com --type MX,TXT

# HTTP checks
cli http get https://api.example.com/health
cli http get https://example.com --follow-redirects --timeout 10s

# Certificate inspection
cli cert check example.com:443
cli cert check example.com --warn-days 30

# TCP connectivity
cli tcp check db.internal:5432 --timeout 5s
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

### Shell Completion

```bash
# Generate completion scripts
cli completion bash > /etc/bash_completion.d/cli
cli completion zsh > ~/.zsh/completions/_cli
cli completion fish > ~/.config/fish/completions/cli.fish

# Completions are dynamic based on installed plugins
cli <TAB>           # aws, dns, http, cert, tcp, plugin, completion, help
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
│   │   ├── dynamic.go          # Dynamic command generation from schemas
│   │   └── completion.go       # Shell completion logic
│   ├── plugin/
│   │   ├── manager.go          # Plugin lifecycle (install, update, remove)
│   │   ├── registry.go         # Registry client
│   │   ├── loader.go           # WASM plugin loading
│   │   └── schema.go           # Schema parsing and validation
│   ├── runtime/
│   │   ├── executor.go         # Plugin execution
│   │   └── hostfuncs.go        # Host functions (reuse from reglet-sdk)
│   ├── output/
│   │   ├── table.go            # Human-readable tables
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
    github.com/reglet-dev/reglet-sdk/go  // WASM runtime, host functions
    github.com/spf13/cobra               // CLI framework
    github.com/charmbracelet/lipgloss    // Terminal styling
    github.com/olekukonko/tablewriter    // Table output
)
```

---

## Schema-to-CLI Mapping

Plugins expose a JSON Schema via `Describe()`. CLI parses this to generate commands.

### Plugin Schema Example (AWS)

```json
{
  "name": "aws",
  "version": "1.0.0",
  "description": "AWS infrastructure inspection",
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
        "description": "AWS region (defaults to AWS_REGION env var)"
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
  "operations": {
    "iam": ["get_account_summary", "get_account_password_policy", "list_access_keys_with_usage"],
    "ec2": ["describe_security_groups", "describe_instances_metadata"],
    "s3": ["list_buckets_with_encryption", "list_buckets_with_public_access", "list_buckets_with_versioning"]
  }
}
```

### Generated CLI Structure

```
cli aws
├── iam
│   ├── get_account_summary
│   ├── get_account_password_policy
│   └── list_access_keys_with_usage
├── ec2
│   ├── describe_security_groups [--region] [--filters]
│   └── describe_instances_metadata [--region] [--filters]
├── s3
│   ├── list_buckets_with_encryption
│   ├── list_buckets_with_public_access
│   └── list_buckets_with_versioning
└── [global flags: --timeout, --output, --quiet]
```

### Mapping Rules

| Schema Property | CLI Element |
|-----------------|-------------|
| `enum` with few values | Nested subcommands |
| `enum` with many values | Flag with completion |
| `string` | `--flag value` |
| `integer` | `--flag 123` |
| `boolean` | `--flag` (presence = true) |
| `array` | `--flag val1 --flag val2` or `--flag val1,val2` |
| `object` | `--flag key=value` |
| `required` | Positional arg or required flag |
| `default` | Flag default value |
| `description` | Help text |

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

Embedded: `dns`, `http`, `tcp`, `cert`, `file`
Registry-only: `aws`, `gcp`, `azure`, `k8s`

---

## Output Formatting

### Table Output (Default)

```
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
- [ ] WASM runtime integration (from reglet-sdk)
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

### Why Reuse reglet-sdk?

- Same WASM runtime (wazero)
- Same host functions
- Plugins work in both contexts
- No code duplication

### Why Separate from Reglet?

- Different use case (interactive vs batch)
- Simpler, focused tool
- Separate release cycle
- Smaller binary (no compliance features)

### Plugin Schema Extensions

Plugins may need additional metadata for CLI generation:

```json
{
  "cli_hints": {
    "operations_as_subcommands": true,
    "service_as_subcommand": true,
    "region_completion": "aws_regions",
    "hidden_flags": ["internal_option"]
  }
}
```

---

## Open Questions

1. **Final name**: `cli` is a placeholder - need a real name
2. **Plugin format**: Same WASM as Reglet, or need metadata sidecar?
3. **Registry**: Shared with Reglet, or separate?
4. **Authentication**: How to handle cloud credentials? Defer to env vars?
5. **Caching**: Cache plugin schemas for faster startup?

---

## Success Criteria

1. **Fast startup**: < 100ms to first prompt with completions
2. **Intuitive**: New users can discover commands via tab completion
3. **Scriptable**: JSON output works with jq, scripts
4. **Extensible**: Easy to add new plugins
5. **Portable**: Single binary, no dependencies
