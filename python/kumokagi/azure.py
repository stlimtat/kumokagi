from __future__ import annotations

from azure.core.exceptions import ResourceNotFoundError
from azure.identity import DefaultAzureCredential
from azure.keyvault.secrets import SecretClient

from kumokagi.config import Config
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class AzureProvider(Provider):
    """Azure Key Vault provider."""

    def __init__(self, cfg: Config) -> None:
        vault_url = cfg.azure.vault_url or cfg.mount
        self._client = SecretClient(vault_url=vault_url, credential=DefaultAzureCredential())

    def _name(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--{path.key}"

    def _prefix(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--"

    def get(self, path: SecretPath) -> str:
        try:
            return self._client.get_secret(self._name(path)).value
        except ResourceNotFoundError:
            raise SecretNotFoundError(self._name(path))

    def set(self, path: SecretPath, value: str) -> None:
        self._client.set_secret(self._name(path), value)

    def delete(self, path: SecretPath) -> None:
        try:
            self._client.begin_delete_secret(self._name(path)).result()
            self._client.purge_deleted_secret(self._name(path))
        except ResourceNotFoundError:
            pass

    def list(self, path: SecretPath) -> list[str]:
        prefix = self._prefix(path)
        return [
            p.name[len(prefix):]
            for p in self._client.list_properties_of_secrets()
            if p.name and p.name.startswith(prefix)
        ]

    def exists(self, path: SecretPath) -> bool:
        try:
            self._client.get_secret_properties(self._name(path))
            return True
        except ResourceNotFoundError:
            return False
