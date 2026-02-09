// Package runtime provides the WASM plugin execution environment for the CLI.
package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	abi "github.com/reglet-dev/reglet-abi"
	"github.com/reglet-dev/reglet-abi/hostfunc"
	hostlib "github.com/reglet-dev/reglet-host-sdk"
	"github.com/reglet-dev/reglet-host-sdk/capability"
	"github.com/reglet-dev/reglet-host-sdk/capability/gatekeeper"
	"github.com/reglet-dev/reglet-host-sdk/capability/grantstore"
	"github.com/reglet-dev/reglet-host-sdk/extractor"
	"github.com/reglet-dev/reglet-host-sdk/host"
	"github.com/whiskeyjimb/tack-cli/internal/meta"
)

// PluginRunner loads and executes WASM plugins.
type PluginRunner struct {
	executor   *host.Executor
	checker    *hostlib.CapabilityChecker
	extractors *capability.Registry
	trustAll   bool
}

// RunnerOption configures a PluginRunner.
type RunnerOption func(*runnerConfig)

type runnerConfig struct {
	verbose      bool
	trustPlugins bool
}

// WithVerbose enables or disables verbose logging.
func WithVerbose(verbose bool) RunnerOption {
	return func(c *runnerConfig) {
		c.verbose = verbose
	}
}

// WithTrustPlugins enables auto-granting of capabilities.
func WithTrustPlugins(trust bool) RunnerOption {
	return func(c *runnerConfig) {
		c.trustPlugins = trust
	}
}

// NewPluginRunner creates a PluginRunner with all standard host functions registered.
//
// It sets up:
//   - NetworkBundle: dns_lookup, tcp_connect, http_request
//   - ExecBundle: exec_command
//   - SMTPBundle: smtp_connect
//   - NetfilterBundle: ssrf_check
//   - PanicRecoveryMiddleware: catches panics in host functions
//
// The caller must call Close() when done to release WASM runtime resources.
func NewPluginRunner(ctx context.Context, opts ...RunnerOption) (*PluginRunner, error) {
	config := &runnerConfig{}
	for _, opt := range opts {
		opt(config)
	}

	// Initialize capability checker with empty grants (will be populated on load)
	checker := hostlib.NewCapabilityChecker(make(map[string]*hostfunc.GrantSet))

	registry, err := hostlib.NewRegistry(
		hostlib.WithMiddleware(hostlib.PanicRecoveryMiddleware()),
		hostlib.WithMiddleware(hostlib.CapabilityMiddleware(checker)),
		hostlib.WithBundle(hostlib.AllBundles()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating host function registry: %w", err)
	}

	executor, err := host.NewExecutor(ctx,
		host.WithHostFunctions(registry),
		host.WithVerbose(config.verbose),
	)
	if err != nil {
		return nil, fmt.Errorf("creating WASM executor: %w", err)
	}

	extractors := capability.NewRegistry()
	extractor.RegisterDefaultExtractors(extractors)

	return &PluginRunner{
		executor:   executor,
		checker:    checker,
		extractors: extractors,
		trustAll:   config.trustPlugins,
	}, nil
}

// Close releases the WASM runtime and all loaded modules.
func (r *PluginRunner) Close(ctx context.Context) error {
	return r.executor.Close(ctx)
}

// LoadedPlugin represents a plugin that has been loaded and had its manifest read.
type LoadedPlugin struct {
	runner   *PluginRunner
	instance *host.PluginInstance
	Manifest abi.Manifest
}

// LoadPlugin loads a WASM binary and reads its manifest.
// Returns a LoadedPlugin ready for Check() calls.
func (r *PluginRunner) LoadPlugin(ctx context.Context, wasmBytes []byte) (*LoadedPlugin, error) {
	instance, err := r.executor.LoadPlugin(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("loading plugin: %w", err)
	}

	manifest, err := instance.Manifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	// Handle grant requests (interactive prompting)
	// If we have an extractor for this plugin, we defer prompting until Check()
	// to get "exact" capabilities. Otherwise, we prompt for the manifest now.
	_, hasExtractor := r.extractors.Get(manifest.Name)

	if !manifest.Capabilities.IsEmpty() && !hasExtractor {
		store := r.getGrantStore()
		gk := gatekeeper.NewGatekeeper(
			gatekeeper.WithStore(store),
		)

		info := map[string]capability.CapabilityInfo{
			manifest.Name: {PluginName: manifest.Name},
		}

		granted, err := gk.GrantCapabilities(&manifest.Capabilities, info, r.trustAll)
		if err != nil {
			return nil, fmt.Errorf("granting capabilities: %w", err)
		}

		// Update the checker with the granted capabilities for this plugin
		r.checker.RegisterGrants(manifest.Name, granted)
	}

	return &LoadedPlugin{
		runner:   r,
		instance: instance,
		Manifest: manifest,
	}, nil
}

func (r *PluginRunner) getGrantStore() capability.GrantStore {
	home, _ := os.UserHomeDir()
	grantsPath := filepath.Join(home, "."+meta.AppName, "grants.yaml")
	return grantstore.NewFileStore(grantstore.WithPath(grantsPath))
}

// Check executes a plugin operation with the given config.
// The config map must include "service" and "operation" keys, plus any
// operation-specific fields.
//
// Returns the plugin's Result which contains:
//   - Status: "success", "failure", or "error"
//   - Data: operation-specific output (map[string]any)
//   - Error: structured error details (if status is "error")
func (p *LoadedPlugin) Check(ctx context.Context, config map[string]any) (abi.Result, error) {
	// 1. Precise capability extraction
	if ext, ok := p.runner.extractors.Get(p.Manifest.Name); ok {
		required := ext.Extract(config)
		if required != nil && !required.IsEmpty() {
			store := p.runner.getGrantStore()
			gk := gatekeeper.NewGatekeeper(
				gatekeeper.WithStore(store),
			)

			info := map[string]capability.CapabilityInfo{
				p.Manifest.Name: {PluginName: p.Manifest.Name},
			}

			granted, err := gk.GrantCapabilities(required, info, p.runner.trustAll)
			if err != nil {
				return abi.Result{}, fmt.Errorf("granting runtime capabilities: %w", err)
			}

			// Merge with any existing grants for this session
			p.runner.checker.RegisterGrants(p.Manifest.Name, granted)
		}
	}

	// 2. Propagate plugin name for runtime enforcement
	ctx = hostlib.WithCapabilityPluginName(ctx, p.Manifest.Name)
	return p.instance.Check(ctx, config)
}
