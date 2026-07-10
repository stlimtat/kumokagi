---
title: Architecture
weight: 4
---

# Architecture

## Overview

![kumokagi architecture — application, sources, factory, providers, backends](/img/architecture.png)

The application never talks to a cloud SDK directly. It reads configuration through its existing config layer (viper in Go, pydantic-settings in Python). kumokagi sits behind that layer as a **source**: `KumokagiSource.Load()` fetches every declared key through the provider returned by `factory.New()`, which selects the backend named in `.kumokagi.yaml`. Each provider translates the common interface into backend-specific API calls, authenticating with whatever ambient credential the runtime already holds.

## Key design decisions

### No in-memory cache

Every `Get()` call hits the backend. This ensures rotation takes effect immediately on the next fetch. See [ADR 0001]({{< relref "/adrs/0001-no-in-memory-cache" >}}).

### Source, not standalone

kumokagi is a **source** in viper (Go) and pydantic-settings (Python) — one entry in the existing config resolution chain. Applications keep their existing config API. See [ADR 0003]({{< relref "/adrs/0003-source-not-standalone" >}}).

### Environment-first path convention

Secret paths are `{env}/{app}/{key}` — environment comes first. This lets Vault ACL policies control prod access with a single rule: `path "secret/data/prod/*"`. See [ADR 0002]({{< relref "/adrs/0002-env-first-path-convention" >}}).

### Ambient credentials only

Providers use whatever auth token the runtime already holds. No credential is stored in `.kumokagi.yaml`. This eliminates the "secrets-to-access-secrets" chain (Key Vault access key → CI secret → 1Password).
