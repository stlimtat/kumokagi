package provider

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a secret does not exist in the backend.
var ErrNotFound = errors.New("secret not found")

// SecretPath is the fully-qualified location of a secret.
// Convention: {mount}/data/{env}/{app}/{key}
type SecretPath struct {
	Mount string
	Env   string
	App   string
	Key   string
}

// DataPath returns the KV v2 read/write path.
func (p SecretPath) DataPath() string {
	return fmt.Sprintf("%s/data/%s/%s/%s", p.Mount, p.Env, p.App, p.Key)
}

// MetadataPath returns the KV v2 metadata path for a single key.
func (p SecretPath) MetadataPath() string {
	return fmt.Sprintf("%s/metadata/%s/%s/%s", p.Mount, p.Env, p.App, p.Key)
}

// ListPath returns the KV v2 metadata list path for an app.
func (p SecretPath) ListPath() string {
	return fmt.Sprintf("%s/metadata/%s/%s", p.Mount, p.Env, p.App)
}

// Provider defines the interface for a secrets backend.
type Provider interface {
	// Get fetches the value of a secret. Returns ErrNotFound if absent.
	Get(ctx context.Context, path SecretPath) (string, error)
	// Set creates or updates a secret value.
	Set(ctx context.Context, path SecretPath, value string) error
	// Delete permanently removes a secret.
	Delete(ctx context.Context, path SecretPath) error
	// List returns all keys present under mount/env/app/.
	List(ctx context.Context, path SecretPath) ([]string, error)
	// Exists returns true if the secret exists in the backend.
	Exists(ctx context.Context, path SecretPath) (bool, error)
}
