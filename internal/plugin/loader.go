// Package plugin provides plugin discovery, loading, and lifecycle management.
package plugin

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/reglet-dev/cli/internal/runtime"
	abi "github.com/reglet-dev/reglet-abi"
	hostdto "github.com/reglet-dev/reglet-host-sdk/plugin/dto"
)

// DiscoveredPlugin holds a plugin's WASM loader and manifest.
type DiscoveredPlugin struct {
	Manifest abi.Manifest
	Loader   func() ([]byte, error)
	Source   string // "embedded", "local", or "oci"
	Path     string // file path (for local/oci plugins)
}

// Loader discovers and loads plugins from multiple sources.
type Loader struct {
	embeddedFS embed.FS     // Embedded WASM files
	pluginsDir string       // Local plugins directory (~/.cli/plugins/)
	cachePath  string       // Path to discovery cache
	stack      *PluginStack // Host-sdk plugin service (for OCI fallback)
	defaultReg string       // Default OCI registry prefix
}

// NewLoader creates a plugin Loader.
// stack may be nil to disable OCI fallback.
func NewLoader(embeddedFS embed.FS, pluginsDir string, stack *PluginStack, defaultRegistry string) *Loader {
	return &Loader{
		embeddedFS: embeddedFS,
		pluginsDir: pluginsDir,
		cachePath:  DefaultCachePath(),
		stack:      stack,
		defaultReg: defaultRegistry,
	}
}

// DiscoverAll finds and loads all available plugins.
func (l *Loader) DiscoverAll(ctx context.Context) ([]DiscoveredPlugin, error) {
	cache := LoadCache(l.cachePath)
	plugins := make(map[string]DiscoveredPlugin)
	cacheUpdated := false

	// 1. Load embedded plugins
	embedded, updatedE, err := l.loadEmbeddedPlugins(ctx, cache)
	if err != nil {
		return nil, fmt.Errorf("loading embedded plugins: %w", err)
	}
	for _, p := range embedded {
		plugins[p.Manifest.Name] = p
	}
	if updatedE {
		cacheUpdated = true
	}

	// 2. Load local plugins (override embedded if same name)
	local, updatedL, err := l.loadLocalPlugins(ctx, cache)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading local plugins: %w", err)
		}
	}
	for _, p := range local {
		plugins[p.Manifest.Name] = p
	}
	if updatedL {
		cacheUpdated = true
	}

	// Save cache if updated
	if cacheUpdated {
		_ = cache.Save(l.cachePath)
	}

	// Convert map to slice
	result := make([]DiscoveredPlugin, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, p)
	}

	return result, nil
}

// LoadByName loads a specific plugin by name or OCI reference.
//
// Resolution order:
//  1. Local cache: ~/.cli/plugins/<name>.wasm or <name>@*.wasm
//  2. Embedded: plugins/<name>.wasm
//  3. OCI registry: <default_registry>/<name>:latest (if stack is configured)
func (l *Loader) LoadByName(ctx context.Context, name string) (*DiscoveredPlugin, error) {
	// 1. Check local cache (unversioned)
	localPath := filepath.Join(l.pluginsDir, name+".wasm")
	if _, err := os.Stat(localPath); err == nil {
		return l.loadLocalFile(ctx, localPath)
	}

	// Check local cache (versioned, pick latest alphabetically)
	matches, _ := filepath.Glob(filepath.Join(l.pluginsDir, name+"@*.wasm"))
	if len(matches) > 0 {
		return l.loadLocalFile(ctx, matches[len(matches)-1])
	}

	// 2. Check embedded plugins
	embeddedPath := "plugins/" + name + ".wasm"
	if _, err := l.embeddedFS.Open(embeddedPath); err == nil {
		return l.loadEmbeddedFile(ctx, embeddedPath)
	}

	// 3. OCI fallback \u2014 resolve via host-sdk PluginService
	if l.stack != nil {
		return l.loadFromOCI(ctx, name)
	}

	return nil, fmt.Errorf("plugin %q not found", name)
}

