---
aliases: ["/examples/gcp/"]
title: GCP Secret Manager
weight: 4
---

# GCP Secret Manager

GCP Secret Manager is the obvious backend on Google Cloud: IAM bindings control access per secret, and on GKE pods authenticate with **Workload Identity Federation** — the metadata server mints tokens for the bound Google service account, so no JSON key file ever exists.

## How authentication works

![GCP Workload Identity Federation authentication flow](/img/auth-gcp.png)

The pod runs as a Kubernetes service account (KSA) that is bound to a Google service account (GSA). When the application asks for credentials, Application Default Credentials (ADC) contacts the GKE metadata server, which returns an OAuth2 access token for the GSA — minted on demand, never stored. Outside GKE the same ADC chain falls back to `gcloud auth application-default login` or a workload-identity-federation config; kumokagi uses whatever ADC resolves.

Official documentation:

- [Workload Identity Federation for GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
- [Secret Manager overview](https://cloud.google.com/secret-manager/docs/overview)
- [Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials)
- [GitHub Actions OIDC with GCP](https://github.com/google-github-actions/auth)

## Install

```bash
pip install "kumokagi[gcp]"   # Python — pulls only the GCP SDK
go get github.com/stlimtat/kumokagi   # Go
```

## Config

```yaml
backend: gcp
app: myapp
env: prod
keys: [db_password]
gcp:
  project: my-gcp-project   # or use the mount field
```

Generate it:

```bash
kumokagi init --app myapp --env prod --backend gcp --gcp-project my-gcp-project
```

Secret Manager names cannot contain `/`, so kumokagi encodes the path with double dashes: `prod--myapp--db_password`.

## Store a secret

```bash
kumokagi set db_password "s3cr3t"
# Creates GCP secret: projects/my-gcp-project/secrets/prod--myapp--db_password
```

## Use in Go

```go
import _ "github.com/stlimtat/kumokagi/pkg/providers/gcp" // links only the GCP SDK

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

## Least-privilege IAM binding

Grant `roles/secretmanager.secretAccessor` to the workload's Google service account, per secret (or on the project with a condition):

```bash
gcloud secrets add-iam-policy-binding prod--myapp--db_password \
  --member="serviceAccount:myapp@my-gcp-project.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

## Kubernetes with Workload Identity (GKE)

Bind the KSA to the GSA and annotate:

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

## CI with GitHub Actions (OIDC)

```yaml
- id: auth
  uses: google-github-actions/auth@v2
  with:
    workload_identity_provider: projects/123/locations/global/workloadIdentityPools/github/providers/github
    service_account: github-ci@my-gcp-project.iam.gserviceaccount.com
```

## Verify and troubleshoot access

Run these **from inside the pod** (`kubectl exec -it deploy/myapp -- sh`); the first failing step tells you which layer is broken.

**1. Which identity does the metadata server hand out?**

```bash
curl -s -H "Metadata-Flavor: Google" \
  "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/email"
```

Expected: `myapp@my-gcp-project.iam.gserviceaccount.com`. If you see the Compute Engine default service account, the KSA→GSA annotation is missing or Workload Identity is not enabled on the node pool.

**2. Can it mint a token?**

```bash
curl -s -H "Metadata-Flavor: Google" \
  "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" | head -c 60
```

Failure → the `roles/iam.workloadIdentityUser` binding does not match `svc.id.goog[<namespace>/<ksa-name>]` exactly. See [troubleshooting Workload Identity](https://cloud.google.com/kubernetes-engine/docs/troubleshooting/troubleshooting-security#workload-identity).

**3. Can it read the secret directly?**

```bash
gcloud secrets versions access latest --secret prod--myapp--db_password --project my-gcp-project
```

`PERMISSION_DENIED` → the `secretAccessor` binding is missing on this secret (or the Secret Manager API is not enabled: `gcloud services enable secretmanager.googleapis.com`).

**4. Does kumokagi resolve it end-to-end?**

```bash
kumokagi verify
kumokagi read kumokagi://gcp/my-gcp-project/prod/myapp/db_password
```

If steps 1–3 pass and this fails, check `.kumokagi.yaml`: `backend`, `project`, `env` (and any `KUMOKAGI_ENV` override), and `app` — remember the stored name is the double-dash encoding.
