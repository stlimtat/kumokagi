---
title: AWS Secrets Manager
weight: 2
---

# AWS Secrets Manager

Hello-world example using AWS Secrets Manager as the kumokagi backend.

## Prerequisites

- AWS credentials configured (env vars, `~/.aws/credentials`, or IAM role)
- For Kubernetes: IRSA (IAM Roles for Service Accounts) — pods authenticate without credentials files

## Install

```bash
pip install kumokagi[aws]   # Python — pulls only the AWS SDK
```

Go uses the standard `go get`:

```bash
go get github.com/stlimtat/kumokagi
```

## Config

```yaml
backend: aws
app: myapp
env: prod
keys: []
aws:
  region: us-east-1   # optional, falls back to AWS_REGION / SDK default
```

Generate it:

```bash
kumokagi init --app myapp --env prod --backend aws --aws-region us-east-1
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

## Kubernetes / IRSA

Annotate your service account and the pod assumes an IAM role automatically — no credentials to rotate:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: myapp
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/myapp-secrets-role
```

No changes to your kumokagi config needed; the AWS SDK picks up the IRSA token automatically.
