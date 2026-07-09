package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// Client implements provider.Provider for Azure Key Vault.
type Client struct {
	az *azsecrets.Client
}

// secretName converts a SecretPath to Azure Key Vault secret name.
// Convention: {env}--{app}--{key}
func secretName(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--%s", path.Env, path.App, path.Key)
}

// listPrefix returns the name prefix used to filter secrets for an app.
func listPrefix(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--", path.Env, path.App)
}

// New creates an Azure Key Vault client using default Azure credential.
// Vault URL taken from cfg.Azure.VaultURL or cfg.Mount.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	vaultURL := cfg.Azure.VaultURL
	if vaultURL == "" {
		vaultURL = cfg.Mount
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}
	return NewWithCredential(vaultURL, cred, nil)
}

// NewWithCredential creates a Client with an explicit credential and options.
// Used in tests to inject a mock transport and fake credential.
func NewWithCredential(vaultURL string, cred azcore.TokenCredential, opts *azsecrets.ClientOptions) (*Client, error) {
	az, err := azsecrets.NewClient(vaultURL, cred, opts)
	if err != nil {
		return nil, fmt.Errorf("azure keyvault client: %w", err)
	}
	return &Client{az: az}, nil
}

func is404(err error) bool {
	var re *azcore.ResponseError
	return errors.As(err, &re) && re.StatusCode == http.StatusNotFound
}

func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	resp, err := c.az.GetSecret(ctx, secretName(path), "", nil)
	if err != nil {
		if is404(err) {
			return "", provider.ErrNotFound
		}
		return "", fmt.Errorf("azure get %s: %w", secretName(path), err)
	}
	if resp.Value == nil {
		return "", provider.ErrNotFound
	}
	return *resp.Value, nil
}

func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	_, err := c.az.SetSecret(ctx, secretName(path), azsecrets.SetSecretParameters{Value: &value}, nil)
	if err != nil {
		return fmt.Errorf("azure set %s: %w", secretName(path), err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	name := secretName(path)
	_, err := c.az.DeleteSecret(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("azure delete %s: %w", name, err)
	}
	_, err = c.az.PurgeDeletedSecret(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("azure purge %s: %w", name, err)
	}
	return nil
}

func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	prefix := listPrefix(path)
	pager := c.az.NewListSecretPropertiesPager(nil)
	var keys []string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("azure list: %w", err)
		}
		for _, sp := range page.Value {
			if sp.ID == nil {
				continue
			}
			n := sp.ID.Name()
			if strings.HasPrefix(n, prefix) {
				keys = append(keys, strings.TrimPrefix(n, prefix))
			}
		}
	}
	if keys == nil {
		return []string{}, nil
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	_, err := c.az.GetSecret(ctx, secretName(path), "", nil)
	if err != nil {
		if is404(err) {
			return false, nil
		}
		return false, fmt.Errorf("azure exists %s: %w", secretName(path), err)
	}
	return true, nil
}
