---
title: GCP Secret Manager
weight: 4
---

# GCP Secret Manager

Hello-world example using GCP Secret Manager as the kumokagi backend.

## Prerequisites

- A GCP project with Secret Manager API enabled
- Application Default Credentials (ADC) configured: `gcloud auth application-default login`, a service account key, or Workload Identity Federation

## Install

```bash
pip install kumokagi[gcp]   # Python — pulls only the GCP SDK
```

## Config

```yaml
backend: gcp
app: myapp
env: prod
keys: []
gcp:
  project: my-gcp-project
```

Generate it:

```bash
kumokagi init --app myapp --env prod --backend gcp --gcp-project my-gcp-project
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

## Workload Identity Federation (GKE)

For GKE pods, annotate the Kubernetes service account to bind it to a GCP service account — no JSON keys needed:

```bash
gcloud iam service-accounts add-iam-policy-binding myapp@my-gcp-project.iam.gserviceaccount.com \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:my-gcp-project.svc.id.goog[default/myapp]"
```

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: myapp
  annotations:
    iam.gke.io/gcp-service-account: myapp@my-gcp-project.iam.gserviceaccount.com
```

ADC picks up the GKE metadata server token automatically.
