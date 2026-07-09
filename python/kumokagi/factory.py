from __future__ import annotations

from kumokagi.config import Config
from kumokagi.provider import Provider


def new_provider(cfg: Config) -> Provider:
    match cfg.backend:
        case "vault":
            from kumokagi.vault import VaultProvider
            return VaultProvider(address=cfg.vault.address)
        case "aws":
            from kumokagi.aws import AWSProvider
            return AWSProvider(cfg)
        case "azure":
            from kumokagi.azure import AzureProvider
            return AzureProvider(cfg)
        case "gcp":
            from kumokagi.gcp import GCPProvider
            return GCPProvider(cfg)
        case "onepassword":
            from kumokagi.onepassword import OnePasswordProvider
            return OnePasswordProvider(cfg)
        case _:
            raise ValueError(f"unknown backend {cfg.backend!r}")
