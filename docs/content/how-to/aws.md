---
aliases: ["/examples/aws/"]
title: AWS Secrets Manager
weight: 2
---

# AWS Secrets Manager

AWS Secrets Manager is the natural backend when your workloads already run on AWS: there is no server to operate, IAM handles all access control, and on EKS your pods authenticate with **IRSA** (IAM Roles for Service Accounts) — no access keys anywhere.

## How authentication works

![AWS IRSA authentication flow](/img/auth-aws.png)

The pod receives a projected service-account token (an OIDC JWT). The AWS SDK's default credential chain automatically exchanges it with STS (`AssumeRoleWithWebIdentity`) for temporary IAM credentials, which it then uses to call Secrets Manager. kumokagi never sees any of this — it simply uses the SDK, so whatever the default chain resolves (IRSA, instance profile, `~/.aws/credentials`, env vars) works unchanged.

Official documentation:

- [IAM roles for service accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
- [AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/latest/userguide/intro.html)
- [SDK default credential chain](https://docs.aws.amazon.com/sdkref/latest/guide/standardized-credentials.html)
- [GitHub Actions OIDC with AWS](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services)

## Install

```bash
pip install "kumokagi[aws]"   # Python — pulls only boto3
go get github.com/stlimtat/kumokagi   # Go
```

## Config

```yaml
backend: aws
app: myapp
env: prod
keys: [db_password]
aws:
  region: ap-southeast-1   # optional; falls back to AWS_REGION / SDK default
```

Generate it:

```bash
kumokagi init --app myapp --env prod --backend aws --aws-region ap-southeast-1
```

## Store a secret

```bash
kumokagi set db_password "s3cr3t"
# Stores as: prod/myapp/db_password → {"value":"s3cr3t"}
```

## Use in Go

```go
import _ "github.com/stlimtat/kumokagi/pkg/providers/aws" // links only the AWS SDK

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

## Least-privilege IAM policy

Grant the workload role read access to exactly its own path prefix:

```json
{
  "Effect": "Allow",
  "Action": [
    "secretsmanager:GetSecretValue",
    "secretsmanager:DescribeSecret",
    "secretsmanager:ListSecrets"
  ],
  "Resource": "arn:aws:secretsmanager:*:*:secret:prod/myapp/*"
}
```

## Kubernetes with IRSA

Associate an [OIDC provider with your cluster](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html), create a role with the policy above and a trust policy for the service account, then annotate the service account:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: myapp
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/myapp-secrets-role
```

No changes to your kumokagi config are needed; the SDK picks up the IRSA token automatically.

## CI with GitHub Actions (OIDC)

```yaml
- uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: arn:aws:iam::123456789012:role/github-ci
    aws-region: ap-southeast-1
```

No stored credentials — the OIDC token is exchanged for a temporary IAM role.

## Verify and troubleshoot access

Work through this ladder **from inside the pod** (`kubectl exec -it deploy/myapp -- sh`). Each step isolates one layer; the first failing step tells you where the problem is.

**1. Is the IRSA token projected into the pod?**

```bash
ls /var/run/secrets/eks.amazonaws.com/serviceaccount/
echo $AWS_ROLE_ARN $AWS_WEB_IDENTITY_TOKEN_FILE
```

Empty output → the service account annotation is missing or the pod does not use that service account. Check `kubectl get sa myapp -o yaml` and the pod spec's `serviceAccountName`.

**2. Does STS accept the identity?**

```bash
aws sts get-caller-identity
```

Expected: the *role* ARN (e.g. `arn:aws:sts::…:assumed-role/myapp-secrets-role/…`). An error here means the role's trust policy does not match the cluster OIDC provider, namespace, or service-account name. See [troubleshooting IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts-minimum-sdk.html).

**3. Can the role read the secret directly?**

```bash
aws secretsmanager get-secret-value --secret-id prod/myapp/db_password --query SecretString
```

`AccessDeniedException` → the IAM policy's `Resource` prefix does not match the secret name; compare against `{env}/{app}/{key}`.

**4. Does kumokagi resolve it end-to-end?**

```bash
kumokagi verify                                          # all declared keys
kumokagi read kumokagi://aws//prod/myapp/db_password     # a single key by URI
```

If steps 1–3 pass and this fails, the config file is wrong — check `backend`, `env` (and any `KUMOKAGI_ENV` override), `app`, and the region.
