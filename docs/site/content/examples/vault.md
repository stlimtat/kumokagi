---
title: Vault
weight: 1
---

# HashiCorp Vault

Hello-world example using HashiCorp Vault as the kumokagi backend.

## Prerequisites

- Vault running and unsealed
- `VAULT_ADDR` and `VAULT_TOKEN` set in your environment

## Config

Create `.kumokagi.yaml`:

```yaml
backend: vault
mount: secret
app: myapp
env: prod
keys: []
vault:
  address: https://vault.example.com
```

Or generate it:

```bash
kumokagi init --app myapp --env prod --backend vault \
  --vault-addr https://vault.example.com
```

## Store a secret

```bash
kumokagi set db_password s3cr3t
```

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

```bash
pip install kumokagi
```

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
