# Additional Providers Design: AWS, Azure, GCP, 1Password

## Goal

Extend kumokagi with four new backends — AWS Secrets Manager, Azure Key Vault, GCP Secret Manager, and 1Password CLI — while keeping the existing `provider.Provider` interface and CLI unchanged.

## Architecture

Approach: per-backend packages under `internal/`, a new `pkg/factory` that selects the right provider from `config.Backend`, and Python mirrors in `python/kumokagi/`.

```
internal/
  vault/          (existing)
  aws/client.go
  azure/client.go
  gcp/client.go
  onepassword/client.go
pkg/factory/
  factory.go
pkg/config/
  config.go       (extend — add AWSConfig, AzureConfig, GCPConfig, OnePasswordConfig)
python/kumokagi/
  aws.py
  azure.py
  gcp.py
  onepassword.py
  factory.py      (map backend string → Provider)
```

---

## Config Extension

`pkg/config/config.go` adds four optional backend blocks. `Mount` is repurposed per backend.

```go
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

type AWSConfig struct {
    Region string `yaml:"region"` // optional; falls back to AWS_DEFAULT_REGION / SDK chain
}

type AzureConfig struct {
    VaultURL string `yaml:"vault_url"` // e.g. https://myapp.vault.azure.net
}

type GCPConfig struct {
    Project string `yaml:"project"` // GCP project ID
}

type OnePasswordConfig struct{} // no config — op CLI uses local session
```

### Mount semantics per backend

| Backend       | Mount means              | Required config field            |
|---------------|--------------------------|----------------------------------|
| `vault`       | KV mount path            | `vault.address`                  |
| `aws`         | unused (region via SDK)  | `aws.region` optional            |
| `azure`       | Azure Key Vault URL      | `azure.vault_url` or `mount`     |
| `gcp`         | GCP project ID           | `gcp.project` or `mount`         |
| `onepassword` | 1Password vault name     | `mount` required                 |

### Example configs

```yaml
# AWS
backend: aws
app: myapp
env: prod
aws:
  region: ap-southeast-1   # optional

# Azure
backend: azure
mount: https://myapp.vault.azure.net
app: myapp
env: prod

# GCP
backend: gcp
mount: my-gcp-project
app: myapp
env: prod

# 1Password
backend: onepassword
mount: Development
app: myapp
env: prod
```

### Validate() additions

```
azure  → error if mount == "" && azure.vault_url == ""
gcp    → error if mount == "" && gcp.project == ""
onepassword → error if mount == ""
```

---

## Path Mapping Per Backend

### AWS Secrets Manager

- Secret name: `{env}/{app}/{key}` — AWS SM natively supports `/` in names
- Value stored as JSON envelope: `{"value":"s3cr3t"}` (consistent with Vault)
- Auth: AWS SDK default credential chain (env vars, IRSA, instance profile)
- SDK: `github.com/aws/aws-sdk-go-v2/service/secretsmanager`

| Operation | AWS call |
|-----------|----------|
| Get       | `GetSecretValue` → parse JSON → extract `value` |
| Set       | `CreateSecret` or `PutSecretValue` if already exists |
| Delete    | `DeleteSecret` with `ForceDeleteWithoutRecovery: true` |
| List      | `ListSecrets` + client-side filter on prefix `{env}/{app}/` |
| Exists    | `DescribeSecret` — nil result = false |

### Azure Key Vault

- Secret name: `{env}--{app}--{key}` (double-dash as separator; single hyphens allowed within components)
- Vault URL: from `azure.vault_url` or `mount`
- Auth: `azidentity.NewDefaultAzureCredential()` (Managed Identity, Azure CLI, env vars)
- SDK: `github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets`

| Operation | Azure call |
|-----------|------------|
| Get       | `GetSecret` |
| Set       | `SetSecret` |
| Delete    | `BeginDeleteSecret` then `PurgeDeletedSecret` (permanent) |
| List      | `NewListSecretPropertiesPager` + client-side filter on prefix `{env}--{app}--` |
| Exists    | `GetSecretProperties` — 404 = false |

### GCP Secret Manager

- Secret name: `{env}--{app}--{key}`
- Full resource: `projects/{project}/secrets/{env}--{app}--{key}`
- Project: from `gcp.project` or `mount`
- Auth: Application Default Credentials (`GOOGLE_APPLICATION_CREDENTIALS`, Workload Identity)
- SDK: `cloud.google.com/go/secretmanager/apiv1`

