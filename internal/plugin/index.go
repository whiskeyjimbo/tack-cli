package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultIndexURL = "https://raw.githubusercontent.com/reglet-dev/reglet-plugins/main/index.json"

// PluginIndex represents a fetched plugin index.
type PluginIndex struct {
	Repository string        `json:"repository"`
	Registry   string        `json:"registry"`
	Updated    string        `json:"updated"`
	Plugins    []PluginEntry `json:"plugins"`
}

// PluginEntry is a single plugin in an index.
type PluginEntry struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Latest       string   `json:"latest"`
}

// IndexSource identifies where an index came from.
type IndexSource struct {
	URL  string
	Name string // "official", "community", etc.
}

// SearchResult is a PluginEntry annotated with its source.
type SearchResult struct {
	PluginEntry
	Source   string // display name of the index
	Registry string // OCI registry prefix for install
}

// FetchIndex downloads and parses a plugin index.
func FetchIndex(ctx context.Context, url string) (*PluginIndex, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching index: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading index: %w", err)
	}

	var idx PluginIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("parsing index: %w", err)
	}

	return &idx, nil
}

// SearchAll fetches all indexes and returns matching plugins.
// Empty query matches everything.
func SearchAll(ctx context.Context, sources []IndexSource, query string, forceRefresh bool) ([]SearchResult, error) {
	var results []SearchResult
	query = strings.ToLower(query)

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}
	cacheDir := filepath.Join(home, ".tack", "cache", "indexes")

	maxAge := 1 * time.Hour
	if forceRefresh {
		maxAge = 0
	}

	for _, src := range sources {
		idx, err := cachedFetch(ctx, src, cacheDir, maxAge)
		if err != nil {
			// Warn but don't fail — one bad index shouldn't block others
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch %s index: %v\n", src.Name, err)
			continue
		}

		for _, p := range idx.Plugins {
			if query == "" || strings.Contains(strings.ToLower(p.Name), query) ||
				strings.Contains(strings.ToLower(p.Description), query) {
				results = append(results, SearchResult{
					PluginEntry: p,
					Source:      src.Name,
					Registry:    idx.Registry,
				})
			}
		}
	}

	return results, nil
}

func cachedFetch(ctx context.Context, src IndexSource, cacheDir string, maxAge time.Duration) (*PluginIndex, error) {
	cachePath := filepath.Join(cacheDir, src.Name+".json")

	cached, cacheOK := readCache(cachePath)
	if cacheOK && maxAge > 0 {
		info, err := os.Stat(cachePath)
		if err == nil && time.Since(info.ModTime()) < maxAge {
			return cached, nil
		}
	}

	idx, err := FetchIndex(ctx, src.URL)
	if err != nil {
		// Serve stale cache rather than failing entirely
		if cacheOK {
			fmt.Fprintf(os.Stderr, "Warning: using stale %s index (fetch failed: %v)\n", src.Name, err)
			return cached, nil
		}
		return nil, err
	}

	_ = saveCache(idx, src, cacheDir)
	return idx, nil
}

func readCache(path string) (*PluginIndex, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var idx PluginIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, false
	}
	return &idx, true
}

func saveCache(idx *PluginIndex, src IndexSource, cacheDir string) error {
	cachePath := filepath.Join(cacheDir, src.Name+".json")
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0o644)
}
