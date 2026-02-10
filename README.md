# ![logo-small](.github/assets/tack-rotate.png) Tack

Pluggable CLI that runs WASM plugins built with the [reglet SDK](https://github.com/reglet-dev/reglet-plugin-sdk). Each plugin contributes its own commands, flags, and completions.

<p align="center">
  <a href="https://github.com/whiskeyjimbo/tack-cli/actions"><img src="https://github.com/whiskeyjimbo/tack-cli/workflows/CI/badge.svg" alt="Build Status"></a>
  <a href="https://goreportcard.com/report/github.com/whiskeyjimbo/tack-cli"><img src="https://goreportcard.com/badge/github.com/whiskeyjimbo/tack-cli?style=flat" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <img src="https://img.shields.io/github/v/release/whiskeyjimbo/tack-cli?include_prereleases" alt="Latest Release">
</p>

## Usage

```
tack <plugin> <operation> [flags]
tack <plugin> <service> <operation> [flags]    # multi-service plugins (e.g. aws)
```

```bash
tack dns resolve --hostname example.com --record-type A
tack http check --url https://api.example.com/health --expected-status 200
tack file check --path /etc/hostname
tack tcp connect --host db.internal --port 5432
tack smtp check --host mail.example.com --port 587

# aws has multiple services, so the service name comes first
tack aws ec2 describe_security_groups --region us-east-1
tack aws iam get_account_summary
tack aws s3 list_buckets
```

Output as `--output table` (default), `json`, `yaml`, or `--quiet` (exit code only).

## Plugins

Official plugins from [reglet-plugins](https://github.com/reglet-dev/reglet-plugins):

| Plugin | Description |
|--------|-------------|
| aws | AWS infrastructure inspection and compliance |
| command | Execute commands and validate output |
| dns | DNS resolution and record validation |
| file | File system checks and validation |
| http | HTTP/HTTPS request checking and validation |
| smtp | SMTP connection testing and server validation |
| tcp | TCP connection testing and TLS validation |

```bash
tack plugin search                                        # browse available plugins
tack plugin install dns                                   # from default registry
tack plugin install dns@1.2.0                             # pinned version
tack plugin install ghcr.io/my-org/plugins/custom:1.0.0   # custom registry
tack plugin install ./my-plugin.wasm                      # local file
tack plugin list
tack plugin remove dns
tack plugin prune --keep 3
```

## Configuration

`~/.tack/config.yaml`

```yaml
output: table
timeout: 30s
default_registry: ghcr.io/reglet-dev/plugins

plugin_defaults:
  aws:
    region: us-east-1

aliases:
  sg: aws ec2 describe_security_groups
  buckets: aws s3 list_buckets
```

Aliases create top-level shortcuts: `tack sg --region us-west-2`.

Env vars `TACK_OUTPUT`, `TACK_TIMEOUT`, `TACK_DEFAULT_REGISTRY` override the config file.

## Building

```bash
make build          # build binary
make build-embed    # build with plugins baked in
make install        # install to GOBIN
make test           # run tests
make check          # fmt + vet + test
```

## Shell Completions

```bash
tack completion bash > /etc/bash_completion.d/tack
tack completion zsh > "${fpath[1]}/_tack"
tack completion fish > ~/.config/fish/completions/tack.fish
```

## License

Apache 2.0
