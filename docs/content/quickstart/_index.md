---
title: Quickstart
weight: 2
---

# Quickstart

Get kumokagi running in 5 minutes with HashiCorp Vault.

## Prerequisites

- HashiCorp Vault running and unsealed
- `VAULT_ADDR` and `VAULT_TOKEN` set in your environment
- Go 1.21+ or Python 3.12+

## 1. Create a config file

```bash
kumokagi init --app myapp --env prod --backend vault \
  --vault-addr https://vault.example.com
```

This creates `.kumokagi.yaml`:

```yaml
backend: vault
mount: secret
app: myapp
env: prod
keys: []
vault:
  address: https://vault.example.com
```

## 2. Store a secret

```bash
kumokagi set db_password "s3cr3t"
```

## 3. Verify it exists

```bash
kumokagi verify
```

## 4. Use in Go

```go
import (
    "github.com/spf13/viper"
    "github.com/stlimtat/kumokagi/pkg/config"
    "github.com/stlimtat/kumokagi/pkg/factory"
    "github.com/stlimtat/kumokagi/pkg/vipersource"
)

cfg, _ := config.Load(".kumokagi.yaml")
provider, _ := factory.New(ctx, cfg)
source := vipersource.New(provider, cfg)
source.Load(ctx, viper.GetViper())

password := viper.GetString("db_password")
```

## 5. Use in Python

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
