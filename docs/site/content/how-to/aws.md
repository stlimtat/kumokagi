---
title: Use AWS Secrets Manager
weight: 2
---

# Use AWS Secrets Manager

## Prerequisites

- AWS account with Secrets Manager access
- IAM credentials configured (env vars, IRSA, or instance profile)

## Config

```yaml
backend: aws
app: myapp
env: prod
aws:
  region: ap-southeast-1   # optional; defaults to AWS_DEFAULT_REGION
```

## Store secrets

```bash
kumokagi set db_password "s3cr3t"
# Stores as: prod/myapp/db_password → {"value":"s3cr3t"}
```

## IAM policy

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

## CI with GitHub Actions (OIDC)

```yaml
- uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: arn:aws:iam::123456789:role/github-ci
    aws-region: ap-southeast-1
```

No stored credentials — the OIDC token is exchanged for a temporary IAM role.

## K8s with IRSA

Annotate the service account with the IAM role ARN:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: myapp
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789:role/myapp
```
