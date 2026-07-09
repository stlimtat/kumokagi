---
title: Use GCP Secret Manager
weight: 4
---

# Use GCP Secret Manager

## Prerequisites

- GCP project with Secret Manager API enabled
- Application Default Credentials configured (`gcloud auth application-default login` or Workload Identity)

## Config

```yaml
backend: gcp
mount: my-gcp-project   # or use gcp.project
app: myapp
env: prod
```

Secret names use double-dash encoding: `prod--myapp--db_password`.

## Store secrets

```bash
kumokagi set db_password "s3cr3t"
# Creates GCP secret: projects/my-gcp-project/secrets/prod--myapp--db_password
```

## IAM binding

Grant `roles/secretmanager.secretAccessor` to the workload service account:

```bash
gcloud secrets add-iam-policy-binding prod--myapp--db_password \
  --member="serviceAccount:myapp@my-project.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

## CI with GitHub Actions (OIDC)

```yaml
- id: auth
  uses: google-github-actions/auth@v2
  with:
    workload_identity_provider: projects/123/locations/global/workloadIdentityPools/github/providers/github
    service_account: github-ci@my-project.iam.gserviceaccount.com
```

## K8s with Workload Identity

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    iam.gke.io/gcp-service-account: myapp@my-project.iam.gserviceaccount.com
```
