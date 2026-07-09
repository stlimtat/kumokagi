---
title: Secrets and Ephemerality
weight: 1
---

# Secrets and Ephemerality

## What is a Secret?

A **Secret** is an application credential — a database password, API key, or connection token — stored in a secrets backend. kumokagi enforces one rule: a Secret is never written to disk, environment variables, or container manifests. It lives only in process memory during the window of use.

## Why Not Environment Variables?

The traditional pattern injects secrets as environment variables at container startup:

```yaml
# K8s manifest — secret leaks into the environment permanently
env:
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: myapp-secrets
        key: db_password
```

Problems:
- Secret persists in the process environment for the container's lifetime
- Rotation requires a pod restart
- Any code in the process can read `os.environ`
- Secrets end up in K8s manifests, Helm charts, CI configs, and 1Password — three places to keep in sync

## Ephemeral Means Fetch-On-Use

kumokagi fetches secrets from the backend on demand. There is no in-memory cache. Every `secrets.get("db_password")` call hits the backend and returns the current value.

**Rotation is automatic**: when a secret rotates in the backend, the next fetch returns the new value. No pod restart required.

**Connection pool rotation**: catch authentication failures, re-fetch the secret, reconnect. The library provides a fresh value; reconnection is the application's responsibility.

## Secret Path Convention

Secrets are located by a structured path: `{mount}/{env}/{app}/{key}`.

| Component | Example | Source |
|-----------|---------|--------|
| mount | `secret` | `.kumokagi.yaml` mount field |
| env | `prod` | `KUMOKAGI_ENV` env var or config `env:` field |
| app | `myapp` | `.kumokagi.yaml` app field |
| key | `db_password` | Application code |

Environment is resolved in this order:
1. `KUMOKAGI_ENV` environment variable
2. `env:` field in `.kumokagi.yaml`

This lets you use one config file and switch environments by setting `KUMOKAGI_ENV=staging`.
