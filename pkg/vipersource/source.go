package vipersource

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// Source loads declared secrets into viper at application startup.
type Source struct {
	p   provider.Provider
	cfg *config.Config
}

// New creates a Source for the given provider and config.
func New(p provider.Provider, cfg *config.Config) *Source {
	return &Source{p: p, cfg: cfg}
}

// Load fetches all declared secrets and sets them in viper.
// Values set via v.Set() take highest precedence in viper's resolution chain.
// Call once in PersistentPreRunE or application init.
func (s *Source) Load(ctx context.Context, v *viper.Viper) error {
	for _, key := range s.cfg.Keys {
		path := provider.SecretPath{
			Mount: s.cfg.Mount,
			Env:   s.cfg.Env,
			App:   s.cfg.App,
			Key:   key,
		}
		val, err := s.p.Get(ctx, path)
		if err != nil {
			return fmt.Errorf("load secret %q: %w", key, err)
		}
		v.Set(key, val)
	}
	return nil
}

// Verify checks that all declared secrets exist without loading their values.
// Returns an error listing all missing keys.
func (s *Source) Verify(ctx context.Context) error {
	var missing []string
	for _, key := range s.cfg.Keys {
		path := provider.SecretPath{
			Mount: s.cfg.Mount,
			Env:   s.cfg.Env,
			App:   s.cfg.App,
			Key:   key,
		}
		exists, err := s.p.Exists(ctx, path)
		if err != nil {
			return fmt.Errorf("check secret %q: %w", key, err)
		}
		if !exists {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing secrets: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Prune returns keys present in the backend but not declared in config.Keys.
func (s *Source) Prune(ctx context.Context) ([]string, error) {
	path := provider.SecretPath{
		Mount: s.cfg.Mount,
		Env:   s.cfg.Env,
		App:   s.cfg.App,
	}
	backendKeys, err := s.p.List(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list backend secrets: %w", err)
	}
	declared := make(map[string]struct{}, len(s.cfg.Keys))
	for _, k := range s.cfg.Keys {
		declared[k] = struct{}{}
	}
	var orphaned []string
	for _, k := range backendKeys {
		if _, ok := declared[k]; !ok {
			orphaned = append(orphaned, k)
		}
	}
	return orphaned, nil
}
