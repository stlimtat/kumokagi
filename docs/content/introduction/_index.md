---
aliases: ["/concepts/"]
title: Introduction
weight: 1
---

# Introduction

![kumokagi — a gold key inside a blue cloud](/img/logo.svg)

## The situation today

Almost every application needs credentials: a database password, a payment-provider API key, a signing token. And almost every team distributes those credentials the same way — through the environment. A `.env` file on a laptop, an `env:` block in a Kubernetes manifest, a `secrets` stanza in a CI pipeline. The pattern is so common that it feels safe. It is not.

Once a secret enters an environment variable, you have lost control of it:

- **It persists for the lifetime of the process.** Any library, any dependency, any injected code can read `os.environ` at any time. A logging framework that dumps the environment on crash publishes your database password to your log aggregator.
- **It spreads.** Child processes inherit the environment. Debug endpoints print it. `kubectl describe pod` exposes it to anyone with read access to the namespace.
- **It ends up in git.** The `.env` file gets committed "temporarily". The Kubernetes `Secret` manifest is base64 — not encryption — and lives in the same repository as the code.
- **It cannot rotate.** Changing the value means rebuilding the environment: redeploying pods, re-running CI, editing manifests. So nobody rotates, and the same password lives for years.
- **It multiplies.** The same secret is copied into 1Password *and* the CI settings *and* the Helm values *and* a teammate's laptop, and now four places must be kept in sync — and audited.

Meanwhile every cloud already runs a purpose-built secrets service — Vault, AWS Secrets Manager, Azure Key Vault, GCP Secret Manager — with access control, audit logs, versioning, and rotation built in. The problem is not storage. The problem is the *last mile*: getting the value from the secrets service into the process without smearing it across the environment on the way.

**kumokagi is that last mile.** It is a small library (Go and Python) and CLI that fetches secrets directly from the backend into process memory at the moment of use, and nowhere else.

## What kumokagi intends, and how

Each subsection below states one intent and shows the code that delivers it.

### We do not trust secrets in environment variables

This is the founding intent. The conventional pattern injects the secret into the container's environment:

```yaml
# The pattern we reject: the secret leaks into the environment permanently
env:
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: myapp-secrets
        key: db_password
```

With kumokagi, the environment carries **no secret at all**. The application fetches the value straight from the backend into memory:

```go
cfg, _ := config.Load(".kumokagi.yaml")
p, _ := factory.New(ctx, cfg)

val, _ := p.Get(ctx, provider.SecretPath{
    Mount: "secret", Env: "prod", App: "myapp", Key: "db_password",
})
// val exists only in this process's memory. os.Environ() never saw it.
```

Nothing to inherit, nothing for a crash handler to dump, nothing for `kubectl describe` to reveal.

### No secrets in manifests, config files, or git

The only file kumokagi needs is `.kumokagi.yaml`, and it is deliberately boring — backend name, app name, key *names*. It is safe to commit because there is nothing in it worth stealing:

```yaml
backend: vault
mount: secret
app: myapp
env: prod
keys:
  - db_password
  - api_key
```

`kumokagi verify` checks at startup (or in CI) that every declared key actually exists in the backend, so a missing secret fails the deploy, not the 3am request.

### Applications survive credential rotation

Because kumokagi keeps **no cache** ([ADR 0001]({{< relref "/adrs/0001-no-in-memory-cache" >}})), every fetch returns the backend's current value. Rotate the secret in the backend and the very next fetch picks it up — no restart, no redeploy, no manifest edit:

![How an application survives credential rotation with kumokagi](/img/rotation.png)

For long-lived connections the pattern is: catch the authentication failure, re-fetch, reconnect:

```go
conn, err := pool.Acquire(ctx)
if err != nil {
    src.Load(ctx, v)               // re-fetch — always returns the fresh value
    pool, _ = newDBPool(ctx, src, v)
    conn, err = pool.Acquire(ctx)
}
```

Generating the new value is one command:

```bash
kumokagi rotate db_password --length 32
```

### One interface, any cloud

Swapping the secrets backend is a one-line config change, not a code change:

```yaml
backend: vault      # today
# backend: aws      # tomorrow — application code untouched
```

All five providers implement the same interface (`Get`, `Set`, `Delete`, `List`, `Exists`), and every secret is addressable by a single URI scheme:

```bash
kumokagi read kumokagi://vault/secret/prod/myapp/db_password
kumokagi read kumokagi://aws//prod/myapp/db_password
```

### No credentials to manage

kumokagi never stores, caches, or forwards a credential of its own. It delegates entirely to the backend SDK's **ambient credentials** — the identity the runtime already holds: IRSA on EKS, Workload Identity on AKS/GKE, `VAULT_TOKEN`, `op signin`. This eliminates the "secret to access the secrets" chain. See [Providers]({{< relref "/introduction/providers" >}}) for the per-backend mechanisms.

### The same behavior in Go and Python

Both libraries read the same `.kumokagi.yaml` and resolve the same paths, so a polyglot team shares one mental model. Go plugs into viper, Python into pydantic-settings — kumokagi is a *source* in your existing config chain, not a new config system ([ADR 0003]({{< relref "/adrs/0003-source-not-standalone" >}})):

```go
// Go
source := vipersource.New(p, cfg)
source.Load(ctx, viper.GetViper())
password := viper.GetString("db_password")
```

```python
# Python
class AppSecrets(BaseSettings):
    db_password: str = ""

    @classmethod
    def settings_customise_sources(cls, settings_cls, **kwargs):
        return (KumokagiSettingsSource(settings_cls, config=cfg),)
```

### Link only what you use

Backends are registered like `database/sql` drivers — a blank import links exactly the SDKs you need, keeping binaries small:

```go
import _ "github.com/stlimtat/kumokagi/pkg/providers/aws"   // AWS SDK only
```

```bash
pip install "kumokagi[aws]"    # boto3 only — no Azure, no GCP
```

## Reading on

- [Secrets and Ephemerality]({{< relref "/introduction/secrets" >}}) — what "ephemeral" means precisely
- [Providers]({{< relref "/introduction/providers" >}}) — the backend interface and path encodings
- [Configuration]({{< relref "/introduction/config" >}}) — the full `.kumokagi.yaml` schema
- [Glossary]({{< relref "/introduction/glossary" >}}) — the domain vocabulary used across these docs
