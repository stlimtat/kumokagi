---
aliases: ["/concepts/providers/"]
title: Providers
weight: 2
---

# Providers

A **Provider** is a kumokagi implementation of a specific secrets backend. All providers implement the same interface:

```go
type Provider interface {
    Get(ctx context.Context, path SecretPath) (string, error)
    Set(ctx context.Context, path SecretPath, value string) error
    Delete(ctx context.Context, path SecretPath) error
    List(ctx context.Context, path SecretPath) ([]string, error)
    Exists(ctx context.Context, path SecretPath) (bool, error)
}
```

## Supported Backends

| Backend | Value | Auth |
|---------|-------|------|
| HashiCorp Vault | `vault` | `VAULT_TOKEN`, `~/.vault-token` |
| AWS Secrets Manager | `aws` | IAM role, IRSA, env vars |
| Azure Key Vault | `azure` | Managed Identity, `az login` |
| GCP Secret Manager | `gcp` | Workload Identity, ADC |
| 1Password CLI | `onepassword` | `op signin` session |

## Path Encoding Per Backend

Each backend has different naming constraints. kumokagi handles encoding internally.

| Backend | Path format | Example |
|---------|------------|---------|
| Vault | `{mount}/data/{env}/{app}/{key}` | `secret/data/prod/myapp/db_password` |
| AWS | `{env}/{app}/{key}` | `prod/myapp/db_password` |
| Azure | `{env}--{app}--{key}` | `prod--myapp--db_password` |
| GCP | `{env}--{app}--{key}` | `prod--myapp--db_password` |
| 1Password | Item `{env}--{app}--{key}`, field `password` | Item: `prod--myapp--db_password` |

## Ambient Credentials

Providers use **Ambient Credentials** — whatever authentication token the runtime already holds. No credential is stored in `.kumokagi.yaml` or application code.

| Actor | Mechanism |
|-------|-----------|
| Human (local/VPN) | `vault login`, `az login`, `aws configure`, `gcloud auth`, `op signin` |
| CI (GitHub Actions) | OIDC federation — no stored credential |
| K8s workload | IRSA, Workload Identity, Vault K8s auth |
