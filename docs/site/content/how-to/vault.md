---
title: Use HashiCorp Vault
weight: 1
---

# Use HashiCorp Vault

## Prerequisites

- Vault running with KV v2 secrets engine mounted at `secret/`
- `VAULT_ADDR` pointing to your Vault instance
- `VAULT_TOKEN` set (or `vault login` completed)

## Config

```yaml
backend: vault
mount: secret
app: myapp
env: prod
vault:
  address: https://vault.example.com
```

## Store secrets

```bash
# Via CLI
kumokagi set db_password "s3cr3t"
kumokagi set api_key "abc123"

# Via Vault CLI directly
vault kv put secret/prod/myapp/db_password value="s3cr3t"
```

## Vault policy for applications

Grant read-only access to an application's secrets in one environment:

```hcl
path "secret/data/prod/myapp/*" {
  capabilities = ["read", "list"]
}
path "secret/metadata/prod/myapp/*" {
  capabilities = ["read", "list"]
}
```

## Vault Agent sidecar (Kubernetes)

For K8s workloads, use the Vault Agent Injector to obtain a Vault token automatically:

```yaml
# K8s deployment annotations
annotations:
  vault.hashicorp.com/agent-inject: "true"
  vault.hashicorp.com/role: "myapp"
  vault.hashicorp.com/agent-inject-token: "true"
```

The agent writes a token to `/home/vault/.vault-token`. The Vault SDK picks this up automatically.

## CI with GitHub Actions (OIDC)

No stored credentials — GitHub Actions authenticates via OIDC:

```yaml
- uses: hashicorp/vault-action@v3
  with:
    url: ${{ vars.VAULT_ADDR }}
    method: jwt
    role: github-ci
    exportToken: true
```

This sets `VAULT_TOKEN` for subsequent steps.
