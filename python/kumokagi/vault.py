from __future__ import annotations

import hvac

from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class VaultProvider(Provider):
    """HashiCorp Vault KV v2 provider.

    Uses VAULT_TOKEN and VAULT_ADDR environment variables by default.
    Pass explicit address/token for testing.
    """

    def __init__(self, address: str = "", token: str = "") -> None:
        self._client = hvac.Client(url=address or None, token=token or None)

    def get(self, path: SecretPath) -> str:
        try:
            resp = self._client.secrets.kv.v2.read_secret_version(
                path=f"{path.env}/{path.app}/{path.key}",
                mount_point=path.mount,
                raise_on_deleted_version=True,
            )
        except hvac.exceptions.InvalidPath:
            raise SecretNotFoundError(path.data_path())
        data = resp.get("data", {}).get("data", {})
        if "value" not in data:
            raise SecretNotFoundError(f"{path.data_path()}: value field missing")
        return data["value"]

    def set(self, path: SecretPath, value: str) -> None:
        self._client.secrets.kv.v2.create_or_update_secret(
            path=f"{path.env}/{path.app}/{path.key}",
            secret={"value": value},
            mount_point=path.mount,
        )

    def delete(self, path: SecretPath) -> None:
        self._client.secrets.kv.v2.delete_metadata_and_all_versions(
            path=f"{path.env}/{path.app}/{path.key}",
            mount_point=path.mount,
        )

    def list(self, path: SecretPath) -> list[str]:
        try:
            resp = self._client.secrets.kv.v2.list_secrets(
                path=f"{path.env}/{path.app}",
                mount_point=path.mount,
            )
        except hvac.exceptions.InvalidPath:
            return []
        return resp.get("data", {}).get("keys", [])

    def exists(self, path: SecretPath) -> bool:
        try:
            self._client.secrets.kv.v2.read_secret_metadata(
                path=f"{path.env}/{path.app}/{path.key}",
                mount_point=path.mount,
            )
            return True
        except hvac.exceptions.InvalidPath:
            return False
