//go:build embed_plugins

package plugin

import "embed"

// EmbeddedPlugins contains the core plugins bundled with the CLI binary.
//
// To add embedded plugins, copy .wasm files to the cli/internal/plugin/plugins/ directory.
//
//go:embed plugins/*.wasm
var EmbeddedPlugins embed.FS
