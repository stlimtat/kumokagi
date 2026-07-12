package factory

import (
	"context"
	"fmt"
	"sync"

	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// Constructor builds a Provider for a given config.
type Constructor func(ctx context.Context, cfg *config.Config) (provider.Provider, error)

var (
	mu       sync.RWMutex
	registry = map[string]Constructor{}
)

// Register registers a backend constructor. Called from provider init() functions.
func Register(name string, fn Constructor) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = fn
}

// New returns the Provider for cfg.Backend.
// Import _ "github.com/stlimtat/kumokagi/pkg/factory/all" to register all backends.
func New(ctx context.Context, cfg *config.Config) (provider.Provider, error) {
	mu.RLock()
	fn, ok := registry[cfg.Backend]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown backend %q — did you import its package?", cfg.Backend)
	}
	p, err := fn(ctx, cfg)
	if err != nil {
		return nil, err
	}
	// Wrap so every SecretPath is validated before it reaches a backend path,
	// resource name, or the op CLI argv — the single injection chokepoint.
	return provider.NewValidating(p), nil
}
