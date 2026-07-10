---
title: "ADR 0002: Secret Path Convention — Environment First"
weight: 2
---

# ADR 0002: Secret Path Convention — Environment First

## Status
Accepted

## Context
Secret Paths must follow a convention. Two natural orderings exist:
- `{app}/{env}/{key}` — group by app, subdivide by env
- `{env}/{app}/{key}` — group by env, subdivide by app

## Decision
The Secret Path convention is `{mount}/data/{env}/{app}/{key}`.

Example: `secret/data/prod/myapp/db_password`

## Rationale
Vault ACL policies are defined at path prefixes. Environment-first grouping allows a single policy to govern all access to a given environment across all applications:

```
path "secret/data/prod/*"  { capabilities = ["read"] }
path "secret/data/staging/*" { capabilities = ["read", "create", "update", "delete"] }
```

App-first grouping (`{app}/{env}`) would require per-app policies to restrict prod access, multiplying policy surface area as apps grow.

Environment-first is the harder constraint to enforce operationally — prod access is the critical boundary. App ownership is secondary.

## Consequences
- `kumokagi prune` lists secrets under `{mount}/data/{env}/{app}/` — requires knowing env at prune time
- Adding a new environment requires no policy changes if the pattern `{env}/*` is already restricted
- Migrating existing secrets from app-first paths requires a one-time rename operation
