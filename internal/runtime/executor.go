// Package runtime provides the WASM plugin execution environment for the CLI.
package runtime

import (
	"context"
	"fmt"

	abi "github.com/reglet-dev/reglet-abi"
	hostlib "github.com/reglet-dev/reglet-host-sdk"
	"github.com/reglet-dev/reglet-host-sdk/host"
)

// PluginRunner loads and executes WASM plugins.
type PluginRunner struct {
	executor *host.Executor
}

// RunnerOption configures a PluginRunner.
type RunnerOption func(*runnerConfig)

type runnerConfig struct {
	verbose bool
}

// WithVerbose enables or disables verbose logging.
func WithVerbose(verbose bool) RunnerOption {
	return func(c *runnerConfig) {
		c.verbose = verbose
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

	registry, err := hostlib.NewRegistry(
		hostlib.WithMiddleware(hostlib.PanicRecoveryMiddleware()),
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

	return &PluginRunner{executor: executor}, nil
}

// Close releases the WASM runtime and all loaded modules.
func (r *PluginRunner) Close(ctx context.Context) error {
	return r.executor.Close(ctx)
}

// LoadedPlugin represents a plugin that has been loaded and had its manifest read.
type LoadedPlugin struct {
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

	return &LoadedPlugin{
		instance: instance,
		Manifest: manifest,
	}, nil
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
	return p.instance.Check(ctx, config)
}
