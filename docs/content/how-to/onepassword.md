---
aliases: ["/examples/onepassword/"]
title: 1Password
weight: 5
---

# 1Password

1Password is the zero-infrastructure backend: no server, no IAM, no cloud account. One master password (or biometric) unlocks everything. It is ideal for solo developers and small teams — and a common starting point before graduating to a cloud backend (see the [Startup MVP Guide]({{< relref "startup-mvp" >}})).

## How authentication works

![1Password CLI session flow](/img/auth-onepassword.png)

A human signs in once with `op signin`; the session token lives in the OS keychain. kumokagi shells out to the [`op` CLI](https://developer.1password.com/docs/cli/), which uses that session. There is no ambient machine identity here — that is the trade-off for zero infrastructure, and why 1Password is best for developer-facing use rather than production workloads.

Official documentation:

- [1Password CLI](https://developer.1password.com/docs/cli/)
- [op signin](https://developer.1password.com/docs/cli/reference/commands/signin/)
- [Service accounts (for CI)](https://developer.1password.com/docs/service-accounts/)
- [Secret reference syntax (op://)](https://developer.1password.com/docs/cli/secret-references/)

## Prerequisites

- `op` CLI installed (`brew install --cask 1password/tap/1password-cli`)
- Signed in: `op signin`
- A 1Password vault created for your application secrets

## Config

```yaml
backend: onepassword
app: myapp
env: prod
keys: [db_password]
onepassword:
  mount: Development   # the 1Password vault name
```

Generate it:

```bash
kumokagi init --app myapp --env prod --backend onepassword \
  --onepassword-mount Development
```

Each secret becomes a Login item named `{env}--{app}--{key}` with a `password` field.

## Store a secret

```bash
kumokagi set db_password "s3cr3t"
# Creates 1Password item "prod--myapp--db_password" in vault "Development"
```

Read it directly with `op` to cross-check:

```bash
op read "op://Development/prod--myapp--db_password/password"
```

## Use in Go

```go
import _ "github.com/stlimtat/kumokagi/pkg/providers/onepassword"

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

## Team sharing

Share the 1Password vault with teammates. Each member signs in with their own account — no shared passwords, full audit log. Ideal for teams of 2–10 before you need cloud IAM.

## CI

1Password does not support OIDC federation. For CI, use a [service account token](https://developer.1password.com/docs/service-accounts/):

```yaml
- uses: 1password/load-secrets-action@v2
  with:
    export-env: false
  env:
    OP_SERVICE_ACCOUNT_TOKEN: ${{ secrets.OP_SERVICE_ACCOUNT_TOKEN }}
```

For automated workloads at scale, prefer AWS/Azure/GCP — 1Password is best for developer-facing secret management.

## Verify and troubleshoot access

Each step isolates one layer; the first failing step tells you where the problem is.

**1. Is the CLI installed and signed in?**

```bash
op --version
op whoami
```

`not signed in` → run `op signin` (or `eval $(op signin)` in scripts). Sessions expire after 30 minutes idle.

**2. Can you see the vault?**

```bash
op vault list
```

Vault missing → your account was not granted access to it; ask the vault owner to share it.

**3. Does the item exist with the expected name?**

```bash
op item list --vault Development | grep prod--myapp
op read "op://Development/prod--myapp--db_password/password"
```

`isn't an item` → the item name does not match the `{env}--{app}--{key}` encoding; check for a different `env` value.

**4. Does kumokagi resolve it end-to-end?**

```bash
kumokagi verify
kumokagi read kumokagi://onepassword/Development/prod/myapp/db_password
```

If steps 1–3 pass and this fails, check `.kumokagi.yaml`: `mount` (vault name is case-sensitive), `env` (and any `KUMOKAGI_ENV` override), and `app`.
