from __future__ import annotations

import os
from dataclasses import dataclass, field

import yaml

from kumokagi.provider import SecretPath

ENV_VAR = "KUMOKAGI_ENV"
DEFAULT_MOUNT = "secret"
CONFIG_FILE = ".kumokagi.yaml"

# Cap the config file size to avoid a memory-exhaustion DoS from a hostile
# .kumokagi.yaml (the file is meant to be committed and small).
MAX_CONFIG_BYTES = 1 << 20  # 1 MiB

VALID_BACKENDS = {"vault", "aws", "azure", "gcp", "onepassword"}


@dataclass
class VaultConfig:
    address: str = ""


@dataclass
class AWSConfig:
    region: str = ""


@dataclass
class AzureConfig:
    vault_url: str = ""


@dataclass
class GCPConfig:
    project: str = ""


@dataclass
class OnePasswordConfig:
    pass


@dataclass
class Config:
    backend: str = ""
    mount: str = DEFAULT_MOUNT
    app: str = ""
    env: str = ""
    keys: list[str] = field(default_factory=list)
    vault: VaultConfig = field(default_factory=VaultConfig)
    aws: AWSConfig = field(default_factory=AWSConfig)
    azure: AzureConfig = field(default_factory=AzureConfig)
    gcp: GCPConfig = field(default_factory=GCPConfig)
    onepassword: OnePasswordConfig = field(default_factory=OnePasswordConfig)

    def validate(self) -> None:
        if not self.backend:
            raise ValueError("backend is required")
        if self.backend not in VALID_BACKENDS:
            raise ValueError(
                f"unknown backend {self.backend!r} (valid: {', '.join(sorted(VALID_BACKENDS))})"
            )
        if not self.app:
            raise ValueError("app is required")
        if not self.env:
            raise ValueError(f"env is required (set in config or {ENV_VAR})")
        # Validate mount/env/app and every declared key up front, so a hostile
        # .kumokagi.yaml is rejected before any value reaches a backend. Keys
        # are checked here because they are not otherwise validated until fetch.
        SecretPath(mount=self.mount, env=self.env, app=self.app).validate()
        for key in self.keys:
            SecretPath(mount=self.mount, env=self.env, app=self.app, key=key).validate()
        if self.backend == "azure" and not self.mount and not self.azure.vault_url:
            raise ValueError("azure backend requires vault URL in mount or azure.vault_url")
        if self.backend == "gcp" and not self.mount and not self.gcp.project:
            raise ValueError("gcp backend requires project ID in mount or gcp.project")
        if self.backend == "onepassword" and not self.mount:
            raise ValueError("onepassword backend requires vault name in mount")


def load_config(path: str = CONFIG_FILE) -> Config:
    size = os.path.getsize(path)
    if size > MAX_CONFIG_BYTES:
        raise ValueError(f"config {path} is too large ({size} bytes, max {MAX_CONFIG_BYTES})")
    with open(path) as f:
        data = yaml.safe_load(f) or {}

    vault_data = data.get("vault", {}) or {}
    aws_data = data.get("aws", {}) or {}
    azure_data = data.get("azure", {}) or {}
    gcp_data = data.get("gcp", {}) or {}

    cfg = Config(
        backend=data.get("backend", ""),
        mount=data.get("mount", DEFAULT_MOUNT) or DEFAULT_MOUNT,
        app=data.get("app", ""),
        env=data.get("env", ""),
        keys=data.get("keys", []),
        vault=VaultConfig(address=vault_data.get("address", "")),
        aws=AWSConfig(region=aws_data.get("region", "")),
        azure=AzureConfig(vault_url=azure_data.get("vault_url", "")),
        gcp=GCPConfig(project=gcp_data.get("project", "")),
        onepassword=OnePasswordConfig(),
    )
    if env_val := os.getenv(ENV_VAR):
        cfg.env = env_val
    return cfg
