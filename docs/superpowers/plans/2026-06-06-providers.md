# Additional Providers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add AWS Secrets Manager, Azure Key Vault, GCP Secret Manager, and 1Password CLI providers to kumokagi, each implementing the existing `provider.Provider` interface, plus a factory and Python mirrors.

**Architecture:** Four new packages under `internal/` mirror `internal/vault`. A new `pkg/factory` reads `config.Backend` and returns the right `provider.Provider`. The CLI's `loadConfig()` swaps `vault.New()` for `factory.New()`. Python mirrors each provider with native SDKs, plus a `factory.py`. No changes to the `provider.Provider` interface or CLI commands.

**Tech Stack:** Go 1.26, aws-sdk-go-v2, azure-sdk-for-go/keyvault/azsecrets, cloud.google.com/go/secretmanager, os/exec for 1Password; Python 3.14, boto3, azure-keyvault-secrets+azure-identity, google-cloud-secret-manager, subprocess.

---

## File Structure

### Go — new files
| File | Responsibility |
|------|----------------|
| `pkg/config/config.go` | Extend: add AWSConfig, AzureConfig, GCPConfig, OnePasswordConfig; update Validate() |
| `pkg/config/config_test.go` | Extend: add backend-specific validation test cases |
| `internal/aws/client.go` | AWS Secrets Manager — implements provider.Provider |
| `internal/aws/client_test.go` | Tests via secretsManagerAPI interface mock |
| `internal/azure/client.go` | Azure Key Vault — implements provider.Provider |
| `internal/azure/client_test.go` | Tests via httptest IPv4 mock + fake credential |
| `internal/gcp/client.go` | GCP Secret Manager — implements provider.Provider |
| `internal/gcp/client_test.go` | Tests via bufconn in-memory gRPC server |
| `internal/onepassword/client.go` | 1Password CLI — implements provider.Provider via os/exec |
| `internal/onepassword/client_test.go` | Tests via executor interface mock |
| `pkg/factory/factory.go` | Selects provider from config.Backend |
| `pkg/factory/factory_test.go` | Tests unknown backend error |
| `cmd/kumokagi/root.go` | Modify: replace vault.New() with factory.New(); change vaultClient type |

### Python — new files
| File | Responsibility |
|------|----------------|
| `python/kumokagi/config.py` | Extend: AWSConfig, AzureConfig, GCPConfig, OnePasswordConfig dataclasses |
| `python/kumokagi/aws.py` | AWSProvider — boto3 |
| `python/kumokagi/azure.py` | AzureProvider — azure-keyvault-secrets |
| `python/kumokagi/gcp.py` | GCPProvider — google-cloud-secret-manager |
| `python/kumokagi/onepassword.py` | OnePasswordProvider — subprocess |
| `python/kumokagi/factory.py` | new_provider(cfg) → Provider |
| `python/kumokagi/__init__.py` | Extend: add new exports |
| `python/pyproject.toml` | Extend: add optional dep groups aws, azure, gcp |
| `python/tests/test_aws.py` | boto3 Stubber mock |
| `python/tests/test_azure.py` | unittest.mock |
| `python/tests/test_gcp.py` | unittest.mock |
| `python/tests/test_onepassword.py` | unittest.mock subprocess |
| `python/tests/test_factory.py` | factory tests |

---

## Task 1: Config Extension

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `pkg/config/config_test.go`

- [ ] **Step 1: Write failing tests for new validation cases**

Add these cases to the existing `TestConfig_Validate` table in `pkg/config/config_test.go`:

```go
{
    name:    "azure missing vault url",
    cfg:     config.Config{Backend: "azure", App: "myapp", Env: "prod"},
    wantErr: "azure backend requires",
},
{
    name: "azure with mount ok",
    cfg:  config.Config{Backend: "azure", App: "myapp", Env: "prod", Mount: "https://myvault.vault.azure.net"},
},
{
    name: "azure with vault_url ok",
    cfg:  config.Config{Backend: "azure", App: "myapp", Env: "prod", Azure: config.AzureConfig{VaultURL: "https://myvault.vault.azure.net"}},
},
{
    name:    "gcp missing project",
    cfg:     config.Config{Backend: "gcp", App: "myapp", Env: "prod"},
    wantErr: "gcp backend requires",
},
{
    name: "gcp with mount ok",
    cfg:  config.Config{Backend: "gcp", App: "myapp", Env: "prod", Mount: "my-project"},
},
{
    name:    "onepassword missing mount",
    cfg:     config.Config{Backend: "onepassword", App: "myapp", Env: "prod"},
    wantErr: "onepassword backend requires",
},
{
    name: "onepassword with mount ok",
    cfg:  config.Config{Backend: "onepassword", App: "myapp", Env: "prod", Mount: "MyVault"},
},
```

- [ ] **Step 2: Run to verify fail**

```bash
go test -count=1 ./pkg/config/...
```

Expected: compile error — `config.AzureConfig` undefined.

- [ ] **Step 3: Extend config.go**

Replace the `Config` struct and add new types. Full file content:

```go
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
	Region string `yaml:"region"` // optional; falls back to AWS_DEFAULT_REGION
}

// AzureConfig holds Azure Key Vault config.
type AzureConfig struct {
	VaultURL string `yaml:"vault_url"` // e.g. https://myapp.vault.azure.net
}

// GCPConfig holds GCP Secret Manager config.
type GCPConfig struct {
	Project string `yaml:"project"` // GCP project ID
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
```

- [ ] **Step 4: Run tests — all pass**

```bash
go test -race -count=1 ./pkg/config/...
```

Expected: all tests pass including the 7 new validation cases.

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "feat: extend config with AWS/Azure/GCP/1Password backend structs"
```

---

## Task 2: AWS Provider

**Files:**
- Create: `internal/aws/client.go`
- Create: `internal/aws/client_test.go`

- [ ] **Step 1: Add Go dependencies**

```bash
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/service/secretsmanager@latest
go mod tidy
```

- [ ] **Step 2: Write failing tests**

```go
// internal/aws/client_test.go
package aws_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	awsprovider "github.com/stlimtat/kumokagi/internal/aws"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// mockSM is an in-memory mock for secretsManagerAPI.
type mockSM struct {
	secrets map[string]string // secret name -> JSON-encoded value
}

func (m *mockSM) GetSecretValue(_ context.Context, params *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	name := aws.ToString(params.SecretId)
	val, ok := m.secrets[name]
	if !ok {
		return nil, &types.ResourceNotFoundException{Message: aws.String("not found")}
	}
	return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(val)}, nil
}

func (m *mockSM) CreateSecret(_ context.Context, params *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	m.secrets[aws.ToString(params.Name)] = aws.ToString(params.SecretString)
	return &secretsmanager.CreateSecretOutput{}, nil
}

func (m *mockSM) PutSecretValue(_ context.Context, params *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	name := aws.ToString(params.SecretId)
	if _, ok := m.secrets[name]; !ok {
		return nil, &types.ResourceNotFoundException{Message: aws.String("not found")}
	}
	m.secrets[name] = aws.ToString(params.SecretString)
	return &secretsmanager.PutSecretValueOutput{}, nil
}

func (m *mockSM) DeleteSecret(_ context.Context, params *secretsmanager.DeleteSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	delete(m.secrets, aws.ToString(params.SecretId))
	return &secretsmanager.DeleteSecretOutput{}, nil
}

func (m *mockSM) DescribeSecret(_ context.Context, params *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	name := aws.ToString(params.SecretId)
	if _, ok := m.secrets[name]; !ok {
		return nil, &types.ResourceNotFoundException{Message: aws.String("not found")}
	}
	return &secretsmanager.DescribeSecretOutput{Name: aws.String(name)}, nil
}

func (m *mockSM) ListSecrets(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	var entries []types.SecretListEntry
	for name := range m.secrets {
		n := name
		entries = append(entries, types.SecretListEntry{Name: aws.String(n)})
	}
	return &secretsmanager.ListSecretsOutput{SecretList: entries}, nil
}

func newTestClient(secrets map[string]string) *awsprovider.Client {
	return awsprovider.NewWithClient(&mockSM{secrets: secrets})
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	t.Run("returns value for existing secret", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(map[string]string{"prod/myapp/db_password": `{"value":"s3cr3t"}`})
		val, err := c.Get(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", val)
	})
	t.Run("returns ErrNotFound for missing secret", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(map[string]string{})
		_, err := c.Get(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
		require.ErrorIs(t, err, provider.ErrNotFound)
	})
}

func TestClient_Set_Create(t *testing.T) {
	t.Parallel()
	c := newTestClient(map[string]string{})
	err := c.Set(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "newkey"}, "newval")
	require.NoError(t, err)
}

