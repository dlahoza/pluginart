package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

func (m *PluginManager) start(ctx context.Context) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(m.plugins))
	for _, p := range m.plugins {
		wg.Add(1)
		go func(p *plugin) {
			defer wg.Done()
			if err := p.start(ctx); err != nil {
				errs <- fmt.Errorf("plugin %q: %w", p.cfg.Name, err)
			}
		}(p)
	}
	wg.Wait()
	close(errs)
	var errsSlice []error
	for e := range errs {
		errsSlice = append(errsSlice, e)
	}
	return errors.Join(errsSlice...)
}

func (m *PluginManager) call(ctx context.Context, name string, payload []byte) ([]byte, error) {
	p, ok := m.plugins[name]
	if !ok {
		return nil, fmt.Errorf("unknown plugin %q", name)
	}
	return p.call(ctx, payload)
}

func (m *PluginManager) shutdown(ctx context.Context) error {
	var errsSlice []error
	for _, p := range m.plugins {
		if err := p.shutdown(ctx); err != nil {
			errsSlice = append(errsSlice, err)
		}
	}
	return errors.Join(errsSlice...)
}
