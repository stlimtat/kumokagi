---
aliases: ["/concepts/config/"]
title: Configuration
weight: 3
---

# Configuration

kumokagi reads `.kumokagi.yaml` from the current directory by default. Override with `--config <path>`.

## Full schema

```yaml
backend: vault           # required: vault | aws | azure | gcp | onepassword
mount: secret            # optional: KV mount (vault), region (aws), vault URL (azure), project (gcp), vault name (1password)
app: myapp               # required: application name
env: prod                # default env; overridden by KUMOKAGI_ENV
keys:                    # declared secrets — used by verify and prune
  - db_password
  - api_key

# Backend-specific (all optional)
vault:
  address: https://vault.example.com

aws:
  region: ap-southeast-1   # optional; falls back to AWS_DEFAULT_REGION

azure:
  vault_url: https://myapp.vault.azure.net   # or use mount field

gcp:
  project: my-gcp-project   # or use mount field
```

## Environment override

`KUMOKAGI_ENV` always overrides the `env:` field. Useful for deploying one config file across environments:

```bash
KUMOKAGI_ENV=staging kumokagi verify
KUMOKAGI_ENV=prod    kumokagi list
```

## Keys list

The `keys:` list declares which secrets your application uses. It drives:
- `kumokagi verify` — checks all declared keys exist
- `kumokagi prune` — reports keys in backend not in this list
- `vipersource.Load()` / `KumokagiSettingsSource` — fetches exactly these keys
