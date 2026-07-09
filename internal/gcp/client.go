package gcp

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client implements provider.Provider for GCP Secret Manager.
type Client struct {
	sm      *secretmanager.Client
	project string
}

// New creates a GCP Secret Manager client using Application Default Credentials.
// Project is taken from cfg.GCP.Project or cfg.Mount.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	project := cfg.GCP.Project
	if project == "" {
		project = cfg.Mount
	}
	sm, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create gcp client: %w", err)
	}
	return &Client{sm: sm, project: project}, nil
}

// NewWithClient creates a Client with a pre-built secretmanager.Client (for tests).
func NewWithClient(sm *secretmanager.Client, project string) *Client {
	return &Client{sm: sm, project: project}
}

// secretName returns the GCP secret name for a given path.
func secretName(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--%s", path.Env, path.App, path.Key)
}

// secretParent returns the resource path for a secret.
func (c *Client) secretParent(name string) string {
	return fmt.Sprintf("projects/%s/secrets/%s", c.project, name)
}

// Get fetches the latest version of a secret. Returns ErrNotFound if absent.
func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	name := fmt.Sprintf("%s/versions/latest", c.secretParent(secretName(path)))
	resp, err := c.sm.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", provider.ErrNotFound
		}
		return "", fmt.Errorf("gcp get %s: %w", name, err)
	}
	return string(resp.Payload.Data), nil
}

// Set creates or updates a secret.
func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	name := secretName(path)
	parent := c.secretParent(name)

	_, err := c.sm.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: parent})
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return fmt.Errorf("gcp set check %s: %w", parent, err)
		}
		// Secret does not exist; create it.
		_, err = c.sm.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
			Parent:   fmt.Sprintf("projects/%s", c.project),
			SecretId: name,
			Secret: &secretmanagerpb.Secret{
				Replication: &secretmanagerpb.Replication{
					Replication: &secretmanagerpb.Replication_Automatic_{
						Automatic: &secretmanagerpb.Replication_Automatic{},
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("gcp create %s: %w", parent, err)
		}
	}

	_, err = c.sm.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  parent,
		Payload: &secretmanagerpb.SecretPayload{Data: []byte(value)},
	})
	if err != nil {
		return fmt.Errorf("gcp add version %s: %w", parent, err)
	}
	return nil
}

// Delete removes a secret. NotFound is treated as a no-op (idempotent).
func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	parent := c.secretParent(secretName(path))
	err := c.sm.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{Name: parent})
	if err != nil && status.Code(err) != codes.NotFound {
		return fmt.Errorf("gcp delete %s: %w", parent, err)
	}
	return nil
}

// List returns the keys under env/app by filtering on the name prefix.
func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	prefix := fmt.Sprintf("%s--%s--", path.Env, path.App)
	it := c.sm.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", c.project),
		Filter: fmt.Sprintf("name:%s", prefix),
	})

	var keys []string
	for {
		secret, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gcp list: %w", err)
		}
		// Full name is "projects/{proj}/secrets/{name}"; extract just the key.
		parts := strings.Split(secret.Name, "/")
		fullName := parts[len(parts)-1]
		key := strings.TrimPrefix(fullName, prefix)
		keys = append(keys, key)
	}
	return keys, nil
}

// Exists reports whether a secret exists.
func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	parent := c.secretParent(secretName(path))
	_, err := c.sm.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: parent})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("gcp exists %s: %w", parent, err)
	}
	return true, nil
}
