// Package plugin provides plugin discovery, loading, and lifecycle management.
package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	abi "github.com/reglet-dev/reglet-abi"
)

// DiscoveryCache stores extracted manifests to speed up plugin registration.
type DiscoveryCache struct {
	// Files maps file paths (or embedded URLs) to cached metadata.
	Files map[string]CacheEntry `json:"files"`
}

// CacheEntry holds metadata and manifest for a single plugin file.
type CacheEntry struct {
	ModTime  time.Time    `json:"mod_time"`
	Size     int64        `json:"size"`
	Manifest abi.Manifest `json:"manifest"`
}

// NewDiscoveryCache creates a new, empty cache.
func NewDiscoveryCache() *DiscoveryCache {
	return &DiscoveryCache{
		Files: make(map[string]CacheEntry),
	}
}

// LoadCache reads the discovery cache from disk.
// Returns an empty cache if the file does not exist or is invalid.
func LoadCache(path string) *DiscoveryCache {
	data, err := os.ReadFile(path)
	if err != nil {
		return NewDiscoveryCache()
	}

	var cache DiscoveryCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return NewDiscoveryCache()
	}

	if cache.Files == nil {
		cache.Files = make(map[string]CacheEntry)
	}

	return &cache
}

// Save writes the discovery cache to disk.
func (c *DiscoveryCache) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// DefaultCachePath returns the default location for the discovery cache.
// ~/.cli/discovery_cache.json
func DefaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".cli", "discovery_cache.json")
	}
	return filepath.Join(home, ".cli", "discovery_cache.json")
}
