package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ErrNotFound is returned when a secret does not exist in the backend.
var ErrNotFound = errors.New("secret not found")

// ErrInvalidPath is returned when a SecretPath component fails validation.
var ErrInvalidPath = errors.New("invalid secret path")

// identifierRe matches a safe env/app/key component: alphanumerics plus
// dot/underscore/dash, first char never a dash. Forbidding "/" blocks Vault
// logical-path traversal; forbidding a leading "-" and "="/"["/"]" blocks
// option and assignment injection into the op CLI. Go's RE2 anchors "$" at end
// of text, so a trailing newline is already rejected. A component equal to "."
// or ".." still passes this pattern, so validateIdentifier rejects those
// explicitly: as a lone path segment, ".." can be collapsed by an HTTP router
// (e.g. Vault's) into a real traversal.
var identifierRe = regexp.MustCompile(`^[A-Za-z0-9_.][A-Za-z0-9._-]{0,252}$`)

// validateIdentifier reports whether s is a safe path component.
func validateIdentifier(field, s string) error {
	if s == "." || s == ".." {
		return fmt.Errorf("%w: %s %q is a reserved path segment", ErrInvalidPath, field, s)
	}
	if !identifierRe.MatchString(s) {
		return fmt.Errorf("%w: %s %q", ErrInvalidPath, field, s)
	}
	return nil
}

// SecretPath is the fully-qualified location of a secret.
// Convention: {mount}/data/{env}/{app}/{key}
type SecretPath struct {
	Mount string
	Env   string
	App   string
	Key   string
}

// Validate rejects SecretPath components that could inject into a backend path,
// resource name, list filter, or the op CLI argv. Env and App are always
// required and must be safe identifiers; Key is validated only when present,
// so listing/prune paths (which carry no key) pass. Mount is checked loosely —
// it may legitimately be a URL (Azure) or empty (AWS ignores it) — but must not
// contain traversal sequences or control characters.
func (p SecretPath) Validate() error {
	if err := validateMount(p.Mount); err != nil {
		return err
	}
	if err := validateIdentifier("env", p.Env); err != nil {
		return err
	}
	if err := validateIdentifier("app", p.App); err != nil {
		return err
	}
	if p.Key != "" {
		if err := validateIdentifier("key", p.Key); err != nil {
			return err
		}
	}
	return nil
}

// validateMount rejects traversal sequences and control characters. Mount is
// overloaded across backends (Vault mount, Azure vault URL, GCP project, 1P
// vault name) and may be empty for AWS, so it cannot use the strict identifier
// rule.
func validateMount(mount string) error {
	if mount == "" {
		return nil
	}
	if strings.Contains(mount, "..") {
		return fmt.Errorf("%w: mount %q contains %q", ErrInvalidPath, mount, "..")
	}
	for _, r := range mount {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: mount contains a control character", ErrInvalidPath)
		}
	}
	return nil
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
	path := SecretPath{Mount: parts[0], Env: parts[1], App: parts[2], Key: parts[3]}
	if err := path.Validate(); err != nil {
		return "", SecretPath{}, err
	}
	return backend, path, nil
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

// Validating wraps a Provider and validates every SecretPath before delegating.
// factory.New returns providers already wrapped, so every command, the viper
// source, and the URI reader are guarded at one chokepoint.
type Validating struct {
	inner Provider
}

// NewValidating wraps p so that all paths are validated before use.
func NewValidating(p Provider) *Validating {
	return &Validating{inner: p}
}

func (v *Validating) Get(ctx context.Context, path SecretPath) (string, error) {
	if err := path.Validate(); err != nil {
		return "", err
	}
	return v.inner.Get(ctx, path)
}

func (v *Validating) Set(ctx context.Context, path SecretPath, value string) error {
	if err := path.Validate(); err != nil {
		return err
	}
	return v.inner.Set(ctx, path, value)
}

func (v *Validating) Delete(ctx context.Context, path SecretPath) error {
	if err := path.Validate(); err != nil {
		return err
	}
	return v.inner.Delete(ctx, path)
}

func (v *Validating) List(ctx context.Context, path SecretPath) ([]string, error) {
	if err := path.Validate(); err != nil {
		return nil, err
	}
	return v.inner.List(ctx, path)
}

func (v *Validating) Exists(ctx context.Context, path SecretPath) (bool, error) {
	if err := path.Validate(); err != nil {
		return false, err
	}
	return v.inner.Exists(ctx, path)
}
