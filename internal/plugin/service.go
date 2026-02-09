// Package plugin provides plugin discovery, loading, and lifecycle management.
package plugin

import (
	"log/slog"
	"os"
	"path/filepath"

	hostplugin "github.com/reglet-dev/reglet-host-sdk/plugin"
	hostoci "github.com/reglet-dev/reglet-host-sdk/plugin/oci"
	hostrepository "github.com/reglet-dev/reglet-host-sdk/plugin/repository"
	hostresolvers "github.com/reglet-dev/reglet-host-sdk/plugin/resolvers"
	hostservices "github.com/reglet-dev/reglet-host-sdk/plugin/services"
	hostsigning "github.com/reglet-dev/reglet-host-sdk/plugin/signing"
)

// PluginServiceConfig holds configuration for the plugin service stack.
type PluginServiceConfig struct {
	// CacheDir is the local plugin cache directory.
	// Default: ~/.cli/plugins/
	CacheDir string

	// RequireSigning controls whether signature verification is mandatory.
	RequireSigning bool

	// Logger for plugin operations. If nil, uses slog.Default().
	Logger *slog.Logger
}

// PluginStack holds the initialized host-sdk plugin management components.
type PluginStack struct {
	Service    *hostplugin.PluginService
	Repository *hostrepository.FSPluginRepository
}

// NewPluginStack creates the full host-sdk plugin management stack.
func NewPluginStack(cfg PluginServiceConfig) (*PluginStack, error) {
	if cfg.CacheDir == "" {
		cfg.CacheDir = DefaultPluginsDir()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// 1. Auth Provider (env-based: REGISTRY_USERNAME, REGISTRY_PASSWORD)
	authProvider := hostoci.NewEnvAuthProvider()

	// 2. OCI Registry Adapter
	registryAdapter := hostoci.NewOCIRegistryAdapter(authProvider)

	// 3. Local Plugin Cache
	repository, err := hostrepository.NewFSPluginRepository(cfg.CacheDir)
	if err != nil {
		return nil, err
	}

	// 4. Integrity
	integrityVerifier := hostsigning.NewCosignVerifier(nil, nil)
	integrityService := hostservices.NewIntegrityService(cfg.RequireSigning)

	// 5. Resolver Chain: Cache -> Registry
	registryResolver := hostresolvers.NewRegistryPluginResolver(
		registryAdapter,
		repository,
		cfg.Logger,
	)
	cachedResolver := hostresolvers.NewCachedPluginResolver(repository)
	cachedResolver.SetNext(registryResolver)

	// 6. Plugin Service
	service := hostplugin.NewPluginService(
		repository,
		registryAdapter,
		hostplugin.WithResolver(cachedResolver),
		hostplugin.WithIntegrityVerifier(integrityVerifier),
		hostplugin.WithIntegrityService(integrityService),
		hostplugin.WithLogger(cfg.Logger),
	)

	return &PluginStack{
		Service:    service,
		Repository: repository,
	}, nil
}

// DefaultPluginsDir returns the default local plugin cache directory.
// ~/.cli/plugins/
func DefaultPluginsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".cli", "plugins")
	}
	return filepath.Join(home, ".cli", "plugins")
}

// EnsurePluginsDir creates the plugins directory if it doesn't exist.
func EnsurePluginsDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
