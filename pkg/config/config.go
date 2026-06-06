package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultMount = "secret"
const EnvVarName = "KUMOKAGI_ENV"
const FileName = ".kumokagi.yaml"

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

// Validate returns an error if required fields are missing.
func (c *Config) Validate() error {
	if c.Backend == "" {
		return fmt.Errorf("backend is required")
	}
	if c.App == "" {
		return fmt.Errorf("app is required")
	}
	if c.Env == "" {
		return fmt.Errorf("env is required (set in config or %s)", EnvVarName)
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
	}
	return nil
}