func TestClient_Set_Update(t *testing.T) {
	t.Parallel()
	c := newTestClient(map[string]string{"prod/myapp/existingkey": `{"value":"old"}`})
	err := c.Set(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "existingkey"}, "new")
	require.NoError(t, err)
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()
	c := newTestClient(map[string]string{"prod/myapp/db_password": `{"value":"s3cr3t"}`})
	require.NoError(t, c.Delete(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"}))
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	c := newTestClient(map[string]string{
		"prod/myapp/db_password": `{"value":"s3cr3t"}`,
		"prod/myapp/api_key":     `{"value":"abc"}`,
	})
	keys, err := c.List(t.Context(), provider.SecretPath{Env: "prod", App: "myapp"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"db_password", "api_key"}, keys)
}

func TestClient_Exists(t *testing.T) {
	t.Parallel()
	c := newTestClient(map[string]string{"prod/myapp/db_password": `{"value":"s3cr3t"}`})
	ok, err := c.Exists(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = c.Exists(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
	require.NoError(t, err)
	assert.False(t, ok)
}
```

- [ ] **Step 3: Run to verify fail**

```bash
go test -count=1 ./internal/aws/...
```

Expected: compile error — `aws` package does not exist.

- [ ] **Step 4: Implement client.go**

```go
// internal/aws/client.go
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
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// secretsManagerAPI is the subset of secretsmanager.Client methods we use.
type secretsManagerAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
	DescribeSecret(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
}

// Client implements provider.Provider for AWS Secrets Manager.
type Client struct {
	svc secretsManagerAPI
}

// New creates a Client using the AWS SDK default credential chain.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	region := cfg.AWS.Region
	if region == "" {
		region = cfg.Mount
	}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Client{svc: secretsmanager.NewFromConfig(awsCfg)}, nil
}

// NewWithClient creates a Client with an injected API (for testing).
func NewWithClient(svc secretsManagerAPI) *Client {
	return &Client{svc: svc}
}

func secretName(path provider.SecretPath) string {
	return fmt.Sprintf("%s/%s/%s", path.Env, path.App, path.Key)
}

func secretPrefix(path provider.SecretPath) string {
	return fmt.Sprintf("%s/%s/", path.Env, path.App)
}

func encodeValue(value string) string {
	b, _ := json.Marshal(map[string]string{"value": value})
	return string(b)
}

func decodeValue(raw string) (string, error) {
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	v, ok := m["value"]
	if !ok {
		return "", fmt.Errorf("secret has no 'value' field")
	}
	return v, nil
}

func isNotFound(err error) bool {
	var e *types.ResourceNotFoundException
	return errors.As(err, &e)
}

func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	name := secretName(path)
	out, err := c.svc.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
	if err != nil {
		if isNotFound(err) {
			return "", provider.ErrNotFound
		}
		return "", fmt.Errorf("aws get %s: %w", name, err)
	}
	if out.SecretString == nil {
		return "", provider.ErrNotFound
	}
	return decodeValue(*out.SecretString)
}

func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	name := secretName(path)
	encoded := encodeValue(value)
	_, err := c.svc.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(name),
		SecretString: aws.String(encoded),
	})
	if err != nil {
		if !isNotFound(err) {
			return fmt.Errorf("aws set %s: %w", name, err)
		}
		// Secret doesn't exist yet — create it.
		_, err = c.svc.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(name),
			SecretString: aws.String(encoded),
		})
		if err != nil {
			return fmt.Errorf("aws create %s: %w", name, err)
		}
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	name := secretName(path)
	force := true
	_, err := c.svc.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(name),
		ForceDeleteWithoutRecovery: &force,
	})
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("aws delete %s: %w", name, err)
	}
	return nil
}

