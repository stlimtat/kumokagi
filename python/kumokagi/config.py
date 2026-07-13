from __future__ import annotations

import os
from dataclasses import dataclass, field
from urllib.parse import urlparse

import yaml

from kumokagi.provider import SecretPath

ENV_VAR = "KUMOKAGI_ENV"
DEFAULT_MOUNT = "secret"
CONFIG_FILE = ".kumokagi.yaml"

# Cap the config file size to avoid a memory-exhaustion DoS from a hostile
# .kumokagi.yaml (the file is meant to be committed and small).
MAX_CONFIG_BYTES = 1 << 20  # 1 MiB

VALID_BACKENDS = {"vault", "aws", "azure", "gcp", "onepassword"}

# Endpoint allowlist env vars. When set (comma-separated), a backend endpoint
# resolved from config must appear in the list, or provider construction fails.
# Opt-in and fail-closed: stops a hostile committed config from redirecting a
# backend to an attacker host and stealing the ambient token (VAULT_TOKEN, or
# an Azure token whose audience covers every Key Vault).
ENV_ALLOWED_VAULT_ADDRS = "KUMOKAGI_ALLOWED_VAULT_ADDRS"
ENV_ALLOWED_AZURE_VAULTS = "KUMOKAGI_ALLOWED_AZURE_VAULTS"
ENV_ALLOWED_GCP_PROJECTS = "KUMOKAGI_ALLOWED_GCP_PROJECTS"


def _split_allow(env_var: str) -> list[str]:
    return [p.strip() for p in os.getenv(env_var, "").split(",") if p.strip()]


def _endpoint_host(s: str) -> str:
    if "://" in s:
        host = urlparse(s).netloc
        if host:
            return host.lower()
    return s.strip("/").lower()


def check_host_allowed(env_var: str, endpoint: str) -> None:
    """Raise ValueError if endpoint's host is not in the allowlist named by
    env_var. An unset allowlist permits any host (opt-in)."""
    allow = _split_allow(env_var)
    if not allow:
        return
    if _endpoint_host(endpoint) not in {_endpoint_host(a) for a in allow}:
        raise ValueError(f"endpoint {endpoint!r} is not in the {env_var} allowlist")


def check_value_allowed(env_var: str, value: str) -> None:
    """Raise ValueError if value is not in the allowlist named by env_var,
    matched exactly (used for non-URL identifiers such as a GCP project)."""
    allow = _split_allow(env_var)
    if not allow:
        return
    if value not in allow:
        raise ValueError(f"value {value!r} is not in the {env_var} allowlist")


class _NoAliasSafeLoader(yaml.SafeLoader):
    """SafeLoader that refuses YAML aliases, blocking the "billion laughs"
    alias-expansion DoS that a <1 MiB config could otherwise trigger."""

    def compose_node(self, parent, index):  # type: ignore[no-untyped-def]
        if isinstance(self.peek_event(), yaml.events.AliasEvent):
            raise yaml.YAMLError("YAML aliases are not allowed in .kumokagi.yaml")
        return super().compose_node(parent, index)


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
        if self.backend == "onepassword":
            if not self.mount:
                raise ValueError("onepassword backend requires vault name in mount")
            # A "/" in the vault name would add a path segment to the op:// ref
            # (op://{vault}/{item}/{field}) and address a different field.
            if "/" in self.mount:
                raise ValueError(f"onepassword vault name must not contain '/': {self.mount!r}")


def load_config(path: str = CONFIG_FILE) -> Config:
    size = os.path.getsize(path)
    if size > MAX_CONFIG_BYTES:
        raise ValueError(f"config {path} is too large ({size} bytes, max {MAX_CONFIG_BYTES})")
    with open(path) as f:
        data = yaml.load(f, Loader=_NoAliasSafeLoader) or {}

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
