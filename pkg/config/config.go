package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/stlimtat/kumokagi/pkg/provider"
	"gopkg.in/yaml.v3"
)

const DefaultMount = "secret"
const EnvVarName = "KUMOKAGI_ENV"
const FileName = ".kumokagi.yaml"

// maxConfigBytes caps the config file size to avoid a memory-exhaustion DoS
// from a hostile .kumokagi.yaml (the file is meant to be committed and small).
const maxConfigBytes = 1 << 20 // 1 MiB

// Endpoint allowlist env vars. When set (comma-separated), a backend endpoint
// resolved from config must appear in the list, or provider construction fails.
// This is opt-in and fails closed: it stops a hostile committed config from
// redirecting a backend to an attacker host and stealing the ambient token
// (e.g. VAULT_TOKEN, or an Azure token whose audience covers every Key Vault).
const (
	EnvAllowedVaultAddrs  = "KUMOKAGI_ALLOWED_VAULT_ADDRS"
	EnvAllowedAzureVaults = "KUMOKAGI_ALLOWED_AZURE_VAULTS"
	EnvAllowedGCPProjects = "KUMOKAGI_ALLOWED_GCP_PROJECTS"
)

func splitAllow(envVar string) []string {
	raw := os.Getenv(envVar)
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func endpointHost(s string) string {
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			return strings.ToLower(u.Host)
		}
	}
	return strings.ToLower(strings.Trim(s, "/"))
}

// CheckHostAllowed verifies that endpoint's host is in the allowlist named by
// envVar. An unset allowlist permits any host (opt-in).
func CheckHostAllowed(envVar, endpoint string) error {
	allow := splitAllow(envVar)
	if len(allow) == 0 {
		return nil
	}
	host := endpointHost(endpoint)
	for _, a := range allow {
		if strings.EqualFold(endpointHost(a), host) {
			return nil
		}
	}
	return fmt.Errorf("endpoint %q is not in the %s allowlist", endpoint, envVar)
}

// CheckValueAllowed verifies that value is in the allowlist named by envVar,
// matched exactly (used for non-URL identifiers such as a GCP project).
func CheckValueAllowed(envVar, value string) error {
	allow := splitAllow(envVar)
	if len(allow) == 0 {
		return nil
	}
	for _, a := range allow {
		if a == value {
			return nil
		}
	}
	return fmt.Errorf("value %q is not in the %s allowlist", value, envVar)
}

// Config holds the non-secret metadata from .kumokagi.yaml.
type Config struct {
	Backend     string            `yaml:"backend"`
	Mount       string            `yaml:"mount"`
	App         string            `yaml:"app"`
	Env         string            `yaml:"env"`
	Keys        []string          `yaml:"keys"`
	Vault       VaultConfig       `yaml:"vault"`
	AWS         AWSConfig         `yaml:"aws"`
	Azure       AzureConfig       `yaml:"azure"`
	GCP         GCPConfig         `yaml:"gcp"`
	OnePassword OnePasswordConfig `yaml:"onepassword"`
}

// VaultConfig holds Vault-specific connection config.
type VaultConfig struct {
	Address string `yaml:"address"`
}

// AWSConfig holds AWS Secrets Manager config.
type AWSConfig struct {
	Region string `yaml:"region"`
}

// AzureConfig holds Azure Key Vault config.
type AzureConfig struct {
	VaultURL string `yaml:"vault_url"`
}

// GCPConfig holds GCP Secret Manager config.
type GCPConfig struct {
	Project string `yaml:"project"`
}

// OnePasswordConfig holds 1Password CLI config (vault name goes in Mount).
type OnePasswordConfig struct{}

// Load reads a .kumokagi.yaml file. KUMOKAGI_ENV overrides the env field.
func Load(path string) (*Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if info.Size() > maxConfigBytes {
		return nil, fmt.Errorf("config %s is too large (%d bytes, max %d)", path, info.Size(), maxConfigBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Mount == "" {
		cfg.Mount = DefaultMount
	}
	if envVal := os.Getenv(EnvVarName); envVal != "" {
		cfg.Env = envVal
	}
	return &cfg, nil
}

var validBackends = map[string]bool{
	"vault": true, "aws": true, "azure": true, "gcp": true, "onepassword": true,
}

// Validate returns an error if required fields are missing.
func (c *Config) Validate() error {
	if c.Backend == "" {
		return fmt.Errorf("backend is required")
	}
	if !validBackends[c.Backend] {
		return fmt.Errorf("unknown backend %q (valid: vault, aws, azure, gcp, onepassword)", c.Backend)
	}
	if c.App == "" {
		return fmt.Errorf("app is required")
	}
	if c.Env == "" {
		return fmt.Errorf("env is required (set in config or %s)", EnvVarName)
	}
	// Validate mount/env/app and every declared key up front, so a hostile
	// .kumokagi.yaml is rejected before any value reaches a backend. Keys are
	// checked here because they are not otherwise validated until fetch time.
	base := provider.SecretPath{Mount: c.Mount, Env: c.Env, App: c.App}
	if err := base.Validate(); err != nil {
		return err
	}
	for _, key := range c.Keys {
		if err := (provider.SecretPath{Mount: c.Mount, Env: c.Env, App: c.App, Key: key}).Validate(); err != nil {
			return err
		}
	}
	switch c.Backend {
	case "azure":
		if c.Mount == "" && c.Azure.VaultURL == "" {
			return fmt.Errorf("azure backend requires vault URL in mount or azure.vault_url")
		}
	case "gcp":
		if c.Mount == "" && c.GCP.Project == "" {
			return fmt.Errorf("gcp backend requires project ID in mount or gcp.project")
		}
	case "onepassword":
		if c.Mount == "" {
			return fmt.Errorf("onepassword backend requires vault name in mount")
		}
		// A "/" in the vault name would add a path segment to the op:// ref
		// (op://{vault}/{item}/{field}) and address a different field.
		if strings.Contains(c.Mount, "/") {
			return fmt.Errorf("onepassword vault name must not contain '/': %q", c.Mount)
		}
	}
	return nil
}
