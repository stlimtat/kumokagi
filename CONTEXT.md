# kumokagi — Domain Glossary

## Secret

An application credential (password, API key, connection token) stored in a secrets backend. A Secret is never written to disk, environment variables, or container manifests. It lives only in process memory during use.

## Ephemeral

A Secret is ephemeral when it exists only in process memory during the window of use — fetched from the backend on demand, never cached, never persisted. Rotation takes effect on the next fetch.

## Secrets Backend

The cloud infrastructure that stores and controls access to Secrets (e.g. HashiCorp Vault, AWS Secrets Manager, Azure Key Vault, GCP Secret Manager). Each backend is a Provider implementation behind a common interface.

## Provider

A kumokagi implementation of the secrets backend interface for a specific Secrets Backend. v1 Provider is HashiCorp Vault. Each Provider translates the common interface to backend-specific API calls.

## Ambient Credential

The infrastructure authentication token the runtime already holds — e.g. `VAULT_TOKEN`, IAM role via IRSA, Azure Managed Identity, GCP Workload Identity. An Ambient Credential is not a Secret; it is infrastructure authentication. The library never stores or manages Ambient Credentials — it delegates entirely to the backend SDK's default credential chain.

## Source

kumokagi's role in the application config resolution chain. In Go: a viper remote provider. In Python: a pydantic-settings custom settings source. kumokagi is one Source among many — it does not replace viper or pydantic-settings.

## Secret Path

The fully-qualified location of a Secret in the backend, resolved by convention: `{mount}/{env}/{app}/{key}`. Example: `secret/data/prod/myapp/db_password`. Components:

- **mount**: The Vault KV secrets engine mount point, configurable in `.kumokagi.yaml` (default: `secret`)
- **env**: The deployment environment (e.g. `prod`, `staging`, `local`), resolved from `KUMOKAGI_ENV` env var, falling back to `env:` in `.kumokagi.yaml`
- **app**: The application name, declared in `.kumokagi.yaml`
- **key**: The logical secret name, declared in the application's secrets struct / Pydantic model

## App Config

The `.kumokagi.yaml` file co-located with the application. Contains non-secret metadata only: backend type, mount, app name, and default env. Never contains Secret values or Ambient Credentials.

## Orphaned Secret

A Secret present in the backend under `{mount}/{env}/{app}/` but not declared in the app's secrets struct or Pydantic model. Identified by `kumokagi prune`.

## Rotation Pattern

The application-level pattern for handling Secret rotation in long-lived connections: catch the authentication failure, re-fetch the Secret (always fresh from backend), reconnect. The library provides a fresh value on every fetch; the reconnect logic is the application's responsibility.
