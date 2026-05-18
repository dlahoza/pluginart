package runtime

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

func newManagerFromConfig(path string) (*PluginManager, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported config version %d", cfg.Version)
	}
	m := &PluginManager{cfg: &cfg, plugins: make(map[string]*plugin)}
	for i := range cfg.Plugins {
		pc := &cfg.Plugins[i]
		if _, dup := m.plugins[pc.Name]; dup {
			return nil, fmt.Errorf("duplicate plugin name %q", pc.Name)
		}
		m.plugins[pc.Name] = newPlugin(pc, cfg.Defaults)
	}
	return m, nil
}
