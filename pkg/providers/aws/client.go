package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/factory"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

func init() {
	factory.Register("aws", func(ctx context.Context, cfg *config.Config) (provider.Provider, error) {
		return New(ctx, cfg)
	})
}

// Client implements provider.Provider for AWS Secrets Manager.
type Client struct {
	svc *secretsmanager.Client
}

// New creates an AWS SM client using default credential chain.
// cfg.AWS.Region overrides the region; empty falls back to SDK chain.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if cfg.AWS.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.AWS.Region))
	}
	awscfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Client{svc: secretsmanager.NewFromConfig(awscfg)}, nil
}

// NewWithSvc creates a Client with an injected secretsmanager.Client (for tests).
func NewWithSvc(svc *secretsmanager.Client) *Client {
	return &Client{svc: svc}
}

// secretName returns the AWS SM secret name for a path.
func secretName(path provider.SecretPath) string {
	return fmt.Sprintf("%s/%s/%s", path.Env, path.App, path.Key)
}

// secretPrefix returns the listing prefix for env/app.
func secretPrefix(path provider.SecretPath) string {
	return fmt.Sprintf("%s/%s/", path.Env, path.App)
}

type envelope struct {
	Value string `json:"value"`
}

func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	out, err := c.svc.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName(path)),
	})
	if err != nil {
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return "", provider.ErrNotFound
		}
		return "", fmt.Errorf("aws get %s: %w", secretName(path), err)
	}
	var env envelope
	if err := json.Unmarshal([]byte(aws.ToString(out.SecretString)), &env); err != nil {
		return "", fmt.Errorf("aws get %s: parse envelope: %w", secretName(path), err)
	}
	return env.Value, nil
}

func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	payload, err := json.Marshal(envelope{Value: value})
	if err != nil {
		return fmt.Errorf("aws set %s: marshal: %w", secretName(path), err)
	}
	secret := aws.String(string(payload))
	name := secretName(path)

	_, err = c.svc.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretString: secret,
	})
	if err != nil {
		var exists *types.ResourceExistsException
		if !errors.As(err, &exists) {
			return fmt.Errorf("aws set %s: create: %w", name, err)
		}
		// Secret already exists — update instead.
		if _, err = c.svc.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
			SecretId:     aws.String(name),
			SecretString: secret,
		}); err != nil {
			return fmt.Errorf("aws set %s: update: %w", name, err)
		}
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	_, err := c.svc.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretName(path)),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("aws delete %s: %w", secretName(path), err)
	}
	return nil
}

func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	prefix := secretPrefix(path)
	paginator := secretsmanager.NewListSecretsPaginator(c.svc, &secretsmanager.ListSecretsInput{})
	var keys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("aws list %s: %w", prefix, err)
		}
		for _, s := range page.SecretList {
			name := aws.ToString(s.Name)
			if strings.HasPrefix(name, prefix) {
				keys = append(keys, strings.TrimPrefix(name, prefix))
			}
		}
	}
	if keys == nil {
		return []string{}, nil
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	_, err := c.svc.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName(path)),
	})
	if err != nil {
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("aws exists %s: %w", secretName(path), err)
	}
	return true, nil
}
