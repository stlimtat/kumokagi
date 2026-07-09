---
title: Startup MVP Guide
weight: 10
---

# Startup MVP Guide

Zero-infra to production-grade secrets in three steps. No secrets in env vars, no plaintext in files — that's the whole point.

## 1. Choose your backend

| Backend | Cost | Complexity | Good for |
|---------|------|------------|----------|
| 1Password | ~$3/month | None | Personal, solo dev, early team |
| AWS Secrets Manager | ~$0.40/secret/month | Low (IRSA handles auth) | Startup on AWS, k8s |
| HashiCorp Vault | Self-hosted: free | Medium | Multi-cloud, compliance needs |

**Rule of thumb:** start with 1Password, migrate to AWS Secrets Manager when you get a second engineer and a Kubernetes cluster.

## 2. Personal workflow — 1Password

Zero infra. One master password unlocks everything.

```bash
# install the 1Password CLI
brew install --cask 1password/tap/1password-cli

# sign in
op signin

# init kumokagi
kumokagi init --app myapp --env dev --backend onepassword --onepassword-mount dev-secrets

# store secrets at setup time
kumokagi set db_password s3cr3t
kumokagi set stripe_key sk_test_abc123
```

`.kumokagi.yaml` (commit this, it has no secrets):

```yaml
backend: onepassword
app: myapp
env: dev
keys: [db_password, stripe_key]
onepassword:
  mount: dev-secrets
```

## 3. Team/startup workflow — AWS Secrets Manager + IRSA

Pods authenticate via IAM role, no credentials to rotate or leak.

```bash
# init for AWS
kumokagi init --app myapp --env prod --backend aws --aws-region us-east-1

# store secrets at deploy time (CI/CD, not in code)
kumokagi set db_password s3cr3t
kumokagi set stripe_key sk_live_abc123
```

`.kumokagi.yaml`:

```yaml
backend: aws
app: myapp
env: prod
keys: [db_password, stripe_key]
aws:
  region: us-east-1
```

IRSA — annotate your Kubernetes service account once:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: myapp
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/myapp-secrets-role
```

IAM policy for the role (least-privilege):

```json
{
  "Effect": "Allow",
  "Action": ["secretsmanager:GetSecretValue"],
  "Resource": "arn:aws:secretsmanager:us-east-1:123456789012:secret:myapp/prod/*"
}
```

No `AWS_ACCESS_KEY_ID`. No `AWS_SECRET_ACCESS_KEY`. The SDK picks up the pod's token automatically.

## 4. Config file

`kumokagi init` generates `.kumokagi.yaml`. Commit it — it lists key names and backend config, never values.

```bash
kumokagi init --app myapp --env prod --backend aws --aws-region us-east-1
```

Add new keys to `keys:` as your app grows; kumokagi validates all keys exist at startup via `kumokagi verify`.

## 5. Fetch at runtime

### Go

```go
cfg, _ := config.Load(".kumokagi.yaml")
provider, _ := factory.New(ctx, cfg)
source := vipersource.New(provider, cfg)
source.Load(ctx, viper.GetViper())

// now use viper as normal — values come from your secrets backend
db := viper.GetString("db_password")
```

### Python

```python
cfg = load_config(".kumokagi.yaml")
provider = new_provider(cfg)

class AppSecrets(BaseSettings):
    db_password: str = ""
    stripe_key: str = ""

    @classmethod
    def settings_customise_sources(cls, settings_cls, **kwargs):
        return (KumokagiSettingsSource(settings_cls, provider=provider, config=cfg),)

secrets = AppSecrets()
```

## 6. Never store secrets in env vars or files

- `.env` files get committed. Env vars leak into logs and child processes.
- kumokagi fetches at runtime from a secrets backend that has access control, audit logs, and rotation.
- The config file (`.kumokagi.yaml`) lists key *names*, not values — safe to commit.

Run `kumokagi verify` in your CI to catch missing secrets before deploy.
