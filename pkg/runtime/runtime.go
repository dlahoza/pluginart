// Package runtime provides the host-side PluginManager that loads, starts, and calls plugins.
package runtime

import (
	"context"
	"time"
)

// Config is the parsed representation of pluginart.toml.
type Config struct {
	Version  int            `toml:"version"`
	Defaults DefaultsConfig `toml:"defaults"`
	Plugins  []PluginConfig `toml:"plugins"`
}

// DefaultsConfig holds global defaults overridable per plugin.
type DefaultsConfig struct {
	StartupTimeout   duration `toml:"startup_timeout"`
	ShutdownTimeout  duration `toml:"shutdown_timeout"`
	HealthInterval   duration `toml:"health_interval"`
	MaxRestarts      int      `toml:"max_restarts"`
	RestartBackoffMax duration `toml:"restart_backoff_max"`
	MaxMessageBytes  int      `toml:"max_message_bytes"`
	Compression      string   `toml:"compression"`
}

// PluginConfig holds configuration for a single plugin entry.
type PluginConfig struct {
	Name         string            `toml:"name"`
	Type         string            `toml:"type"`          // binary | docker | remote
	Path         string            `toml:"path"`          // binary
	Args         []string          `toml:"args"`          // binary
	Image        string            `toml:"image"`         // docker
	Address      string            `toml:"address"`       // remote
	Transport    string            `toml:"transport"`     // unix | tcp
	ContractHash string            `toml:"contract_hash"` // sha256:<hex> from schema
	Env          map[string]string `toml:"env"`
	Resources    ResourcesConfig   `toml:"resources"`     // docker

	// Per-plugin overrides (zero value = use default)
	StartupTimeout    duration `toml:"startup_timeout"`
	ShutdownTimeout   duration `toml:"shutdown_timeout"`
	HealthInterval    duration `toml:"health_interval"`
	MaxRestarts       int      `toml:"max_restarts"`
	RestartBackoffMax duration `toml:"restart_backoff_max"`
	MaxMessageBytes   int      `toml:"max_message_bytes"`
	Compression       string   `toml:"compression"`
	DialTimeout       duration `toml:"dial_timeout"` // remote only
}

// ResourcesConfig holds Docker resource limits.
type ResourcesConfig struct {
	Memory string `toml:"memory"`
	CPUs   string `toml:"cpus"`
}

// duration is a time.Duration that unmarshals from a TOML string like "5s".
type duration struct{ time.Duration }

// PluginManager loads and manages the lifecycle of all configured plugins.
type PluginManager struct {
	cfg     *Config
	plugins map[string]*plugin
}

// NewManagerFromConfig loads pluginart.toml from path and returns a PluginManager.
func NewManagerFromConfig(path string) (*PluginManager, error) {
	return newManagerFromConfig(path)
}

// Start launches all plugins and waits for them to be ready.
func (m *PluginManager) Start(ctx context.Context) error {
	return m.start(ctx)
}

// Call invokes a plugin method by name. payload is raw FlatBuffers CallRequest bytes
// built by the generated typed client. Returns raw FlatBuffers CallResponse bytes.
func (m *PluginManager) Call(ctx context.Context, name string, payload []byte) ([]byte, error) {
	return m.call(ctx, name, payload)
}

// Shutdown gracefully stops all plugins within the configured shutdown timeout.
func (m *PluginManager) Shutdown(ctx context.Context) error {
	return m.shutdown(ctx)
}
