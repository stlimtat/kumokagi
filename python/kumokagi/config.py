from __future__ import annotations

import os
from dataclasses import dataclass, field

import yaml

ENV_VAR = "KUMOKAGI_ENV"
DEFAULT_MOUNT = "secret"
CONFIG_FILE = ".kumokagi.yaml"


@dataclass
class VaultConfig:
    address: str = ""


@dataclass
class Config:
    backend: str = ""
    mount: str = DEFAULT_MOUNT
    app: str = ""
    env: str = ""
    keys: list[str] = field(default_factory=list)
    vault: VaultConfig = field(default_factory=VaultConfig)

    def validate(self) -> None:
        if not self.backend:
            raise ValueError("backend is required")
        if not self.app:
            raise ValueError("app is required")
        if not self.env:
            raise ValueError(f"env is required (set in config or {ENV_VAR})")


def load_config(path: str = CONFIG_FILE) -> Config:
    with open(path) as f:
        data = yaml.safe_load(f) or {}

    vault_data = data.get("vault", {})
    cfg = Config(
        backend=data.get("backend", ""),
        mount=data.get("mount", DEFAULT_MOUNT) or DEFAULT_MOUNT,
        app=data.get("app", ""),
        env=data.get("env", ""),
        keys=data.get("keys", []),
        vault=VaultConfig(address=vault_data.get("address", "")),
    )
    if env_val := os.getenv(ENV_VAR):
        cfg.env = env_val
    return cfg
