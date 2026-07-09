---
title: Use Azure Key Vault
weight: 3
---

# Use Azure Key Vault

## Prerequisites

- Azure Key Vault instance with RBAC or access policies configured
- Azure identity: Managed Identity, `az login`, or service principal env vars

## Config

```yaml
backend: azure
mount: https://myapp.vault.azure.net   # or use azure.vault_url
app: myapp
env: prod
```

Secret names use double-dash encoding: `prod--myapp--db_password`.

## Store secrets

```bash
kumokagi set db_password "s3cr3t"
# Stores as Azure secret named: prod--myapp--db_password
```

## RBAC assignment

Assign `Key Vault Secrets Officer` to the identity that manages secrets, and `Key Vault Secrets User` to workloads:

```bash
az role assignment create \
  --role "Key Vault Secrets User" \
  --assignee <managed-identity-principal-id> \
  --scope /subscriptions/.../resourceGroups/.../providers/Microsoft.KeyVault/vaults/myapp
```

## CI with GitHub Actions (OIDC)

```yaml
- uses: azure/login@v2
  with:
    client-id: ${{ vars.AZURE_CLIENT_ID }}
    tenant-id: ${{ vars.AZURE_TENANT_ID }}
    subscription-id: ${{ vars.AZURE_SUBSCRIPTION_ID }}
```

No stored credentials — uses OIDC federation.

## K8s with Managed Identity (AKS)

Enable Workload Identity on the AKS cluster and annotate the pod service account:

```yaml
metadata:
  labels:
    azure.workload.identity/use: "true"
```
