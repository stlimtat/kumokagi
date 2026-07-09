---
title: Azure Key Vault
weight: 3
---

# Azure Key Vault

Hello-world example using Azure Key Vault as the kumokagi backend.

## Prerequisites

- An Azure Key Vault instance
- `DefaultAzureCredential` configured: Azure CLI login, managed identity, or workload identity

## Install

```bash
pip install kumokagi[azure]   # Python — pulls only the Azure SDK
```

## Config

```yaml
backend: azure
app: myapp
env: prod
keys: []
azure:
  vault_url: https://my-vault.vault.azure.net/
```

Generate it:

```bash
kumokagi init --app myapp --env prod --backend azure \
  --azure-vault-url https://my-vault.vault.azure.net/
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

## Workload Identity (AKS)

For pods running in AKS, use Azure Workload Identity instead of storing credentials:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: myapp
  annotations:
    azure.workload.identity/client-id: <client-id>
```

`DefaultAzureCredential` picks up the federated token automatically — no secrets in your deployment manifests.
