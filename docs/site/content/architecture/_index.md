---
title: Architecture
weight: 3
---

# Architecture

## Overview

```
Application code
      │
      ▼
┌─────────────────┐        ┌──────────────────┐
│  viper / pydantic│◄──────│  KumokagiSource  │
│  settings        │        │  (Load/Verify)   │
└─────────────────┘        └────────┬─────────┘
                                     │
                            ┌────────▼─────────┐
                            │  factory.New()    │
                            │  (backend select) │
                            └────────┬─────────┘
                                     │
               ┌─────────────────────┼─────────────────────┐
               ▼                     ▼                       ▼
        ┌──────────┐          ┌──────────┐           ┌──────────┐
        │  Vault   │          │   AWS    │           │  Azure   │
        │ Provider │          │ Provider │           │ Provider │
        └──────────┘          └──────────┘           └──────────┘
               │                     │                       │
               ▼                     ▼                       ▼
        HashiCorp Vault       AWS Secrets Mgr        Azure Key Vault
```

## Key design decisions

### No in-memory cache

Every `Get()` call hits the backend. This ensures rotation takes effect immediately on the next fetch. See [ADR 0001](https://github.com/stlimtat/kumokagi/blob/master/docs/adr/0001-no-in-memory-cache.md).

### Source, not standalone

kumokagi is a **source** in viper (Go) and pydantic-settings (Python) — one entry in the existing config resolution chain. Applications keep their existing config API. See [ADR 0003](https://github.com/stlimtat/kumokagi/blob/master/docs/adr/0003-source-not-standalone.md).

### Environment-first path convention

Secret paths are `{env}/{app}/{key}` — environment comes first. This lets Vault ACL policies control prod access with a single rule: `path "secret/data/prod/*"`. See [ADR 0002](https://github.com/stlimtat/kumokagi/blob/master/docs/adr/0002-env-first-path-convention.md).

### Ambient credentials only

Providers use whatever auth token the runtime already holds. No credential is stored in `.kumokagi.yaml`. This eliminates the "secrets-to-access-secrets" chain (Key Vault access key → CI secret → 1Password).
