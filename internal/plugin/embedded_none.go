//go:build !embed_plugins

// Package plugin provides plugin discovery, loading, and lifecycle management.
package plugin

import "embed"

// EmbeddedPlugins is empty when built without the embed_plugins tag.
var EmbeddedPlugins embed.FS
