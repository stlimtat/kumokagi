# kumokagi 雲鍵

[![CI](https://github.com/stlimtat/kumokagi/actions/workflows/ci.yml/badge.svg)](https://github.com/stlimtat/kumokagi/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/stlimtat/kumokagi.svg)](https://pkg.go.dev/github.com/stlimtat/kumokagi)
[![Go Version](https://img.shields.io/badge/go-1.26+-blue.svg)](https://go.dev/doc/install)
[![codecov](https://codecov.io/gh/stlimtat/kumokagi/branch/master/graph/badge.svg)](https://codecov.io/gh/stlimtat/kumokagi)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

<p align="center">
  <img src="docs/site/static/img/logo.svg" alt="kumokagi — cloud key" width="200"/>
</p>

> Ephemeral secrets management for Go and Python — one identity, any cloud.
> Fetch secrets at runtime from Vault, AWS, Azure, GCP, or 1Password without ever writing them to disk, environment variables, or container manifests.

---

## The Problem

| Problem | Common approach | kumokagi |
|---------|----------------|----------|
| Secrets in env vars | `export DB_PASS=s3cr3t` in `.env` | Never touches env vars — fetched into process memory only |
| Secrets in manifests | K8s `Secret` YAML committed to git | Config contains only backend name + path, zero secret values |
| Credential rotation | Restart app, redeploy pods | Next process start picks up new value automatically |
| Multi-cloud | Different SDK per backend, bespoke glue | Single interface — swap backend by changing one config line |
| Identity sprawl | Long-lived API keys per service | Ambient credentials only: IRSA, Workload Identity, `op signin` |
| Language lock-in | Go library, no Python equivalent (or vice versa) | Identical interface and config in both languages |

---

## Supported Backends

| Backend | Auth | Config key |
|---------|------|------------|
| HashiCorp Vault (KV v2) | `VAULT_TOKEN` | `vault` |
| AWS Secrets Manager | IRSA / default chain | `aws` |
| Azure Key Vault | DefaultAzureCredential / Workload Identity | `azure` |
| GCP Secret Manager | Application Default Credentials / WIF | `gcp` |
| 1Password CLI | `op signin` | `onepassword` |

---

## Quickstart

```bash
# install CLI
go install github.com/stlimtat/kumokagi/cmd/kumokagi@latest

# create config (Vault example)
kumokagi init --app myapp --env prod --backend vault

# store a secret
echo -n "s3cr3t" | kumokagi set db_password -

# fetch it
kumokagi get db_password

# fetch by URI (no config file needed)
kumokagi read kumokagi://vault/secret/prod/myapp/db_password
```

Full walkthrough: [stlimtat.github.io/kumokagi/quickstart/](https://stlimtat.github.io/kumokagi/quickstart/)

---

## CLI

```bash
kumokagi [--config .kumokagi.yaml] [--backend vault|aws|azure|gcp|onepassword] <command>
```

| Command | What it does |
|---------|--------------|
| `init` | Create `.kumokagi.yaml` for this app |
| `get <key>` | Fetch a secret value |
| `set <key> [value\|-]` | Store a secret (pass `-` or omit value to read from stdin) |
| `delete <key>` | Permanently delete a secret |
| `list` | List all keys for this app |
| `verify` | Check all declared keys exist in the backend |
| `prune [--force]` | Delete all keys not in the config's `keys:` list |
| `rotate <key> [--length 32] [--show]` | Generate and store a new random secret |
| `read <uri>` | Fetch a secret by `kumokagi://` URI |

### URI addressing

Encode backend, mount, env, app, and key in a single URI:

```
kumokagi://vault/secret/prod/myapp/db_password
kumokagi://aws//prod/myapp/db_password
kumokagi://onepassword/Private/prod/myapp/api_key
```

---

## Library Usage — Go

```go
import (
    "github.com/stlimtat/kumokagi/pkg/config"
    "github.com/stlimtat/kumokagi/pkg/factory"
    "github.com/stlimtat/kumokagi/pkg/vipersource"
    _ "github.com/stlimtat/kumokagi/pkg/factory/all" // register all backends
)

cfg, _ := config.Load(".kumokagi.yaml")
p, _   := factory.New(ctx, cfg)

// Use directly
val, _ := p.Get(ctx, provider.SecretPath{Mount: "secret", Env: "prod", App: "myapp", Key: "db_password"})

// Or wire into viper
source := vipersource.New(p, cfg)
source.Load(ctx, viper.GetViper())
password := viper.GetString("db_password")
```

Import only what you need — `database/sql` driver pattern:

```go
import _ "github.com/stlimtat/kumokagi/pkg/providers/aws"   // links only AWS SDK
import _ "github.com/stlimtat/kumokagi/pkg/providers/vault" // links only Vault SDK
```

---

## Library Usage — Python

```bash
pip install kumokagi           # Vault only
pip install "kumokagi[aws]"    # + boto3
pip install "kumokagi[azure]"  # + azure-keyvault-secrets
pip install "kumokagi[gcp]"    # + google-cloud-secret-manager
```

```python
from pydantic_settings import BaseSettings
from kumokagi.config import load_config
from kumokagi.settings import KumokagiSettingsSource

cfg = load_config(".kumokagi.yaml")

class AppSecrets(BaseSettings):
    db_password: str = ""
    api_key: str = ""

    @classmethod
    def settings_customise_sources(cls, settings_cls, **kwargs):
        return (KumokagiSettingsSource(settings_cls, config=cfg),)

secrets = AppSecrets()
# secrets.db_password is fetched from backend, never touches os.environ
```

---

## Design Principles

1. **Never persist secrets** — values exist only in process memory during use.
2. **Ambient credentials only** — kumokagi never stores or manages credentials; it delegates entirely to the backend SDK's default chain (IRSA, Workload Identity, `op signin`).
3. **Single config, no secrets** — `.kumokagi.yaml` contains only backend name, app name, and paths. Zero secret values.
4. **Rotation is free** — secrets are fetched on every startup. Rotate in the backend; the next deploy picks up the new value.
5. **Backend agnostic** — swap the backend by changing one line in config. No application code changes.

---

## Documentation

**[stlimtat.github.io/kumokagi](https://stlimtat.github.io/kumokagi/)**

- [Quickstart](https://stlimtat.github.io/kumokagi/quickstart/)
- [Concepts](https://stlimtat.github.io/kumokagi/concepts/)
- [Architecture](https://stlimtat.github.io/kumokagi/architecture/)
- [Examples](https://stlimtat.github.io/kumokagi/examples/)
  - [Vault](https://stlimtat.github.io/kumokagi/examples/vault/)
  - [AWS Secrets Manager](https://stlimtat.github.io/kumokagi/examples/aws/)
  - [Azure Key Vault](https://stlimtat.github.io/kumokagi/examples/azure/)
  - [GCP Secret Manager](https://stlimtat.github.io/kumokagi/examples/gcp/)
  - [1Password](https://stlimtat.github.io/kumokagi/examples/onepassword/)
  - [Startup MVP Guide](https://stlimtat.github.io/kumokagi/examples/startup-mvp/)
- How-to guides: [Vault](https://stlimtat.github.io/kumokagi/how-to/vault/) · [AWS](https://stlimtat.github.io/kumokagi/how-to/aws/) · [Azure](https://stlimtat.github.io/kumokagi/how-to/azure/) · [GCP](https://stlimtat.github.io/kumokagi/how-to/gcp/) · [1Password](https://stlimtat.github.io/kumokagi/how-to/onepassword/)

---

## License

[MIT](LICENSE)
