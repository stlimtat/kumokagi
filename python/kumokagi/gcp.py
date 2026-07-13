from __future__ import annotations

from google.api_core.exceptions import NotFound
from google.cloud import secretmanager

from kumokagi.config import ENV_ALLOWED_GCP_PROJECTS, Config, check_value_allowed
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class GCPProvider(Provider):
    """GCP Secret Manager provider."""

    def __init__(self, cfg: Config) -> None:
        project = cfg.gcp.project or cfg.mount
        # Fail closed: a hostile config could point the app at an attacker's
        # project. The Google endpoint is fixed, so this is misdirection rather
        # than token theft, but the allowlist stops it when configured.
        check_value_allowed(ENV_ALLOWED_GCP_PROJECTS, project)
        self._client = secretmanager.SecretManagerServiceClient()
        self._project = project

    def _name(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}--{path.key}"

    def _secret_path(self, name: str) -> str:
        return f"projects/{self._project}/secrets/{name}"

    def get(self, path: SecretPath) -> str:
        try:
            resp = self._client.access_secret_version(
                name=f"{self._secret_path(self._name(path))}/versions/latest"
            )
            return resp.payload.data.decode()
        except NotFound:
            raise SecretNotFoundError(self._name(path))

    def set(self, path: SecretPath, value: str) -> None:
        name = self._name(path)
        secret_path = self._secret_path(name)
        try:
            self._client.get_secret(name=secret_path)
        except NotFound:
            self._client.create_secret(
                parent=f"projects/{self._project}",
                secret_id=name,
                secret={"replication": {"automatic": {}}},
            )
        self._client.add_secret_version(
            parent=secret_path,
            payload={"data": value.encode()},
        )

    def delete(self, path: SecretPath) -> None:
        try:
            self._client.delete_secret(name=self._secret_path(self._name(path)))
        except NotFound:
            pass

    def list(self, path: SecretPath) -> list[str]:
        prefix = f"{path.env}--{path.app}--"
        secrets = self._client.list_secrets(
            request={"parent": f"projects/{self._project}", "filter": f"name:{prefix}"}
        )
        return [
            s.name.split("/")[-1][len(prefix):]
            for s in secrets
            if s.name.split("/")[-1].startswith(prefix)
        ]

    def exists(self, path: SecretPath) -> bool:
        try:
            self._client.get_secret(name=self._secret_path(self._name(path)))
            return True
        except NotFound:
            return False
