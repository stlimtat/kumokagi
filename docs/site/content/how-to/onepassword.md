---
title: Use 1Password CLI
weight: 5
---

# Use 1Password CLI

## Prerequisites

- `op` CLI installed and in PATH
- Active 1Password session (`op signin`)
- 1Password vault created for your application

## Config

```yaml
backend: onepassword
mount: Development   # 1Password vault name
app: myapp
env: prod
```

Each secret becomes a 1Password Login item named `{env}--{app}--{key}` with a `password` field.

## Store secrets

```bash
kumokagi set db_password "s3cr3t"
# Creates 1Password item: "prod--myapp--db_password" in vault "Development"
```

## Read directly with op

```bash
op read "op://Development/prod--myapp--db_password/password"
```

## CI

1Password does not support OIDC federation. For CI, use a service account token:

```yaml
- uses: 1password/load-secrets-action@v2
  with:
    export-env: false
  env:
    OP_SERVICE_ACCOUNT_TOKEN: ${{ secrets.OP_SERVICE_ACCOUNT_TOKEN }}
```

Consider switching to AWS/Azure/GCP for automated workloads. 1Password is best suited for developer-facing secret management.
