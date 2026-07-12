from __future__ import annotations

import json
import subprocess
from collections.abc import Callable
from typing import Any

from kumokagi.config import Config
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class OnePasswordProvider(Provider):
    """1Password CLI (op) provider."""

    def __init__(self, cfg: Config, _run: Callable[..., Any] | None = None) -> None:
        self._vault = cfg.mount
        self._run = _run or subprocess.run  # ponytail: injection point for tests

    def _item(self, path: SecretPath) -> str:
        return f"{path.env}--{path.app}"

    # Every op invocation places flags before a "--" end-of-options terminator
    # and untrusted operands (item title, secret ref, field assignments) after
    # it, so no operand can be reinterpreted as a flag. This is defense in
    # depth: new_provider already validates every SecretPath, so env/app/key
    # cannot contain the leading "-", "=", "[", "]", or "/" that
    # option/assignment injection needs.
    def get(self, path: SecretPath) -> str:
        ref = f"op://{self._vault}/{self._item(path)}/{path.key}"
        result = self._run(
            ["op", "read", "--no-newline", "--", ref],
            capture_output=True, text=True, check=False,
        )
        if result.returncode != 0:
            raise SecretNotFoundError(ref)
        return result.stdout

    def set(self, path: SecretPath, value: str) -> None:
        item = self._item(path)
        check = self._run(
            ["op", "item", "get", f"--vault={self._vault}", "--format=json", "--", item],
            capture_output=True, text=True, check=False,
        )
        if check.returncode == 0:
            self._run(
                ["op", "item", "edit", f"--vault={self._vault}", "--", item, f"{path.key}={value}"],
                capture_output=True, text=True, check=True,
            )
        else:
            self._run(
                ["op", "item", "create", f"--vault={self._vault}", f"--title={item}",
                 "--", f"{path.key}={value}"],
                capture_output=True, text=True, check=True,
            )

    def delete(self, path: SecretPath) -> None:
        item = self._item(path)
        result = self._run(
            ["op", "item", "edit", f"--vault={self._vault}", "--", item, f"{path.key}[delete]"],
            capture_output=True, text=True, check=False,
        )
        # ignore if item not found (returncode != 0 means missing, which is fine)

    def list(self, path: SecretPath) -> list[str]:
        item = self._item(path)
        result = self._run(
            ["op", "item", "get", f"--vault={self._vault}", "--format=json", "--", item],
            capture_output=True, text=True, check=False,
        )
        if result.returncode != 0:
            return []
        data = json.loads(result.stdout)
        return [f["label"] for f in data.get("fields", []) if f.get("label")]

    def exists(self, path: SecretPath) -> bool:
        item = self._item(path)
        result = self._run(
            ["op", "item", "get", f"--vault={self._vault}",
             f"--fields=label={path.key}", "--format=json", "--", item],
            capture_output=True, text=True, check=False,
        )
        return result.returncode == 0
