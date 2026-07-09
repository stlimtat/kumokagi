package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
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

// URI returns the canonical kumokagi:// URI for this path.
func (p SecretPath) URI(backend string) string {
	return fmt.Sprintf("kumokagi://%s/%s/%s/%s/%s", backend, p.Mount, p.Env, p.App, p.Key)
}

// ParseURI parses a kumokagi:// URI into (backend, SecretPath, error).
// Format: kumokagi://<backend>/<mount>/<env>/<app>/<key>
// Empty mount is allowed: kumokagi://aws//prod/app/key
func ParseURI(uri string) (string, SecretPath, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", SecretPath{}, fmt.Errorf("invalid URI: %w", err)
	}
	if u.Scheme != "kumokagi" {
		return "", SecretPath{}, fmt.Errorf("URI scheme must be kumokagi://, got %q", u.Scheme)
	}
	backend := u.Host
	if backend == "" {
		return "", SecretPath{}, fmt.Errorf("URI missing backend (host)")
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 4)
	if len(parts) != 4 {
		return "", SecretPath{}, fmt.Errorf("URI path must have 4 segments /<mount>/<env>/<app>/<key>, got %q", u.Path)
	}
	if parts[3] == "" {
		return "", SecretPath{}, fmt.Errorf("URI key segment is empty")
	}
	return backend, SecretPath{Mount: parts[0], Env: parts[1], App: parts[2], Key: parts[3]}, nil
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
