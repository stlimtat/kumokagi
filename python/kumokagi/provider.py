from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass


class SecretNotFoundError(Exception):
    """Raised when a secret does not exist in the backend."""


@dataclass
class SecretPath:
    mount: str
    env: str
    app: str
    key: str = ""

    def data_path(self) -> str:
        return f"{self.mount}/data/{self.env}/{self.app}/{self.key}"

    def metadata_path(self) -> str:
        return f"{self.mount}/metadata/{self.env}/{self.app}/{self.key}"

    def list_path(self) -> str:
        return f"{self.mount}/metadata/{self.env}/{self.app}"


class Provider(ABC):
    @abstractmethod
    def get(self, path: SecretPath) -> str:
        """Return the secret value. Raise SecretNotFoundError if absent."""

    @abstractmethod
    def set(self, path: SecretPath, value: str) -> None:
        """Create or update a secret."""

    @abstractmethod
    def delete(self, path: SecretPath) -> None:
        """Permanently delete a secret."""

    @abstractmethod
    def list(self, path: SecretPath) -> list[str]:
        """List all keys under mount/env/app/."""

    @abstractmethod
    def exists(self, path: SecretPath) -> bool:
        """Return True if the secret exists."""
