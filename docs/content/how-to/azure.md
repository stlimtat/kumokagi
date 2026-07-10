---
aliases: ["/examples/azure/"]
title: Azure Key Vault
weight: 3
---

# Azure Key Vault

Azure Key Vault is the right backend when your workloads live on Azure: RBAC governs access, and on AKS pods authenticate with **Workload Identity** — a federated Kubernetes token exchanged for an Entra ID access token, with no client secret to store or rotate.

## How authentication works

![Azure Workload Identity authentication flow](/img/auth-azure.png)

The pod receives a projected service-account token. `DefaultAzureCredential` (the azure-sdk default chain) presents it to Microsoft Entra ID as a federated credential and receives an access token scoped to the vault. Outside Kubernetes the same chain falls back to `az login`, managed identity, or service-principal environment variables — kumokagi uses whatever the chain resolves.

Official documentation:

- [Azure AD Workload Identity on AKS](https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview)
- [Azure Key Vault secrets](https://learn.microsoft.com/en-us/azure/key-vault/secrets/about-secrets)
- [DefaultAzureCredential](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/credential-chains)
- [GitHub Actions OIDC with Azure](https://learn.microsoft.com/en-us/azure/developer/github/connect-from-azure-openid-connect)

## Install

```bash
pip install "kumokagi[azure]"   # Python — pulls only the Azure SDK
go get github.com/stlimtat/kumokagi   # Go
```

## Config

```yaml
backend: azure
app: myapp
env: prod
keys: [db_password]
azure:
  vault_url: https://myapp.vault.azure.net   # or use the mount field
```

Generate it:

```bash
kumokagi init --app myapp --env prod --backend azure \
  --azure-vault-url https://myapp.vault.azure.net
```

Key Vault secret names cannot contain `/`, so kumokagi encodes the path with double dashes: `prod--myapp--db_password`.

## Store a secret

```bash
kumokagi set db_password "s3cr3t"
# Stores as Azure secret named: prod--myapp--db_password
```

## Use in Go

```go
import _ "github.com/stlimtat/kumokagi/pkg/providers/azure" // links only the Azure SDK

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

## Least-privilege RBAC

Assign `Key Vault Secrets User` (read-only) to workloads and `Key Vault Secrets Officer` only to the identity that manages secrets:

```bash
az role assignment create \
  --role "Key Vault Secrets User" \
  --assignee <managed-identity-principal-id> \
  --scope /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.KeyVault/vaults/myapp
```

## Kubernetes with Workload Identity (AKS)

Enable the [Workload Identity add-on](https://learn.microsoft.com/en-us/azure/aks/workload-identity-deploy-cluster), create a managed identity with a federated credential for the service account, then label and annotate:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: myapp
  annotations:
    azure.workload.identity/client-id: <managed-identity-client-id>
---
# pod template
metadata:
  labels:
    azure.workload.identity/use: "true"
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

## Verify and troubleshoot access

Run these **from inside the pod** (`kubectl exec -it deploy/myapp -- sh`); the first failing step tells you which layer is broken.

**1. Is the federated token projected?**

```bash
echo $AZURE_CLIENT_ID $AZURE_TENANT_ID $AZURE_FEDERATED_TOKEN_FILE
ls /var/run/secrets/azure/tokens/
```

Empty → the pod is missing the `azure.workload.identity/use: "true"` label or the service account annotation. Check both, then restart the pod (the webhook injects at admission time).

**2. Can the identity get a token?**

```bash
az login --federated-token "$(cat $AZURE_FEDERATED_TOKEN_FILE)" \
  --service-principal -u $AZURE_CLIENT_ID -t $AZURE_TENANT_ID
az account get-access-token --resource https://vault.azure.net --query expiresOn
```

Failure → the federated credential on the managed identity does not match the cluster's OIDC issuer, namespace, or service-account name. See [federated identity credentials](https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation).

**3. Can it read the secret directly?**

```bash
az keyvault secret show --vault-name myapp --name prod--myapp--db_password --query value
```

`Forbidden` → the RBAC assignment is missing, on the wrong scope, or still propagating (RBAC changes can take a few minutes). Verify with `az role assignment list --assignee <client-id> --scope <vault-scope>`.

**4. Does kumokagi resolve it end-to-end?**

```bash
kumokagi verify
kumokagi read kumokagi://azure/https%3A%2F%2Fmyapp.vault.azure.net/prod/myapp/db_password
```

If steps 1–3 pass and this fails, check `.kumokagi.yaml`: `backend`, `vault_url`, `env` (and any `KUMOKAGI_ENV` override), and `app` — remember the stored name is the double-dash encoding.
