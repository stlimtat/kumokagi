package vault

import (
	"context"
	"fmt"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// Client implements provider.Provider for HashiCorp Vault KV v2.
type Client struct {
	vault *vaultapi.Client
}

// New creates a Vault client using the SDK default credential chain.
// VAULT_ADDR and VAULT_TOKEN are read from the environment automatically.
func New(ctx context.Context) (*Client, error) {
	return NewWithAddress(ctx, "")
}

// NewWithAddress creates a Vault client with an explicit address.
// An empty address falls back to VAULT_ADDR environment variable.
func NewWithAddress(ctx context.Context, address string) (*Client, error) {
	cfg := vaultapi.DefaultConfig()
	if address != "" {
		cfg.Address = address
	}
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create vault client: %w", err)
	}
	return &Client{vault: client}, nil
}

func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	secret, err := c.vault.Logical().ReadWithContext(ctx, path.DataPath())
	if err != nil {
		return "", fmt.Errorf("vault get %s: %w", path.DataPath(), err)
	}
	if secret == nil {
		return "", provider.ErrNotFound
	}
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return "", provider.ErrNotFound
	}
	val, ok := data["value"].(string)
	if !ok {
		return "", fmt.Errorf("vault get %s: value field missing or not a string", path.DataPath())
	}
	return val, nil
}

func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	_, err := c.vault.Logical().WriteWithContext(ctx, path.DataPath(), map[string]interface{}{
		"data": map[string]interface{}{"value": value},
	})
	if err != nil {
		return fmt.Errorf("vault set %s: %w", path.DataPath(), err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	_, err := c.vault.Logical().DeleteWithContext(ctx, path.MetadataPath())
	if err != nil {
		return fmt.Errorf("vault delete %s: %w", path.MetadataPath(), err)
	}
	return nil
}

func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	secret, err := c.vault.Logical().ListWithContext(ctx, path.ListPath())
	if err != nil {
		return nil, fmt.Errorf("vault list %s: %w", path.ListPath(), err)
	}
	if secret == nil || secret.Data == nil {
		return []string{}, nil
	}
	rawKeys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return []string{}, nil
	}
	keys := make([]string, 0, len(rawKeys))
	for _, k := range rawKeys {
		if s, ok := k.(string); ok {
			keys = append(keys, s)
		}
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	secret, err := c.vault.Logical().ReadWithContext(ctx, path.MetadataPath())
	if err != nil {
		return false, fmt.Errorf("vault exists %s: %w", path.MetadataPath(), err)
	}
	return secret != nil, nil
}