// loadFromOCI resolves a plugin name to an OCI reference and loads it
// via the host-sdk PluginService.
func (l *Loader) loadFromOCI(ctx context.Context, name string) (*DiscoveredPlugin, error) {
	ref := l.resolveOCIReference(name)

	dto := &hostdto.PluginSpecDTO{Name: ref}
	wasmPath, err := l.stack.Service.LoadPlugin(ctx, dto)
	if err != nil {
		return nil, fmt.Errorf("loading plugin %q from OCI: %w", name, err)
	}

	data, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("reading cached plugin: %w", err)
	}

	return l.loadPluginBytes(ctx, data, "oci", wasmPath)
}

// resolveOCIReference builds a full OCI reference from a plugin name.
func (l *Loader) resolveOCIReference(name string) string {
	// Already a full OCI reference
	if strings.Contains(name, "/") {
		return name
	}

	// Parse name@version
	pluginName, version := parseNameVersion(name)
	if version == "" {
		version = "latest"
	}

	return fmt.Sprintf("%s/%s:%s", l.defaultReg, pluginName, version)
}

// parseNameVersion splits "aws@1.2.0" into ("aws", "1.2.0").
func parseNameVersion(s string) (string, string) {
	if idx := strings.Index(s, "@"); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func (l *Loader) loadLocalFile(ctx context.Context, path string) (*DiscoveredPlugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return l.loadPluginBytes(ctx, data, "local", path)
}

func (l *Loader) loadEmbeddedFile(ctx context.Context, path string) (*DiscoveredPlugin, error) {
	data, err := l.embeddedFS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return l.loadPluginBytes(ctx, data, "embedded", "embedded://"+path)
}

func (l *Loader) loadEmbeddedPlugins(ctx context.Context, cache *DiscoveryCache) ([]DiscoveredPlugin, bool, error) {
	entries, err := l.embeddedFS.ReadDir("plugins")
	if err != nil {
		return nil, false, nil
	}

	var plugins []DiscoveredPlugin
	updated := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}

		path := "plugins/" + entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}

		cacheKey := "embedded://" + path
		if cached, ok := cache.Files[cacheKey]; ok && cached.Size == info.Size() {
			plugins = append(plugins, DiscoveredPlugin{
				Manifest: cached.Manifest,
				Loader:   func() ([]byte, error) { return l.embeddedFS.ReadFile(path) },
				Source:   "embedded",
				Path:     cacheKey,
			})
			continue
		}

		// Cache miss
		data, err := l.embeddedFS.ReadFile(path)
		if err != nil {
			continue
		}
		p, err := l.loadPluginBytes(ctx, data, "embedded", cacheKey)
		if err != nil {
			continue
		}

		cache.Files[cacheKey] = CacheEntry{
			Size:     info.Size(),
			Manifest: p.Manifest,
		}
		updated = true
		plugins = append(plugins, *p)
	}

	return plugins, updated, nil
}

func (l *Loader) loadLocalPlugins(ctx context.Context, cache *DiscoveryCache) ([]DiscoveredPlugin, bool, error) {
	entries, err := os.ReadDir(l.pluginsDir)
	if err != nil {
		return nil, false, err
	}

	var plugins []DiscoveredPlugin
	updated := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}

		path := filepath.Join(l.pluginsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if cached, ok := cache.Files[path]; ok && cached.Size == info.Size() && cached.ModTime.Equal(info.ModTime()) {
			plugins = append(plugins, DiscoveredPlugin{
				Manifest: cached.Manifest,
				Loader:   func() ([]byte, error) { return os.ReadFile(path) },
				Source:   "local",
				Path:     path,
			})
			continue
		}

		// Cache miss
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		p, err := l.loadPluginBytes(ctx, data, "local", path)
		if err != nil {
			continue
		}

		cache.Files[path] = CacheEntry{
			ModTime:  info.ModTime(),
			Size:     info.Size(),
			Manifest: p.Manifest,
		}
		updated = true
		plugins = append(plugins, *p)
	}

	return plugins, updated, nil
}

func (l *Loader) loadPluginBytes(ctx context.Context, data []byte, source, path string) (*DiscoveredPlugin, error) {
	runner, err := runtime.NewPluginRunner(ctx)
	if err != nil {
		return nil, err
	}
	defer runner.Close(ctx)

	loaded, err := runner.LoadPlugin(ctx, data)
	if err != nil {
		return nil, err
	}

	return &DiscoveredPlugin{
		Manifest: loaded.Manifest,
		Loader:   func() ([]byte, error) { return data, nil },
		Source:   source,
		Path:     path,
	}, nil
}
