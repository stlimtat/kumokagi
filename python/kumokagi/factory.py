from __future__ import annotations

from kumokagi.config import Config
from kumokagi.provider import Provider, ValidatingProvider


def new_provider(cfg: Config) -> Provider:
    inner: Provider
    match cfg.backend:
        case "vault":
            from kumokagi.vault import VaultProvider
            inner = VaultProvider(address=cfg.vault.address)
        case "aws":
            from kumokagi.aws import AWSProvider
            inner = AWSProvider(cfg)
        case "azure":
            from kumokagi.azure import AzureProvider
            inner = AzureProvider(cfg)
        case "gcp":
            from kumokagi.gcp import GCPProvider
            inner = GCPProvider(cfg)
        case "onepassword":
            from kumokagi.onepassword import OnePasswordProvider
            inner = OnePasswordProvider(cfg)
        case _:
            raise ValueError(f"unknown backend {cfg.backend!r}")
    # Wrap so every SecretPath is validated before it reaches a backend path,
    # resource name, or the op CLI argv — the single injection chokepoint.
    return ValidatingProvider(inner)