func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	prefix := secretPrefix(path)
	out, err := c.svc.ListSecrets(ctx, &secretsmanager.ListSecretsInput{
		Filters: []types.Filter{{
			Key:    types.FilterNameStringTypeName,
			Values: []string{prefix},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("aws list %s: %w", prefix, err)
	}
	keys := make([]string, 0, len(out.SecretList))
	for _, s := range out.SecretList {
		if s.Name == nil {
			continue
		}
		name := *s.Name
		if strings.HasPrefix(name, prefix) {
			keys = append(keys, name[len(prefix):])
		}
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	name := secretName(path)
	_, err := c.svc.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{SecretId: aws.String(name)})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("aws exists %s: %w", name, err)
	}
	return true, nil
}
```

- [ ] **Step 5: Run tests — all pass**

```bash
go test -race -count=1 ./internal/aws/...
```

Expected: all 6 tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/aws/ go.mod go.sum
git commit -m "feat: add AWS Secrets Manager provider"
```

---

## Task 3: Azure Key Vault Provider

**Files:**
- Create: `internal/azure/client.go`
- Create: `internal/azure/client_test.go`

- [ ] **Step 1: Add dependencies**

```bash
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity@latest
go get github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets@latest
go mod tidy
```

- [ ] **Step 2: Write failing tests**

```go
// internal/azure/client_test.go
package azure_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	azureprovider "github.com/stlimtat/kumokagi/internal/azure"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type fakeCredential struct{}

func (f *fakeCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fake", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// GET /secrets/prod--myapp--db_password → 200
	mux.HandleFunc("/secrets/prod--myapp--db_password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"value": "s3cr3t",
				"id":    "https://test/secrets/prod--myapp--db_password/v1",
				"attributes": map[string]any{
					"enabled": true, "created": 1625000000, "updated": 1625000000, "recoveryLevel": "Purgeable",
				},
			})
		case http.MethodDelete:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"id":    "https://test/secrets/prod--myapp--db_password/v1",
				"value": "s3cr3t",
				"attributes": map[string]any{
					"enabled": true, "created": 1625000000, "updated": 1625000000, "recoveryLevel": "Purgeable",
				},
				"recoveryId":         "https://test/deletedSecrets/prod--myapp--db_password",
				"deletedDate":        1625000000,
				"scheduledPurgeDate": 1625000000,
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// POST /deletedSecrets/prod--myapp--db_password/purge → 204
	mux.HandleFunc("/deletedSecrets/prod--myapp--db_password/purge", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// GET /secrets/prod--myapp--missing → 404
	mux.HandleFunc("/secrets/prod--myapp--missing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "SecretNotFound", "message": "not found"},
		})
	})

	// PUT /secrets/prod--myapp--newkey → 200
	mux.HandleFunc("/secrets/prod--myapp--newkey", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"value": "newval",
			"id":    "https://test/secrets/prod--myapp--newkey/v1",
			"attributes": map[string]any{
				"enabled": true, "created": 1625000000, "updated": 1625000000,
			},
		})
	})

	// GET /secrets → list
	mux.HandleFunc("/secrets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{"id": "https://test/secrets/prod--myapp--db_password/", "attributes": map[string]any{"enabled": true}},
				{"id": "https://test/secrets/prod--myapp--api_key/", "attributes": map[string]any{"enabled": true}},
			},
			"nextLink": nil,
		})
	})

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T) *azureprovider.Client {
	t.Helper()
	srv := newMockServer(t)
	c, err := azureprovider.NewWithCredential(t.Context(), srv.URL, &fakeCredential{})
	require.NoError(t, err)
	return c
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	t.Run("existing", func(t *testing.T) {
		t.Parallel()
		val, err := c.Get(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", val)
	})
	t.Run("missing returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		_, err := c.Get(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
		require.ErrorIs(t, err, provider.ErrNotFound)
	})
}

func TestClient_Set(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	require.NoError(t, c.Set(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "newkey"}, "newval"))
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	require.NoError(t, c.Delete(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"}))
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	keys, err := c.List(t.Context(), provider.SecretPath{Env: "prod", App: "myapp"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"db_password", "api_key"}, keys)
}

func TestClient_Exists(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	ok, err := c.Exists(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = c.Exists(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
	require.NoError(t, err)
	assert.False(t, ok)
}
```

- [ ] **Step 3: Run to verify fail**

```bash
go test -count=1 ./internal/azure/...
```

Expected: compile error — `azure` package does not exist.

- [ ] **Step 4: Implement client.go**

```go
// internal/azure/client.go
package azure

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// Client implements provider.Provider for Azure Key Vault.
type Client struct {
	client *azsecrets.Client
}

// New creates a Client using DefaultAzureCredential.
func New(_ context.Context, cfg *config.Config) (*Client, error) {
	vaultURL := cfg.Mount
	if vaultURL == "" {
		vaultURL = cfg.Azure.VaultURL
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}
	return NewWithCredential(context.Background(), vaultURL, cred)
}

// NewWithCredential creates a Client with an explicit credential (for testing).
func NewWithCredential(_ context.Context, vaultURL string, cred azcore.TokenCredential) (*Client, error) {
	c, err := azsecrets.NewClient(vaultURL, cred, &azsecrets.ClientOptions{
		DisableChallengeResourceVerification: true,
	})
	if err != nil {
		return nil, fmt.Errorf("azure client: %w", err)
	}
	return &Client{client: c}, nil
}

func secretName(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--%s", path.Env, path.App, path.Key)
}

func secretPrefix(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--", path.Env, path.App)
}

func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == 404
}

func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	name := secretName(path)
	resp, err := c.client.GetSecret(ctx, name, "", nil)
	if err != nil {
		if isNotFound(err) {
			return "", provider.ErrNotFound
		}
		return "", fmt.Errorf("azure get %s: %w", name, err)
	}
	if resp.Value == nil {
		return "", provider.ErrNotFound
	}
	return *resp.Value, nil
}

func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	name := secretName(path)
	_, err := c.client.SetSecret(ctx, name, azsecrets.SetSecretParameters{Value: &value}, nil)
	if err != nil {
		return fmt.Errorf("azure set %s: %w", name, err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	name := secretName(path)
	poller, err := c.client.BeginDeleteSecret(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("azure delete %s: %w", name, err)
	}
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("azure delete poll %s: %w", name, err)
	}
	if _, err = c.client.PurgeDeletedSecret(ctx, name, nil); err != nil {
		return fmt.Errorf("azure purge %s: %w", name, err)
	}
	return nil
}

func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	prefix := secretPrefix(path)
	pager := c.client.NewListSecretPropertiesPager(nil)
	var keys []string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("azure list: %w", err)
		}
		for _, s := range page.Value {
			if s.ID == nil {
				continue
			}
			name := s.ID.Name()
			if strings.HasPrefix(name, prefix) {
				keys = append(keys, name[len(prefix):])
			}
		}
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	name := secretName(path)
	_, err := c.client.GetSecret(ctx, name, "", nil)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("azure exists %s: %w", name, err)
	}
	return true, nil
}
```

- [ ] **Step 5: Run tests — all pass**

```bash
go test -race -count=1 ./internal/azure/...
```

Expected: all 6 tests pass. If `NewWithCredential` receives a context.Context import issue, add `"context"` to imports.

- [ ] **Step 6: Commit**

```bash
git add internal/azure/ go.mod go.sum
git commit -m "feat: add Azure Key Vault provider"
```

---

## Task 4: GCP Secret Manager Provider

**Files:**
- Create: `internal/gcp/client.go`
- Create: `internal/gcp/client_test.go`

- [ ] **Step 1: Add dependencies**

```bash
go get cloud.google.com/go/secretmanager/apiv1@latest
go get google.golang.org/grpc@latest
go mod tidy
```

- [ ] **Step 2: Write failing tests**

The GCP client uses gRPC. Tests use `bufconn` (in-memory listener) to avoid real network calls.

```go
// internal/gcp/client_test.go
package gcp_test

import (
	"context"
	"net"
	"strings"
	"testing"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	gcpprovider "github.com/stlimtat/kumokagi/internal/gcp"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// fakeSecretManager implements SecretManagerServiceServer in memory.
type fakeSecretManager struct {
	secretmanagerpb.UnimplementedSecretManagerServiceServer
	secrets map[string]string // short name -> latest value
}

func (f *fakeSecretManager) AccessSecretVersion(_ context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	parts := strings.Split(req.Name, "/secrets/")
	if len(parts) != 2 {
		return nil, status.Error(codes.InvalidArgument, "bad name")
	}
	name := strings.Split(parts[1], "/")[0]
	val, ok := f.secrets[name]
	if !ok {
		return nil, status.Error(codes.NotFound, "not found")
	}
	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{Data: []byte(val)},
	}, nil
}

func (f *fakeSecretManager) CreateSecret(_ context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error) {
	name := req.SecretId
	f.secrets[name] = ""
	return &secretmanagerpb.Secret{Name: "projects/test/secrets/" + name}, nil
}

func (f *fakeSecretManager) AddSecretVersion(_ context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	parts := strings.Split(req.Parent, "/secrets/")
	if len(parts) != 2 {
		return nil, status.Error(codes.InvalidArgument, "bad parent")
	}
	name := parts[1]
	if _, ok := f.secrets[name]; !ok {
		return nil, status.Error(codes.NotFound, "secret not found")
	}
	f.secrets[name] = string(req.Payload.Data)
	return &secretmanagerpb.SecretVersion{Name: req.Parent + "/versions/1"}, nil
}

func (f *fakeSecretManager) DeleteSecret(_ context.Context, req *secretmanagerpb.DeleteSecretRequest) (*emptypb.Empty, error) {
	parts := strings.Split(req.Name, "/secrets/")
	if len(parts) != 2 {
		return nil, status.Error(codes.InvalidArgument, "bad name")
	}
	name := parts[1]
	delete(f.secrets, name)
	return &emptypb.Empty{}, nil
}

func (f *fakeSecretManager) GetSecret(_ context.Context, req *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
	parts := strings.Split(req.Name, "/secrets/")
	if len(parts) != 2 {
		return nil, status.Error(codes.InvalidArgument, "bad name")
	}
	name := parts[1]
	if _, ok := f.secrets[name]; !ok {
		return nil, status.Error(codes.NotFound, "not found")
	}
	return &secretmanagerpb.Secret{Name: req.Name}, nil
}

func (f *fakeSecretManager) ListSecrets(_ context.Context, req *secretmanagerpb.ListSecretsRequest) (*secretmanagerpb.ListSecretsResponse, error) {
	var secrets []*secretmanagerpb.Secret
	for name := range f.secrets {
		secrets = append(secrets, &secretmanagerpb.Secret{
			Name: fmt.Sprintf("projects/test/secrets/%s", name),
		})
	}
	return &secretmanagerpb.ListSecretsResponse{Secrets: secrets}, nil
}

func newTestClient(t *testing.T, fake *fakeSecretManager) *gcpprovider.Client {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	secretmanagerpb.RegisterSecretManagerServiceServer(s, fake)
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	smClient, err := secretmanager.NewClient(t.Context(), option.WithGRPCConn(conn))
	require.NoError(t, err)
	t.Cleanup(func() { smClient.Close() })

	return gcpprovider.NewWithClient(smClient, "test")
}

func makeSecrets() map[string]string {
	return map[string]string{
		"prod--myapp--db_password": "s3cr3t",
		"prod--myapp--api_key":     "abc123",
	}
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, &fakeSecretManager{secrets: makeSecrets()})
	t.Run("existing", func(t *testing.T) {
		t.Parallel()
		val, err := c.Get(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", val)
	})
	t.Run("missing returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		_, err := c.Get(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
		require.ErrorIs(t, err, provider.ErrNotFound)
	})
}

func TestClient_Set_Create(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, &fakeSecretManager{secrets: map[string]string{}})
	require.NoError(t, c.Set(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "newkey"}, "newval"))
}

func TestClient_Set_Update(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, &fakeSecretManager{secrets: map[string]string{"prod--myapp--existingkey": "old"}})
	require.NoError(t, c.Set(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "existingkey"}, "new"))
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, &fakeSecretManager{secrets: makeSecrets()})
	require.NoError(t, c.Delete(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"}))
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, &fakeSecretManager{secrets: makeSecrets()})
	keys, err := c.List(t.Context(), provider.SecretPath{Env: "prod", App: "myapp"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"db_password", "api_key"}, keys)
}

func TestClient_Exists(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, &fakeSecretManager{secrets: makeSecrets()})
	ok, err := c.Exists(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = c.Exists(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
	require.NoError(t, err)
	assert.False(t, ok)
}
```

Note: the test file needs `"fmt"` and `"google.golang.org/protobuf/types/known/emptypb"` imports.

- [ ] **Step 3: Run to verify fail**

```bash
go test -count=1 ./internal/gcp/...
```

Expected: compile error — `gcp` package does not exist.

- [ ] **Step 4: Implement client.go**

```go
// internal/gcp/client.go
package gcp

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// Client implements provider.Provider for GCP Secret Manager.
type Client struct {
	client  *secretmanager.Client
	project string
}

// New creates a Client using Application Default Credentials.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	project := cfg.GCP.Project
	if project == "" {
		project = cfg.Mount
	}
	if project == "" {
		return nil, fmt.Errorf("gcp: project ID required in gcp.project or mount")
	}
	c, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcp client: %w", err)
	}
	return &Client{client: c, project: project}, nil
}

// NewWithClient creates a Client with an injected gRPC client (for testing).
func NewWithClient(c *secretmanager.Client, project string) *Client {
	return &Client{client: c, project: project}
}

func secretResourceName(project string, path provider.SecretPath) string {
	return fmt.Sprintf("projects/%s/secrets/%s--%s--%s", project, path.Env, path.App, path.Key)
}

func secretShortName(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--%s", path.Env, path.App, path.Key)
}

func secretPrefix(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--", path.Env, path.App)
}

func isNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}

func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	name := secretResourceName(c.project, path) + "/versions/latest"
	result, err := c.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
	if err != nil {
		if isNotFound(err) {
			return "", provider.ErrNotFound
		}
		return "", fmt.Errorf("gcp get %s: %w", name, err)
	}
	return string(result.Payload.Data), nil
}

func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	parent := secretResourceName(c.project, path)
	shortName := secretShortName(path)
	payload := &secretmanagerpb.SecretPayload{Data: []byte(value)}

	_, err := c.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  parent,
		Payload: payload,
	})
	if err == nil {
		return nil
	}
	if !isNotFound(err) {
		return fmt.Errorf("gcp set %s: %w", shortName, err)
	}
	// Secret doesn't exist — create then add version.
	_, err = c.client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", c.project),
		SecretId: shortName,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gcp create %s: %w", shortName, err)
	}
	_, err = c.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  parent,
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("gcp add version %s: %w", shortName, err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	name := secretResourceName(c.project, path)
	err := c.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{Name: name})
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("gcp delete %s: %w", name, err)
	}
	return nil
}

func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	parent := fmt.Sprintf("projects/%s", c.project)
	prefix := secretPrefix(path)
	it := c.client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{Parent: parent})
	var keys []string
	for {
		secret, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gcp list: %w", err)
		}
		parts := strings.Split(secret.Name, "/secrets/")
		if len(parts) != 2 {
			continue
		}
		name := parts[1]
		if strings.HasPrefix(name, prefix) {
			keys = append(keys, name[len(prefix):])
		}
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	name := secretResourceName(c.project, path)
	_, err := c.client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: name})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("gcp exists %s: %w", name, err)
	}
	return true, nil
}
```

- [ ] **Step 5: Run tests — all pass**

```bash
go test -race -count=1 ./internal/gcp/...
```

Expected: all tests pass. Fix any import issues (add `"fmt"`, `"google.golang.org/protobuf/types/known/emptypb"` in test if needed).

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/ go.mod go.sum
git commit -m "feat: add GCP Secret Manager provider"
```

---

## Task 5: 1Password CLI Provider

**Files:**
- Create: `internal/onepassword/client.go`
- Create: `internal/onepassword/client_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/onepassword/client_test.go
package onepassword_test

import (
	"context"
	"encoding/json"
	"testing"

	opprovider "github.com/stlimtat/kumokagi/internal/onepassword"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// mockExecutor records calls and returns scripted responses.
type mockExecutor struct {
	// responses maps "arg0 arg1 ..." to (stdout, exitCode)
	responses map[string]mockResponse
}

type mockResponse struct {
	out  []byte
	code int
}

func (m *mockExecutor) Run(_ context.Context, args ...string) ([]byte, int, error) {
	key := ""
	for i, a := range args {
		if i > 0 {
			key += " "
		}
		key += a
	}
	if r, ok := m.responses[key]; ok {
		return r.out, r.code, nil
	}
	return nil, 1, nil
}

func listJSON(titles []string) []byte {
	items := make([]map[string]string, len(titles))
	for i, t := range titles {
		items[i] = map[string]string{"title": t}
	}
	b, _ := json.Marshal(items)
	return b
}

func newClient(responses map[string]mockResponse) *opprovider.Client {
	return opprovider.NewWithExecutor("MyVault", &mockExecutor{responses: responses})
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	t.Run("existing", func(t *testing.T) {
		t.Parallel()
		c := newClient(map[string]mockResponse{
			"read op://MyVault/prod--myapp--db_password/password": {out: []byte("s3cr3t\n"), code: 0},
		})
		val, err := c.Get(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", val)
	})
	t.Run("missing returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		c := newClient(map[string]mockResponse{})
		_, err := c.Get(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
		require.ErrorIs(t, err, provider.ErrNotFound)
	})
}

func TestClient_Set_Create(t *testing.T) {
	t.Parallel()
	c := newClient(map[string]mockResponse{
		// Exists check fails (item not found)
		"item get prod--myapp--newkey --vault=MyVault --format=json": {code: 1},
		// Create succeeds
		"item create --vault=MyVault --category=Login --title=prod--myapp--newkey password=newval": {code: 0},
	})
	require.NoError(t, c.Set(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "newkey"}, "newval"))
}

func TestClient_Set_Update(t *testing.T) {
	t.Parallel()
	c := newClient(map[string]mockResponse{
		"item get prod--myapp--existingkey --vault=MyVault --format=json": {out: []byte(`{}`), code: 0},
		"item edit prod--myapp--existingkey --vault=MyVault password=updated": {code: 0},
	})
	require.NoError(t, c.Set(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "existingkey"}, "updated"))
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()
	c := newClient(map[string]mockResponse{
		"item delete prod--myapp--db_password --vault=MyVault": {code: 0},
	})
	require.NoError(t, c.Delete(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"}))
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	c := newClient(map[string]mockResponse{
		"item list --vault=MyVault --format=json": {
			out:  listJSON([]string{"prod--myapp--db_password", "prod--myapp--api_key", "other--app--key"}),
			code: 0,
		},
	})
	keys, err := c.List(t.Context(), provider.SecretPath{Env: "prod", App: "myapp"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"db_password", "api_key"}, keys)
}

func TestClient_Exists(t *testing.T) {
	t.Parallel()
	c := newClient(map[string]mockResponse{
		"item get prod--myapp--db_password --vault=MyVault --format=json": {out: []byte(`{}`), code: 0},
		"item get prod--myapp--missing --vault=MyVault --format=json":     {code: 1},
	})
	ok, err := c.Exists(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = c.Exists(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
	require.NoError(t, err)
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run to verify fail**

```bash
go test -count=1 ./internal/onepassword/...
```

Expected: compile error — `onepassword` package does not exist.

- [ ] **Step 3: Implement client.go**

```go
// internal/onepassword/client.go
package onepassword

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// executor abstracts os/exec for testing.
type executor interface {
	Run(ctx context.Context, args ...string) ([]byte, int, error)
}

type defaultExecutor struct{}

func (e *defaultExecutor) Run(ctx context.Context, args ...string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, "op", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, exitErr.ExitCode(), nil
		}
		return nil, -1, err
	}
	return out, 0, nil
}

// Client implements provider.Provider using the 1Password CLI (op).
// Each secret maps to one 1Password item: title={env}--{app}--{key}, password field = value.
type Client struct {
	vault string
	exec  executor
}

// New creates a Client. cfg.Mount must be set to the 1Password vault name.
func New(cfg *config.Config) (*Client, error) {
	if cfg.Mount == "" {
		return nil, fmt.Errorf("onepassword: vault name required in mount field")
	}
	return &Client{vault: cfg.Mount, exec: &defaultExecutor{}}, nil
}

// NewWithExecutor creates a Client with an injected executor (for testing).
func NewWithExecutor(vault string, exec executor) *Client {
	return &Client{vault: vault, exec: exec}
}

func itemTitle(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--%s", path.Env, path.App, path.Key)
}

func itemPrefix(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s--", path.Env, path.App)
}

func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	ref := fmt.Sprintf("op://%s/%s/password", c.vault, itemTitle(path))
	out, code, err := c.exec.Run(ctx, "read", ref)
	if err != nil {
		return "", fmt.Errorf("op read %s: %w", ref, err)
	}
	if code != 0 {
		return "", provider.ErrNotFound
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	title := itemTitle(path)
	_, code, _ := c.exec.Run(ctx, "item", "get", title, "--vault="+c.vault, "--format=json")
	if code == 0 {
		_, _, err := c.exec.Run(ctx, "item", "edit", title, "--vault="+c.vault, "password="+value)
		if err != nil {
			return fmt.Errorf("op item edit %s: %w", title, err)
		}
		return nil
	}
	_, _, err := c.exec.Run(ctx, "item", "create",
		"--vault="+c.vault,
		"--category=Login",
		"--title="+title,
		"password="+value,
	)
	if err != nil {
		return fmt.Errorf("op item create %s: %w", title, err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	title := itemTitle(path)
	_, _, err := c.exec.Run(ctx, "item", "delete", title, "--vault="+c.vault)
	if err != nil {
		return fmt.Errorf("op item delete %s: %w", title, err)
	}
	return nil
}

type opItem struct {
	Title string `json:"title"`
}

func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	out, _, err := c.exec.Run(ctx, "item", "list", "--vault="+c.vault, "--format=json")
	if err != nil {
		return nil, fmt.Errorf("op item list: %w", err)
	}
	var items []opItem
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parse op item list: %w", err)
	}
	prefix := itemPrefix(path)
	var keys []string
	for _, item := range items {
		if strings.HasPrefix(item.Title, prefix) {
			key := item.Title[len(prefix):]
			if key != "" {
				keys = append(keys, key)
			}
		}
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	title := itemTitle(path)
	_, code, _ := c.exec.Run(ctx, "item", "get", title, "--vault="+c.vault, "--format=json")
	return code == 0, nil
}
```

- [ ] **Step 4: Run tests — all pass**

```bash
go test -race -count=1 ./internal/onepassword/...
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/onepassword/
git commit -m "feat: add 1Password CLI provider"
```

---

## Task 6: Factory + CLI Wiring

**Files:**
- Create: `pkg/factory/factory.go`
- Create: `pkg/factory/factory_test.go`
- Modify: `cmd/kumokagi/root.go`

- [ ] **Step 1: Write failing factory test**

```go
// pkg/factory/factory_test.go
package factory_test

import (
	"testing"

	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/factory"
	"github.com/stretchr/testify/require"
)

func TestNew_UnknownBackend(t *testing.T) {
	t.Parallel()
	_, err := factory.New(t.Context(), &config.Config{
		Backend: "unknown", App: "myapp", Env: "prod",
	})
	require.ErrorContains(t, err, "unknown backend")
	require.ErrorContains(t, err, "unknown")
}
```

- [ ] **Step 2: Run to verify fail**

```bash
go test -count=1 ./pkg/factory/...
```

Expected: compile error.

- [ ] **Step 3: Implement factory.go**

```go
// pkg/factory/factory.go
package factory

import (
	"context"
	"fmt"

	"github.com/stlimtat/kumokagi/internal/aws"
	"github.com/stlimtat/kumokagi/internal/azure"
	"github.com/stlimtat/kumokagi/internal/gcp"
	"github.com/stlimtat/kumokagi/internal/onepassword"
	"github.com/stlimtat/kumokagi/internal/vault"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// New creates a Provider for the backend named in cfg.Backend.
func New(ctx context.Context, cfg *config.Config) (provider.Provider, error) {
	switch cfg.Backend {
	case "vault":
		return vault.New(ctx)
	case "aws":
		return aws.New(ctx, cfg)
	case "azure":
		return azure.New(ctx, cfg)
	case "gcp":
		return gcp.New(ctx, cfg)
	case "onepassword":
		return onepassword.New(cfg)
	default:
		return nil, fmt.Errorf("unknown backend %q (valid: vault, aws, azure, gcp, onepassword)", cfg.Backend)
	}
}
```

- [ ] **Step 4: Run factory test — passes**

```bash
go test -race -count=1 ./pkg/factory/...
```

Expected: PASS.

- [ ] **Step 5: Update cmd/kumokagi/root.go**

Replace the entire file with:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/factory"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stlimtat/kumokagi/pkg/vipersource"
)

var (
	cfgFile     string
	appCfg      *config.Config
	vaultClient provider.Provider
	source      *vipersource.Source
)

var rootCmd = &cobra.Command{
	Use:           "kumokagi",
	Short:         "Ephemeral secrets management for cloud infrastructure",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", config.FileName, "config file path")
	viper.SetEnvPrefix("KUMOKAGI")
	viper.AutomaticEnv()
}

// loadConfig is called by commands that need the provider (all except init).
func loadConfig(ctx context.Context) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	appCfg = cfg

	p, err := factory.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	vaultClient = p
	source = vipersource.New(p, cfg)
	return nil
}
```

- [ ] **Step 6: Build and verify**

```bash
go build ./cmd/kumokagi/ && echo BUILD_OK && rm -f kumokagi
```

Expected: `BUILD_OK`

- [ ] **Step 7: Run full test suite**

```bash
go test -race -count=1 -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
```

Expected: all tests pass, coverage ≥ 70%.

- [ ] **Step 8: Commit**

```bash
git add pkg/factory/ cmd/kumokagi/root.go go.mod go.sum
git commit -m "feat: add provider factory and wire CLI to all backends"
```

---

## Task 7: Python Config Extension

**Files:**
- Modify: `python/kumokagi/config.py`
- Modify: `python/tests/test_config.py`

- [ ] **Step 1: Write failing tests**

Add these to `python/tests/test_config.py`:

```python
def test_validate_azure_missing_vault_url(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: azure\napp: myapp\nenv: prod\n")
    cfg = load_config(str(f))
    with pytest.raises(ValueError, match="azure"):
        cfg.validate()


def test_validate_azure_with_mount_ok(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: azure\napp: myapp\nenv: prod\nmount: https://vault.azure.net\n")
    cfg = load_config(str(f))
    cfg.validate()  # no error


def test_validate_gcp_missing_project(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: gcp\napp: myapp\nenv: prod\n")
    cfg = load_config(str(f))
    with pytest.raises(ValueError, match="gcp"):
        cfg.validate()


def test_validate_onepassword_missing_mount(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: onepassword\napp: myapp\nenv: prod\n")
    cfg = load_config(str(f))
    with pytest.raises(ValueError, match="onepassword"):
        cfg.validate()


def test_load_aws_config(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: aws\napp: myapp\nenv: prod\naws:\n  region: ap-southeast-1\n")
    cfg = load_config(str(f))
    assert cfg.aws.region == "ap-southeast-1"
```

- [ ] **Step 2: Run to verify fail**

```bash
cd python && python3 -m pytest tests/test_config.py -v -k "test_validate_azure or test_validate_gcp or test_validate_onepassword or test_load_aws"
```

Expected: AttributeError — `Config` has no `aws` attribute.

- [ ] **Step 3: Update config.py**

Replace the full file:

```python
from __future__ import annotations

import os
from dataclasses import dataclass, field

import yaml

ENV_VAR = "KUMOKAGI_ENV"
DEFAULT_MOUNT = "secret"
CONFIG_FILE = ".kumokagi.yaml"


@dataclass
class VaultConfig:
    address: str = ""


@dataclass
class AWSConfig:
    region: str = ""


@dataclass
class AzureConfig:
    vault_url: str = ""


@dataclass
class GCPConfig:
    project: str = ""


@dataclass
class OnePasswordConfig:
    pass


@dataclass
class Config:
    backend: str = ""
    mount: str = DEFAULT_MOUNT
    app: str = ""
    env: str = ""
    keys: list[str] = field(default_factory=list)
    vault: VaultConfig = field(default_factory=VaultConfig)
    aws: AWSConfig = field(default_factory=AWSConfig)
    azure: AzureConfig = field(default_factory=AzureConfig)
    gcp: GCPConfig = field(default_factory=GCPConfig)
    onepassword: OnePasswordConfig = field(default_factory=OnePasswordConfig)

    def validate(self) -> None:
        if not self.backend:
            raise ValueError("backend is required")
        if not self.app:
            raise ValueError("app is required")
        if not self.env:
            raise ValueError(f"env is required (set in config or {ENV_VAR})")
        if self.backend == "azure" and not self.mount and not self.azure.vault_url:
            raise ValueError("azure backend requires vault URL in mount or azure.vault_url")
        if self.backend == "gcp" and not self.mount and not self.gcp.project:
            raise ValueError("gcp backend requires project ID in mount or gcp.project")
        if self.backend == "onepassword" and not self.mount:
            raise ValueError("onepassword backend requires vault name in mount")


def load_config(path: str = CONFIG_FILE) -> Config:
    with open(path) as f:
        data = yaml.safe_load(f) or {}

    vault_data = data.get("vault", {}) or {}
    aws_data = data.get("aws", {}) or {}
    azure_data = data.get("azure", {}) or {}
    gcp_data = data.get("gcp", {}) or {}

    cfg = Config(
        backend=data.get("backend", ""),
        mount=data.get("mount", DEFAULT_MOUNT) or DEFAULT_MOUNT,
        app=data.get("app", ""),
        env=data.get("env", ""),
        keys=data.get("keys", []),
        vault=VaultConfig(address=vault_data.get("address", "")),
        aws=AWSConfig(region=aws_data.get("region", "")),
        azure=AzureConfig(vault_url=azure_data.get("vault_url", "")),
        gcp=GCPConfig(project=gcp_data.get("project", "")),
    )
    if env_val := os.getenv(ENV_VAR):
        cfg.env = env_val
    return cfg
```

- [ ] **Step 4: Run all config tests — pass**

```bash
cd python && python3 -m pytest tests/test_config.py -v
```

Expected: all 12 tests pass.

- [ ] **Step 5: Commit**

```bash
git add python/kumokagi/config.py python/tests/test_config.py
git commit -m "feat: extend Python config with AWS/Azure/GCP/1Password structs"
```

---

## Task 8: Python AWS Provider

**Files:**
- Modify: `python/pyproject.toml`
- Create: `python/kumokagi/aws.py`
- Create: `python/tests/test_aws.py`

- [ ] **Step 1: Add boto3 optional dependency to pyproject.toml**

```toml
[project.optional-dependencies]
aws   = ["boto3>=1.34"]
azure = ["azure-keyvault-secrets>=4.8", "azure-identity>=1.16"]
gcp   = ["google-cloud-secret-manager>=2.20"]
dev = [
    "pytest>=8.3.0",
    "responses>=0.25.0",
    "pytest-cov>=6.0.0",
    "boto3>=1.34",
    "moto[secretsmanager]>=5.0",
    "azure-keyvault-secrets>=4.8",
    "azure-identity>=1.16",
    "google-cloud-secret-manager>=2.20",
]
```

- [ ] **Step 2: Install dev deps**

```bash
cd python && python3 -m pip install -e ".[dev]"
```

- [ ] **Step 3: Write failing tests**

```python
# python/tests/test_aws.py
import json
import pytest
import boto3
from moto import mock_aws

from kumokagi.aws import AWSProvider
from kumokagi.config import AWSConfig, Config
from kumokagi.provider import SecretNotFoundError, SecretPath


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="", env="prod", app="myapp", key=key)


def make_cfg(region: str = "us-east-1") -> Config:
    return Config(backend="aws", app="myapp", env="prod", aws=AWSConfig(region=region))


@pytest.fixture
def aws_client():
    with mock_aws():
        yield AWSProvider(make_cfg())


def _create(region="us-east-1"):
    """Return a boto3 SM client inside mock_aws context."""
    return boto3.client("secretsmanager", region_name=region)


@mock_aws
def test_get_existing_secret():
    sm = _create()
    sm.create_secret(Name="prod/myapp/db_password", SecretString=json.dumps({"value": "s3cr3t"}))
    c = AWSProvider(make_cfg())
    assert c.get(make_path("db_password")) == "s3cr3t"


@mock_aws
def test_get_missing_raises():
    c = AWSProvider(make_cfg())
    with pytest.raises(SecretNotFoundError):
        c.get(make_path("missing"))


@mock_aws
def test_set_creates_new_secret():
    c = AWSProvider(make_cfg())
    c.set(make_path("newkey"), "newval")
    sm = _create()
    resp = sm.get_secret_value(SecretId="prod/myapp/newkey")
    assert json.loads(resp["SecretString"])["value"] == "newval"


@mock_aws
def test_set_updates_existing_secret():
    sm = _create()
    sm.create_secret(Name="prod/myapp/db_password", SecretString=json.dumps({"value": "old"}))
    c = AWSProvider(make_cfg())
    c.set(make_path("db_password"), "new")
    resp = sm.get_secret_value(SecretId="prod/myapp/db_password")
    assert json.loads(resp["SecretString"])["value"] == "new"


@mock_aws
def test_delete_secret():
    sm = _create()
    sm.create_secret(Name="prod/myapp/db_password", SecretString=json.dumps({"value": "s3cr3t"}))
    c = AWSProvider(make_cfg())
    c.delete(make_path("db_password"))


@mock_aws
def test_list_secrets():
    sm = _create()
    sm.create_secret(Name="prod/myapp/db_password", SecretString=json.dumps({"value": "a"}))
    sm.create_secret(Name="prod/myapp/api_key", SecretString=json.dumps({"value": "b"}))
    sm.create_secret(Name="prod/otherapp/key", SecretString=json.dumps({"value": "c"}))
    c = AWSProvider(make_cfg())
    keys = c.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


@mock_aws
def test_exists_true():
    sm = _create()
    sm.create_secret(Name="prod/myapp/db_password", SecretString=json.dumps({"value": "s3cr3t"}))
    c = AWSProvider(make_cfg())
    assert c.exists(make_path("db_password")) is True


@mock_aws
def test_exists_false():
    c = AWSProvider(make_cfg())
    assert c.exists(make_path("missing")) is False
```

- [ ] **Step 4: Run to verify fail**

```bash
cd python && python3 -m pytest tests/test_aws.py -v
```

Expected: `ModuleNotFoundError: No module named 'kumokagi.aws'`

- [ ] **Step 5: Implement aws.py**

```python
# python/kumokagi/aws.py
from __future__ import annotations

import json

import boto3
from botocore.exceptions import ClientError

from kumokagi.config import Config
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class AWSProvider(Provider):
    """AWS Secrets Manager provider. Uses AWS SDK default credential chain."""

    def __init__(self, cfg: Config) -> None:
        region = cfg.aws.region or cfg.mount or None
        self._client = boto3.client("secretsmanager", region_name=region)
        self._prefix_sep = "/"

    def _name(self, path: SecretPath) -> str:
        return f"{path.env}/{path.app}/{path.key}"

    def _prefix(self, path: SecretPath) -> str:
        return f"{path.env}/{path.app}/"

    def _encode(self, value: str) -> str:
        return json.dumps({"value": value})

    def _decode(self, raw: str) -> str:
        data = json.loads(raw)
        if "value" not in data:
            raise ValueError("secret has no 'value' field")
        return data["value"]

    def _is_not_found(self, e: ClientError) -> bool:
        return e.response["Error"]["Code"] == "ResourceNotFoundException"

    def get(self, path: SecretPath) -> str:
        name = self._name(path)
        try:
            resp = self._client.get_secret_value(SecretId=name)
        except ClientError as e:
            if self._is_not_found(e):
                raise SecretNotFoundError(name)
            raise
        return self._decode(resp["SecretString"])

    def set(self, path: SecretPath, value: str) -> None:
        name = self._name(path)
        encoded = self._encode(value)
        try:
            self._client.put_secret_value(SecretId=name, SecretString=encoded)
        except ClientError as e:
            if not self._is_not_found(e):
                raise
            self._client.create_secret(Name=name, SecretString=encoded)

    def delete(self, path: SecretPath) -> None:
        name = self._name(path)
        try:
            self._client.delete_secret(SecretId=name, ForceDeleteWithoutRecovery=True)
        except ClientError as e:
            if not self._is_not_found(e):
                raise

    def list(self, path: SecretPath) -> list[str]:
        prefix = self._prefix(path)
        paginator = self._client.get_paginator("list_secrets")
        keys: list[str] = []
        for page in paginator.paginate(Filters=[{"Key": "name", "Values": [prefix]}]):
            for s in page.get("SecretList", []):
                name = s["Name"]
                if name.startswith(prefix):
                    keys.append(name[len(prefix):])
        return keys

    def exists(self, path: SecretPath) -> bool:
        name = self._name(path)
        try:
            self._client.describe_secret(SecretId=name)
            return True
        except ClientError as e:
            if self._is_not_found(e):
                return False
            raise
```

- [ ] **Step 6: Run tests — all pass**

```bash
cd python && python3 -m pytest tests/test_aws.py -v
```

Expected: all 8 tests pass.

- [ ] **Step 7: Commit**

```bash
git add python/kumokagi/aws.py python/tests/test_aws.py python/pyproject.toml
git commit -m "feat: add Python AWS Secrets Manager provider"
```

---

## Task 9: Python Azure Provider

**Files:**
- Create: `python/kumokagi/azure.py`
- Create: `python/tests/test_azure.py`

- [ ] **Step 1: Write failing tests**

```python
# python/tests/test_azure.py
from unittest.mock import MagicMock, patch

import pytest

from kumokagi.azure import AzureProvider
from kumokagi.config import AzureConfig, Config
from kumokagi.provider import SecretNotFoundError, SecretPath


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="https://test.vault.azure.net", env="prod", app="myapp", key=key)


def make_cfg() -> Config:
    return Config(
        backend="azure", app="myapp", env="prod",
        mount="https://test.vault.azure.net",
    )


def secret_name(path: SecretPath) -> str:
    return f"{path.env}--{path.app}--{path.key}" if path.key else f"{path.env}--{path.app}--"


@pytest.fixture
def mock_kv():
    with patch("kumokagi.azure.SecretClient") as MockClient:
        client = MagicMock()
        MockClient.return_value = client
        yield client


def test_get_existing_secret(mock_kv):
    mock_kv.get_secret.return_value.value = "s3cr3t"
    c = AzureProvider(make_cfg())
    assert c.get(make_path("db_password")) == "s3cr3t"
    mock_kv.get_secret.assert_called_once_with("prod--myapp--db_password")


def test_get_missing_raises(mock_kv):
    from azure.core.exceptions import ResourceNotFoundError
    mock_kv.get_secret.side_effect = ResourceNotFoundError("not found")
    c = AzureProvider(make_cfg())
    with pytest.raises(SecretNotFoundError):
        c.get(make_path("missing"))


def test_set_secret(mock_kv):
    c = AzureProvider(make_cfg())
    c.set(make_path("newkey"), "newval")
    mock_kv.set_secret.assert_called_once_with("prod--myapp--newkey", "newval")


def test_delete_secret(mock_kv):
    mock_kv.begin_delete_secret.return_value.result.return_value = None
    c = AzureProvider(make_cfg())
    c.delete(make_path("db_password"))
    mock_kv.begin_delete_secret.assert_called_once_with("prod--myapp--db_password")
    mock_kv.purge_deleted_secret.assert_called_once_with("prod--myapp--db_password")


def test_list_secrets(mock_kv):
    props = [MagicMock(name="prod--myapp--db_password"), MagicMock(name="prod--myapp--api_key"), MagicMock(name="other--app--key")]
    mock_kv.list_properties_of_secrets.return_value = iter(props)
    c = AzureProvider(make_cfg())
    keys = c.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


def test_exists_true(mock_kv):
    mock_kv.get_secret.return_value.value = "s3cr3t"
    c = AzureProvider(make_cfg())
    assert c.exists(make_path("db_password")) is True


def test_exists_false(mock_kv):
    from azure.core.exceptions import ResourceNotFoundError
    mock_kv.get_secret.side_effect = ResourceNotFoundError("not found")
    c = AzureProvider(make_cfg())
    assert c.exists(make_path("missing")) is False
```

- [ ] **Step 2: Run to verify fail**

```bash
cd python && python3 -m pytest tests/test_azure.py -v
```

Expected: `ModuleNotFoundError: No module named 'kumokagi.azure'`

- [ ] **Step 3: Implement azure.py**

```python
# python/kumokagi/azure.py
from __future__ import annotations

import re

from azure.core.exceptions import ResourceNotFoundError
from azure.identity import DefaultAzureCredential
from azure.keyvault.secrets import SecretClient

from kumokagi.config import Config
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class AzureProvider(Provider):
    """Azure Key Vault provider. Uses DefaultAzureCredential."""

    def __init__(self, cfg: Config) -> None:
        vault_url = cfg.mount or cfg.azure.vault_url
        credential = DefaultAzureCredential()
        self._client = SecretClient(vault_url=vault_url, credential=credential)

    def _name(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--{path.key}"

    def _prefix(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--"

    def get(self, path: SecretPath) -> str:
        name = self._name(path)
        try:
            secret = self._client.get_secret(name)
        except ResourceNotFoundError:
            raise SecretNotFoundError(name)
        return secret.value

    def set(self, path: SecretPath, value: str) -> None:
        self._client.set_secret(self._name(path), value)

    def delete(self, path: SecretPath) -> None:
        name = self._name(path)
        poller = self._client.begin_delete_secret(name)
        poller.result()
        self._client.purge_deleted_secret(name)

    def list(self, path: SecretPath) -> list[str]:
        prefix = self._prefix(path)
        keys: list[str] = []
        for props in self._client.list_properties_of_secrets():
            if props.name and props.name.startswith(prefix):
                keys.append(props.name[len(prefix):])
        return keys

    def exists(self, path: SecretPath) -> bool:
        try:
            self._client.get_secret(self._name(path))
            return True
        except ResourceNotFoundError:
            return False
```

- [ ] **Step 4: Run tests — all pass**

```bash
cd python && python3 -m pytest tests/test_azure.py -v
```

Expected: all 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add python/kumokagi/azure.py python/tests/test_azure.py
git commit -m "feat: add Python Azure Key Vault provider"
```

---

## Task 10: Python GCP Provider

**Files:**
- Create: `python/kumokagi/gcp.py`
- Create: `python/tests/test_gcp.py`

- [ ] **Step 1: Write failing tests**

```python
# python/tests/test_gcp.py
from unittest.mock import MagicMock, patch

import pytest
from google.api_core.exceptions import NotFound
from google.cloud.secretmanager_v1.types import (
    AccessSecretVersionResponse,
    SecretPayload,
)

from kumokagi.config import Config, GCPConfig
from kumokagi.gcp import GCPProvider
from kumokagi.provider import SecretNotFoundError, SecretPath


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="my-project", env="prod", app="myapp", key=key)


def make_cfg() -> Config:
    return Config(backend="gcp", app="myapp", env="prod", mount="my-project")


@pytest.fixture
def mock_sm():
    with patch("kumokagi.gcp.SecretManagerServiceClient") as MockClient:
        client = MagicMock()
        MockClient.return_value = client
        yield client


def test_get_existing_secret(mock_sm):
    mock_sm.access_secret_version.return_value = AccessSecretVersionResponse(
        payload=SecretPayload(data=b"s3cr3t")
    )
    c = GCPProvider(make_cfg())
    assert c.get(make_path("db_password")) == "s3cr3t"
    mock_sm.access_secret_version.assert_called_once()


def test_get_missing_raises(mock_sm):
    mock_sm.access_secret_version.side_effect = NotFound("not found")
    c = GCPProvider(make_cfg())
    with pytest.raises(SecretNotFoundError):
        c.get(make_path("missing"))


def test_set_creates_new(mock_sm):
    mock_sm.add_secret_version.side_effect = NotFound("no secret")
    c = GCPProvider(make_cfg())
    c.set(make_path("newkey"), "newval")
    mock_sm.create_secret.assert_called_once()
    assert mock_sm.add_secret_version.call_count == 2


def test_set_updates_existing(mock_sm):
    c = GCPProvider(make_cfg())
    c.set(make_path("existingkey"), "newval")
    mock_sm.add_secret_version.assert_called_once()
    mock_sm.create_secret.assert_not_called()


def test_delete_secret(mock_sm):
    c = GCPProvider(make_cfg())
    c.delete(make_path("db_password"))
    mock_sm.delete_secret.assert_called_once()


def test_list_secrets(mock_sm):
    secrets = [
        MagicMock(name="projects/my-project/secrets/prod--myapp--db_password"),
        MagicMock(name="projects/my-project/secrets/prod--myapp--api_key"),
        MagicMock(name="projects/my-project/secrets/prod--otherapp--key"),
    ]
    mock_sm.list_secrets.return_value = iter(secrets)
    c = GCPProvider(make_cfg())
    keys = c.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


def test_exists_true(mock_sm):
    mock_sm.get_secret.return_value = MagicMock()
    c = GCPProvider(make_cfg())
    assert c.exists(make_path("db_password")) is True


def test_exists_false(mock_sm):
    mock_sm.get_secret.side_effect = NotFound("not found")
    c = GCPProvider(make_cfg())
    assert c.exists(make_path("missing")) is False
```

- [ ] **Step 2: Run to verify fail**

```bash
cd python && python3 -m pytest tests/test_gcp.py -v
```

Expected: `ModuleNotFoundError: No module named 'kumokagi.gcp'`

- [ ] **Step 3: Implement gcp.py**

```python
# python/kumokagi/gcp.py
from __future__ import annotations

import re

from google.api_core.exceptions import NotFound
from google.cloud.secretmanager_v1 import SecretManagerServiceClient
from google.cloud.secretmanager_v1.types import (
    AddSecretVersionRequest,
    CreateSecretRequest,
    DeleteSecretRequest,
    GetSecretRequest,
    ListSecretsRequest,
    Replication,
    Secret,
    SecretPayload,
    SecretVersion,
)

from kumokagi.config import Config
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class GCPProvider(Provider):
    """GCP Secret Manager provider. Uses Application Default Credentials."""

    def __init__(self, cfg: Config) -> None:
        self._project = cfg.gcp.project or cfg.mount
        self._client = SecretManagerServiceClient()

    def _resource(self, path: SecretPath) -> str:
        name = f"{path.env}--{path.app}--{path.key}"
        return f"projects/{self._project}/secrets/{name}"

    def _short_name(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--{path.key}"

    def _prefix(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--"

    def get(self, path: SecretPath) -> str:
        name = self._resource(path) + "/versions/latest"
        try:
            resp = self._client.access_secret_version(request={"name": name})
        except NotFound:
            raise SecretNotFoundError(name)
        return resp.payload.data.decode()

    def set(self, path: SecretPath, value: str) -> None:
        parent = self._resource(path)
        payload = SecretPayload(data=value.encode())
        try:
            self._client.add_secret_version(request={"parent": parent, "payload": payload})
        except NotFound:
            self._client.create_secret(request={
                "parent": f"projects/{self._project}",
                "secret_id": self._short_name(path),
                "secret": Secret(replication=Replication(automatic=Replication.Automatic())),
            })
            self._client.add_secret_version(request={"parent": parent, "payload": payload})

    def delete(self, path: SecretPath) -> None:
        try:
            self._client.delete_secret(request={"name": self._resource(path)})
        except NotFound:
            pass

    def list(self, path: SecretPath) -> list[str]:
        prefix = self._prefix(path)
        parent = f"projects/{self._project}"
        keys: list[str] = []
        for secret in self._client.list_secrets(request={"parent": parent}):
            # Extract short name from "projects/{p}/secrets/{name}"
            parts = secret.name.split("/secrets/")
            if len(parts) == 2 and parts[1].startswith(prefix):
                keys.append(parts[1][len(prefix):])
        return keys

    def exists(self, path: SecretPath) -> bool:
        try:
            self._client.get_secret(request={"name": self._resource(path)})
            return True
        except NotFound:
            return False
```

- [ ] **Step 4: Run tests — all pass**

```bash
cd python && python3 -m pytest tests/test_gcp.py -v
```

Expected: all 8 tests pass.

- [ ] **Step 5: Commit**

```bash
git add python/kumokagi/gcp.py python/tests/test_gcp.py
git commit -m "feat: add Python GCP Secret Manager provider"
```

---

## Task 11: Python 1Password Provider

**Files:**
- Create: `python/kumokagi/onepassword.py`
- Create: `python/tests/test_onepassword.py`

- [ ] **Step 1: Write failing tests**

```python
# python/tests/test_onepassword.py
import json
from unittest.mock import MagicMock, patch

import pytest

from kumokagi.config import Config
from kumokagi.onepassword import OnePasswordProvider
from kumokagi.provider import SecretNotFoundError, SecretPath


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="MyVault", env="prod", app="myapp", key=key)


def make_cfg() -> Config:
    return Config(backend="onepassword", app="myapp", env="prod", mount="MyVault")


def _run_mock(responses: dict[tuple, tuple]):
    """Returns a mock for subprocess.run keyed by args tuple."""
    def _run(args, **kwargs):
        key = tuple(args)
        if key in responses:
            stdout, returncode = responses[key]
            result = MagicMock()
            result.stdout = stdout
            result.returncode = returncode
            result.check_returncode = lambda: None if returncode == 0 else (_ for _ in ()).throw(Exception("nonzero"))
            return result
        result = MagicMock()
        result.stdout = b""
        result.returncode = 1
        result.check_returncode = MagicMock(side_effect=Exception("nonzero"))
        return result
    return _run


@pytest.fixture(autouse=True)
def patch_run():
    with patch("kumokagi.onepassword.subprocess.run") as mock:
        yield mock


def test_get_existing_secret(patch_run):
    patch_run.return_value = MagicMock(stdout=b"s3cr3t\n", returncode=0)
    c = OnePasswordProvider(make_cfg())
    assert c.get(make_path("db_password")) == "s3cr3t"
    args = patch_run.call_args[0][0]
    assert "op://MyVault/prod--myapp--db_password/password" in args


def test_get_missing_raises(patch_run):
    patch_run.return_value = MagicMock(stdout=b"", returncode=1)
    c = OnePasswordProvider(make_cfg())
    with pytest.raises(SecretNotFoundError):
        c.get(make_path("missing"))


def test_set_creates_new_item(patch_run):
    # First call (exists check) returns 1, second (create) returns 0
    patch_run.side_effect = [
        MagicMock(stdout=b"", returncode=1),   # get fails
        MagicMock(stdout=b"", returncode=0),   # create succeeds
    ]
    c = OnePasswordProvider(make_cfg())
    c.set(make_path("newkey"), "newval")
    create_args = patch_run.call_args_list[1][0][0]
    assert "--category=Login" in create_args
    assert "password=newval" in create_args


def test_set_updates_existing_item(patch_run):
    patch_run.side_effect = [
        MagicMock(stdout=b"{}", returncode=0),  # get succeeds
        MagicMock(stdout=b"", returncode=0),    # edit succeeds
    ]
    c = OnePasswordProvider(make_cfg())
    c.set(make_path("existingkey"), "updated")
    edit_args = patch_run.call_args_list[1][0][0]
    assert "item" in edit_args
    assert "edit" in edit_args


def test_delete_secret(patch_run):
    patch_run.return_value = MagicMock(stdout=b"", returncode=0)
    c = OnePasswordProvider(make_cfg())
    c.delete(make_path("db_password"))
    args = patch_run.call_args[0][0]
    assert "delete" in args
    assert "prod--myapp--db_password" in args


def test_list_secrets(patch_run):
    items = [
        {"title": "prod--myapp--db_password"},
        {"title": "prod--myapp--api_key"},
        {"title": "prod--otherapp--key"},
    ]
    patch_run.return_value = MagicMock(stdout=json.dumps(items).encode(), returncode=0)
    c = OnePasswordProvider(make_cfg())
    keys = c.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


def test_exists_true(patch_run):
    patch_run.return_value = MagicMock(stdout=b"{}", returncode=0)
    c = OnePasswordProvider(make_cfg())
    assert c.exists(make_path("db_password")) is True


def test_exists_false(patch_run):
    patch_run.return_value = MagicMock(stdout=b"", returncode=1)
    c = OnePasswordProvider(make_cfg())
    assert c.exists(make_path("missing")) is False
```

- [ ] **Step 2: Run to verify fail**

```bash
cd python && python3 -m pytest tests/test_onepassword.py -v
```

Expected: `ModuleNotFoundError`

- [ ] **Step 3: Implement onepassword.py**

```python
# python/kumokagi/onepassword.py
from __future__ import annotations

import json
import subprocess

from kumokagi.config import Config
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class OnePasswordProvider(Provider):
    """1Password CLI provider. Requires `op` in PATH and an active session."""

    def __init__(self, cfg: Config) -> None:
        self._vault = cfg.mount

    def _item_title(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--{path.key}"

    def _item_prefix(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--"

    def _run(self, args: list[str], check: bool = False) -> subprocess.CompletedProcess:
        return subprocess.run(args, capture_output=True, check=check)

    def get(self, path: SecretPath) -> str:
        ref = f"op://{self._vault}/{self._item_title(path)}/password"
        result = self._run(["op", "read", ref])
        if result.returncode != 0:
            raise SecretNotFoundError(ref)
        return result.stdout.rstrip(b"\n").decode()

    def set(self, path: SecretPath, value: str) -> None:
        title = self._item_title(path)
        exists = self._run(["op", "item", "get", title, f"--vault={self._vault}", "--format=json"])
        if exists.returncode == 0:
            self._run(["op", "item", "edit", title, f"--vault={self._vault}", f"password={value}"], check=True)
        else:
            self._run([
                "op", "item", "create",
                f"--vault={self._vault}",
                "--category=Login",
                f"--title={title}",
                f"password={value}",
            ], check=True)

    def delete(self, path: SecretPath) -> None:
        title = self._item_title(path)
        self._run(["op", "item", "delete", title, f"--vault={self._vault}"], check=True)

    def list(self, path: SecretPath) -> list[str]:
        result = self._run(["op", "item", "list", f"--vault={self._vault}", "--format=json"])
        items: list[dict] = json.loads(result.stdout)
        prefix = self._item_prefix(path)
        keys: list[str] = []
        for item in items:
            title = item.get("title", "")
            if title.startswith(prefix):
                key = title[len(prefix):]
                if key:
                    keys.append(key)
        return keys

    def exists(self, path: SecretPath) -> bool:
        title = self._item_title(path)
        result = self._run(["op", "item", "get", title, f"--vault={self._vault}", "--format=json"])
        return result.returncode == 0
```

- [ ] **Step 4: Run tests — all pass**

```bash
cd python && python3 -m pytest tests/test_onepassword.py -v
```

Expected: all 8 tests pass.

- [ ] **Step 5: Commit**

```bash
git add python/kumokagi/onepassword.py python/tests/test_onepassword.py
git commit -m "feat: add Python 1Password CLI provider"
```

---

## Task 12: Python Factory + __init__ Update

**Files:**
- Create: `python/kumokagi/factory.py`
- Create: `python/tests/test_factory.py`
- Modify: `python/kumokagi/__init__.py`

- [ ] **Step 1: Write failing test**

```python
# python/tests/test_factory.py
import pytest

from kumokagi.config import Config
from kumokagi.factory import new_provider
from kumokagi.provider import SecretNotFoundError


def test_unknown_backend_raises():
    cfg = Config(backend="unknown", app="myapp", env="prod")
    with pytest.raises(ValueError, match="unknown"):
        new_provider(cfg)
```

- [ ] **Step 2: Run to verify fail**

```bash
cd python && python3 -m pytest tests/test_factory.py -v
```

Expected: `ModuleNotFoundError`

- [ ] **Step 3: Implement factory.py**

```python
# python/kumokagi/factory.py
from __future__ import annotations

from kumokagi.config import Config
from kumokagi.provider import Provider


def new_provider(cfg: Config) -> Provider:
    """Create a Provider for the backend named in cfg.backend."""
    match cfg.backend:
        case "vault":
            from kumokagi.vault import VaultProvider
            return VaultProvider(address=cfg.vault.address)
        case "aws":
            from kumokagi.aws import AWSProvider
            return AWSProvider(cfg)
        case "azure":
            from kumokagi.azure import AzureProvider
            return AzureProvider(cfg)
        case "gcp":
            from kumokagi.gcp import GCPProvider
            return GCPProvider(cfg)
        case "onepassword":
            from kumokagi.onepassword import OnePasswordProvider
            return OnePasswordProvider(cfg)
        case _:
            raise ValueError(
                f"unknown backend {cfg.backend!r} (valid: vault, aws, azure, gcp, onepassword)"
            )
```

- [ ] **Step 4: Update __init__.py**

```python
# python/kumokagi/__init__.py
from kumokagi.config import (
    AWSConfig,
    AzureConfig,
    Config,
    GCPConfig,
    OnePasswordConfig,
    VaultConfig,
    load_config,
)
from kumokagi.factory import new_provider
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath
from kumokagi.settings import KumokagiSettingsSource

__all__ = [
    "Config",
    "VaultConfig",
    "AWSConfig",
    "AzureConfig",
    "GCPConfig",
    "OnePasswordConfig",
    "load_config",
    "new_provider",
    "Provider",
    "SecretNotFoundError",
    "SecretPath",
    "KumokagiSettingsSource",
]
```

- [ ] **Step 5: Run full Python test suite**

```bash
cd python && python3 -m pytest tests/ -v --tb=short
```

Expected: all tests pass.

- [ ] **Step 6: Check coverage**

```bash
cd python && python3 -m pytest tests/ --cov=kumokagi --cov-report=term-missing | grep TOTAL
```

Expected: ≥ 70%.

- [ ] **Step 7: Run full Go test suite**

```bash
go test -race -count=1 -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
```

Expected: all tests pass, ≥ 70%.

- [ ] **Step 8: Commit**

```bash
git add python/kumokagi/factory.py python/kumokagi/__init__.py python/tests/test_factory.py
git commit -m "feat: add Python provider factory and update exports"
```

---

## Self-Review

### Spec coverage
- ✅ Config extension (AWS/Azure/GCP/1Password structs, Validate) → Task 1, Task 7
- ✅ AWS Secrets Manager → Task 2, Task 8
- ✅ Azure Key Vault → Task 3, Task 9
- ✅ GCP Secret Manager → Task 4, Task 10
- ✅ 1Password CLI → Task 5, Task 11
- ✅ Factory Go → Task 6
- ✅ Factory Python → Task 12
- ✅ CLI wiring (vault.New → factory.New) → Task 6
- ✅ Double-dash encoding for Azure/GCP → Tasks 3, 4, 9, 10
- ✅ AWS uses `/` path natively → Tasks 2, 8
- ✅ 1Password uses `op://vault/env--app--key/password` → Tasks 5, 11
- ✅ Python optional dep groups (aws/azure/gcp) → Task 8

### Placeholder scan: none found.

### Type consistency
- Go: `vaultClient provider.Provider` in root.go matches factory return type ✅
- Python: `new_provider` returns `Provider` ABC ✅
- `secretName` / `itemTitle` helper functions consistent within each package ✅
