---
aliases: ["/examples/vault/"]
title: HashiCorp Vault
weight: 1
---

# HashiCorp Vault

Vault is kumokagi's first and most capable backend ([ADR 0004]({{< relref "/adrs/0004-vault-v1-backend" >}})): self-hosted, cloud-neutral, with versioned KV v2 storage and auth methods for humans (LDAP/OIDC), CI (JWT), and Kubernetes workloads.

## How authentication works

![Vault Kubernetes authentication flow](/img/auth-vault.png)

A Kubernetes pod logs in to Vault's [kubernetes auth method](https://developer.hashicorp.com/vault/docs/auth/kubernetes) with its service-account JWT and receives a short-lived `VAULT_TOKEN` bound to a policy. The Vault SDK picks the token up from the environment or `~/.vault-token`; kumokagi just uses the SDK. Humans get the same token via `vault login`; CI gets it via OIDC.

Official documentation:

- [Vault KV v2 secrets engine](https://developer.hashicorp.com/vault/docs/secrets/kv/kv-v2)
- [Kubernetes auth method](https://developer.hashicorp.com/vault/docs/auth/kubernetes)
- [Vault Agent Injector](https://developer.hashicorp.com/vault/docs/platform/k8s/injector)
- [JWT/OIDC auth for CI](https://developer.hashicorp.com/vault/docs/auth/jwt)

## Prerequisites

- Vault running with the KV v2 secrets engine mounted at `secret/`
- `VAULT_ADDR` pointing to your Vault instance
- `VAULT_TOKEN` set (or `vault login` completed)

## Config

```yaml
backend: vault
mount: secret
app: myapp
env: prod
keys: [db_password, api_key]
vault:
  address: https://vault.example.com
```

## Store secrets

```bash
# Via kumokagi
kumokagi set db_password "s3cr3t"

# Or via the Vault CLI directly
vault kv put secret/prod/myapp/db_password value="s3cr3t"
```

## Use in Go

```go
import _ "github.com/stlimtat/kumokagi/pkg/providers/vault" // links only the Vault SDK

cfg, _ := config.Load(".kumokagi.yaml")
provider, _ := factory.New(ctx, cfg)
source := vipersource.New(provider, cfg)
source.Load(ctx, viper.GetViper())

password := viper.GetString("db_password")
```

## Use in Python

```python
from pydantic_settings import BaseSettings
from kumokagi.config import load_config
from kumokagi.settings import KumokagiSettingsSource

cfg = load_config(".kumokagi.yaml")

class AppSecrets(BaseSettings):
    db_password: str = ""

    @classmethod
    def settings_customise_sources(cls, settings_cls, **kwargs):
        return (KumokagiSettingsSource(settings_cls, config=cfg),)

secrets = AppSecrets()
```

## Least-privilege Vault policy

Environment-first paths ([ADR 0002]({{< relref "/adrs/0002-env-first-path-convention" >}})) make the prod boundary one rule:

```hcl
path "secret/data/prod/myapp/*" {
  capabilities = ["read", "list"]
}
path "secret/metadata/prod/myapp/*" {
  capabilities = ["read", "list"]
}
```

## Kubernetes with the Vault Agent Injector

The injector obtains a Vault token for the pod automatically:

```yaml
# K8s deployment annotations
annotations:
  vault.hashicorp.com/agent-inject: "true"
  vault.hashicorp.com/role: "myapp"
  vault.hashicorp.com/agent-inject-token: "true"
```

The agent writes a token to `/home/vault/.vault-token`; the Vault SDK picks it up automatically.

## CI with GitHub Actions (OIDC)

```yaml
- uses: hashicorp/vault-action@v3
  with:
    url: ${{ vars.VAULT_ADDR }}
    method: jwt
    role: github-ci
    exportToken: true
```

This sets `VAULT_TOKEN` for subsequent steps — no stored credentials.

## Verify and troubleshoot access

Run these **from inside the pod** (`kubectl exec -it deploy/myapp -- sh`); the first failing step tells you which layer is broken.

**1. Is Vault reachable and does the pod hold a token?**

```bash
echo $VAULT_ADDR
vault status                 # seal state, reachability
vault token lookup           # who am I, which policies, TTL
```

`connection refused` → network/`VAULT_ADDR` problem. `missing client token` → the agent injector annotations are absent or the token file path is not where the SDK looks.

**2. Does the token's policy cover the path?**

```bash
vault token capabilities secret/data/prod/myapp/db_password
```

Expected: `read`. `deny` → the role's policy does not include the path — remember KV v2 inserts `data/` between mount and path.

**3. Can it read the secret directly?**

```bash
vault kv get secret/prod/myapp/db_password
```

`No value found` → the secret was written to a different mount or env; list what exists: `vault kv list secret/prod/myapp/`.

**4. Does kumokagi resolve it end-to-end?**

```bash
kumokagi verify
kumokagi read kumokagi://vault/secret/prod/myapp/db_password
```

If steps 1–3 pass and this fails, check `.kumokagi.yaml`: `mount`, `env` (and any `KUMOKAGI_ENV` override), and `app`.