| Operation | GCP call |
|-----------|----------|
| Get       | `AccessSecretVersion` with `latest` suffix → extract payload |
| Set       | `CreateSecret` + `AddSecretVersion`; skip create if already exists |
| Delete    | `DeleteSecret` |
| List      | `ListSecrets` with filter `name:{env}--{app}--` |
| Exists    | `GetSecret` — NotFound = false |

### 1Password CLI

- Vault: `mount` field (1Password vault name)
- Item: `{env}--{app}` (one item per env+app)
- Field: `key`
- Read path: `op://{{mount}}/{{env}}--{{app}}/{{key}}`
- Auth: assumes `op signin` completed; exec fails fast if unauthenticated
- Binary: `op` must be in PATH

| Operation | op CLI call |
|-----------|-------------|
| Get       | `op read "op://{mount}/{env}--{app}/{key}"` |
| Set       | `op item edit "{env}--{app}" --vault={mount} '{key}={value}'` or `op item create` if item absent |
| Delete    | `op item edit` to remove field; `op item delete` if item becomes empty |
| List      | `op item list --vault={mount} --format=json` + filter items by name prefix `{env}--{app}` → return field labels |
| Exists    | `op item get "{env}--{app}" --vault={mount} --fields={key} --format=json` — exit code 1 = false |

---

## Factory

`pkg/factory/factory.go`:

```go
func New(ctx context.Context, cfg *config.Config) (provider.Provider, error) {
    switch cfg.Backend {
    case "vault":       return vault.New(ctx)
    case "aws":         return aws.New(ctx, cfg)
    case "azure":       return azure.New(ctx, cfg)
    case "gcp":         return gcp.New(ctx, cfg)
    case "onepassword": return onepassword.New(cfg)
    default:
        return nil, fmt.Errorf("unknown backend %q (valid: vault, aws, azure, gcp, onepassword)", cfg.Backend)
    }
}
```

`cmd/kumokagi/root.go` `loadConfig()`: replace `vault.New(ctx)` with `factory.New(ctx, cfg)`. No other CLI changes.

---

## Python

Four new provider classes, each implementing `Provider` ABC:

| File | Class | SDK |
|------|-------|-----|
| `python/kumokagi/aws.py` | `AWSProvider` | `boto3` |
| `python/kumokagi/azure.py` | `AzureProvider` | `azure-keyvault-secrets` + `azure-identity` |
| `python/kumokagi/gcp.py` | `GCPProvider` | `google-cloud-secret-manager` |
| `python/kumokagi/onepassword.py` | `OnePasswordProvider` | `subprocess` (`op` CLI) |

`python/kumokagi/factory.py`:

```python
def new_provider(cfg: Config) -> Provider:
    match cfg.backend:
        case "vault":       return VaultProvider(address=cfg.vault.address)
        case "aws":         return AWSProvider(cfg)
        case "azure":       return AzureProvider(cfg)
        case "gcp":         return GCPProvider(cfg)
        case "onepassword": return OnePasswordProvider(cfg)
        case _:
            raise ValueError(f"unknown backend {cfg.backend!r}")
```

`KumokagiSettingsSource` gains optional `provider` parameter — if omitted, builds from config via factory.

`python/pyproject.toml` adds optional dependency groups so users only install what they need:

```toml
[project.optional-dependencies]
aws   = ["boto3>=1.34"]
azure = ["azure-keyvault-secrets>=4.8", "azure-identity>=1.16"]
gcp   = ["google-cloud-secret-manager>=2.20"]
```

---

## Testing

Each backend tested with httptest mock (Go) or `responses`/`unittest.mock` (Python):

- AWS: mock `secretsmanager` HTTP API responses
- Azure: mock `azsecrets` HTTP API responses
- GCP: mock HTTP responses via httptest (Go) / responses (Python) — GCP Secret Manager exposes a REST transcoding layer
- 1Password: mock `subprocess.run` calls

Integration tests (build tag `integration`) require live credentials and are excluded from CI by default.

---

## Go Dependencies

```
github.com/aws/aws-sdk-go-v2/config
github.com/aws/aws-sdk-go-v2/service/secretsmanager
github.com/Azure/azure-sdk-for-go/sdk/azidentity
github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets
cloud.google.com/go/secretmanager/apiv1
google.golang.org/api/option
```

1Password uses only stdlib `os/exec` — no new Go dependency.
