---
title: Python — pydantic-settings integration
weight: 7
---

# Python — pydantic-settings integration

kumokagi implements a pydantic-settings **custom source** ([ADR 0003]({{< relref "/adrs/0003-source-not-standalone" >}})): your `BaseSettings` model stays the single config API, and kumokagi becomes one more place values can come from. Only keys listed in `config.keys` are fetched from the backend — other fields use env vars, defaults, or other sources.

![kumokagi architecture — where the pydantic-settings source sits](/img/architecture.png)

## Setup

```python
from pydantic_settings import BaseSettings
from kumokagi.config import load_config
from kumokagi.factory import new_provider
from kumokagi.settings import KumokagiSettingsSource

cfg = load_config(".kumokagi.yaml")
provider = new_provider(cfg)

class AppSecrets(BaseSettings):
    db_password: str = ""
    api_key: str = ""
    app_name: str = "myapp"  # not in keys list — loaded from env or default

    @classmethod
    def settings_customise_sources(cls, settings_cls, **kwargs):
        return (
            KumokagiSettingsSource(settings_cls, provider=provider, config=cfg),
            kwargs["env_settings"],  # env vars still work for non-secret fields
        )

secrets = AppSecrets()
```

## Declare which keys to fetch

Only keys in `.kumokagi.yaml`'s `keys:` list are fetched:

```yaml
keys:
  - db_password
  - api_key
```

Fields in your `BaseSettings` model not in `keys:` are resolved by other sources (env vars, defaults).

## Verify at startup

```python
from kumokagi.config import load_config
from kumokagi.factory import new_provider
from kumokagi.settings import KumokagiSettingsSource

cfg = load_config()
cfg.validate()  # raises ValueError if required fields missing

provider = new_provider(cfg)

# Check all declared secrets exist before app startup
for key in cfg.keys:
    from kumokagi.provider import SecretPath
    if not provider.exists(SecretPath(mount=cfg.mount, env=cfg.env, app=cfg.app, key=key)):
        raise RuntimeError(f"Missing secret: {key}")
```

## Connection pool rotation

```python
import psycopg2
from kumokagi.provider import SecretPath

def get_connection(cfg, provider):
    password = provider.get(SecretPath(
        mount=cfg.mount, env=cfg.env, app=cfg.app, key="db_password"
    ))
    return psycopg2.connect(f"host=db user=app password={password}")

# On auth failure:
try:
    conn = pool.getconn()
except psycopg2.OperationalError:
    conn = get_connection(cfg, provider)  # re-fetches fresh secret
```
