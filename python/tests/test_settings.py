import pytest
from pydantic_settings import BaseSettings

from kumokagi.config import Config, VaultConfig
from kumokagi.provider import SecretNotFoundError, SecretPath
from kumokagi.settings import KumokagiSettingsSource


class MockProvider:
    def __init__(self, secrets: dict[str, str]) -> None:
        self._secrets = secrets

    def get(self, path: SecretPath) -> str:
        if path.key not in self._secrets:
            raise SecretNotFoundError(path.key)
        return self._secrets[path.key]

    def set(self, path, value): pass
    def delete(self, path): pass
    def list(self, path): return []
    def exists(self, path): return path.key in self._secrets


def make_cfg(keys: list[str]) -> Config:
    return Config(
        backend="vault", mount="secret", app="myapp", env="prod",
        keys=keys, vault=VaultConfig(address="https://vault.test"),
    )


def test_source_loads_declared_keys():
    provider = MockProvider({"db_password": "s3cr3t", "api_key": "abc123"})
    cfg = make_cfg(["db_password", "api_key"])

    class AppSecrets(BaseSettings):
        db_password: str = ""
        api_key: str = ""

        @classmethod
        def settings_customise_sources(cls, settings_cls, **kwargs):
            return (KumokagiSettingsSource(settings_cls, provider=provider, config=cfg),)

    s = AppSecrets()
    assert s.db_password == "s3cr3t"
    assert s.api_key == "abc123"


def test_source_raises_on_missing_secret():
    provider = MockProvider({"db_password": "s3cr3t"})
    cfg = make_cfg(["db_password", "api_key"])

    class AppSecrets(BaseSettings):
        db_password: str = ""
        api_key: str = ""

        @classmethod
        def settings_customise_sources(cls, settings_cls, **kwargs):
            return (KumokagiSettingsSource(settings_cls, provider=provider, config=cfg),)

    with pytest.raises(SecretNotFoundError):
        AppSecrets()


def test_source_only_loads_declared_keys():
    provider = MockProvider({"db_password": "s3cr3t"})
    cfg = make_cfg(["db_password"])

    class AppSecrets(BaseSettings):
        db_password: str = ""
        api_key: str = "default"

        @classmethod
        def settings_customise_sources(cls, settings_cls, **kwargs):
            return (KumokagiSettingsSource(settings_cls, provider=provider, config=cfg),)

    s = AppSecrets()
    assert s.db_password == "s3cr3t"
    assert s.api_key == "default"
