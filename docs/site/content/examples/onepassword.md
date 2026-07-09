---
title: 1Password
weight: 5
---

# 1Password

Hello-world example using 1Password CLI as the kumokagi backend.

One master password unlocks everything. No infrastructure to run, no IAM to configure — ideal for personal projects and small teams.

## Prerequisites

- [1Password CLI](https://developer.1password.com/docs/cli/) (`op`) installed
- Signed in: `op signin`

## Install

```bash
pip install kumokagi[onepassword]   # Python
```

## Config

```yaml
backend: onepassword
app: myapp
env: prod
keys: []
onepassword:
  mount: my-vault   # the 1Password vault name that holds your secrets
```

Generate it:

```bash
kumokagi init --app myapp --env prod --backend onepassword \
  --onepassword-mount my-vault
```

## Store a secret

```bash
kumokagi set db_password s3cr3t
```

This creates (or updates) a Login item in the `my-vault` vault via the `op` CLI.

## Use in Go

```go
package main

import (
    "context"

    "github.com/spf13/viper"
    "github.com/stlimtat/kumokagi/pkg/config"
    "github.com/stlimtat/kumokagi/pkg/factory"
    "github.com/stlimtat/kumokagi/pkg/vipersource"
)

func main() {
    ctx := context.Background()

    cfg, err := config.Load(".kumokagi.yaml")
    if err != nil {
        panic(err)
    }

    provider, err := factory.New(ctx, cfg)
    if err != nil {
        panic(err)
    }

    source := vipersource.New(provider, cfg)
    if err := source.Load(ctx, viper.GetViper()); err != nil {
        panic(err)
    }

    password := viper.GetString("db_password") // "s3cr3t"
    _ = password
}
```

## Use in Python

```python
from pydantic_settings import BaseSettings
from kumokagi.config import load_config
from kumokagi.factory import new_provider
from kumokagi.settings import KumokagiSettingsSource

cfg = load_config(".kumokagi.yaml")
provider = new_provider(cfg)

class AppSecrets(BaseSettings):
    db_password: str = ""

    @classmethod
    def settings_customise_sources(cls, settings_cls, **kwargs):
        return (KumokagiSettingsSource(settings_cls, provider=provider, config=cfg),)

secrets = AppSecrets()
print(secrets.db_password)  # s3cr3t
```

## Team sharing

Share the 1Password vault with teammates. Each member signs in with their own account — no shared passwords, full audit log. Ideal for teams of 2–10 before you need cloud IAM.
