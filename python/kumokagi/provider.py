from __future__ import annotations

import re
from abc import ABC, abstractmethod
from dataclasses import dataclass


class SecretNotFoundError(Exception):
    """Raised when a secret does not exist in the backend."""


class InvalidPathError(ValueError):
    """Raised when a SecretPath component fails validation."""


# A safe env/app/key component: alphanumerics plus dot/underscore/dash, first
# char never a dash. Forbidding "/" blocks Vault logical-path traversal;
# forbidding a leading "-" and "="/"["/"]" blocks option/assignment injection
# into the op CLI. Matched with fullmatch(): re.match with a "$" anchor accepts
# a trailing newline ("app\n"), which fullmatch() rejects.
_IDENTIFIER_RE = re.compile(r"[A-Za-z0-9_.][A-Za-z0-9._-]{0,252}")


def _validate_identifier(field: str, value: str) -> None:
    # A lone "." or ".." passes the charset but is a reserved path segment that
    # an HTTP router (e.g. Vault's) can collapse into a real traversal.
    if value in (".", ".."):
        raise InvalidPathError(f"{field} {value!r} is a reserved path segment")
    if not _IDENTIFIER_RE.fullmatch(value):
        raise InvalidPathError(f"invalid {field} {value!r}")


@dataclass
class SecretPath:
    mount: str
    env: str
    app: str
    key: str = ""

    def validate(self) -> None:
        """Reject components that could inject into a backend path, resource
        name, list filter, or the op CLI argv. Env and app are required and must
        be safe identifiers; key is validated only when present, so list/prune
        paths pass. Mount is checked loosely (it may be a URL for Azure or empty
        for AWS) but must not contain traversal sequences or control characters.
        """
        _validate_mount(self.mount)
        _validate_identifier("env", self.env)
        _validate_identifier("app", self.app)
        if self.key:
            _validate_identifier("key", self.key)

    def data_path(self) -> str:
        return f"{self.mount}/data/{self.env}/{self.app}/{self.key}"

    def metadata_path(self) -> str:
        return f"{self.mount}/metadata/{self.env}/{self.app}/{self.key}"

    def list_path(self) -> str:
        return f"{self.mount}/metadata/{self.env}/{self.app}"


def _validate_mount(mount: str) -> None:
    if not mount:
        return
    if ".." in mount:
        raise InvalidPathError(f"mount {mount!r} contains '..'")
    if any(ord(c) < 0x20 or ord(c) == 0x7F for c in mount):
        raise InvalidPathError("mount contains a control character")


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


class ValidatingProvider(Provider):
    """Wraps a Provider and validates every SecretPath before delegating.

    new_provider() returns providers already wrapped, so every command, the
    settings source, and any direct caller are guarded at one chokepoint.
    """

    def __init__(self, inner: Provider) -> None:
        self._inner = inner

    def get(self, path: SecretPath) -> str:
        path.validate()
        return self._inner.get(path)

    def set(self, path: SecretPath, value: str) -> None:
        path.validate()
        self._inner.set(path, value)

    def delete(self, path: SecretPath) -> None:
        path.validate()
        self._inner.delete(path)

    def list(self, path: SecretPath) -> list[str]:
        path.validate()
        return self._inner.list(path)

    def exists(self, path: SecretPath) -> bool:
        path.validate()
        return self._inner.exists(path)
