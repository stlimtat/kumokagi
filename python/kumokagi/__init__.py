from kumokagi.config import (
    AWSConfig,
    AzureConfig,
    Config,
    GCPConfig,
    OnePasswordConfig,
    VaultConfig,
    load_config,
)
from kumokagi.factory import new_provider
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath
from kumokagi.settings import KumokagiSettingsSource

__all__ = [
    "AWSConfig",
    "AWSProvider",
    "AzureConfig",
    "AzureProvider",
    "Config",
    "GCPConfig",
    "GCPProvider",
    "KumokagiSettingsSource",
    "OnePasswordConfig",
    "OnePasswordProvider",
    "Provider",
    "SecretNotFoundError",
    "SecretPath",
    "VaultConfig",
    "load_config",
    "new_provider",
]


def __getattr__(name: str):
    # ponytail: lazy imports so missing optional deps don't break unrelated providers
    if name == "AWSProvider":
        from kumokagi.aws import AWSProvider
        return AWSProvider
    if name == "AzureProvider":
        from kumokagi.azure import AzureProvider
        return AzureProvider
    if name == "GCPProvider":
        from kumokagi.gcp import GCPProvider
        return GCPProvider
    if name == "OnePasswordProvider":
        from kumokagi.onepassword import OnePasswordProvider
        return OnePasswordProvider
    raise AttributeError(f"module 'kumokagi' has no attribute {name!r}")
