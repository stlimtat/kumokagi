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

    def get(self, path: SecretPath) -> str:
        ref = f"op://{self._vault}/{self._item(path)}/{path.key}"
        result = self._run(
            ["op", "read", ref, "--no-newline"],
            capture_output=True, text=True, check=False,
        )
        if result.returncode != 0:
            raise SecretNotFoundError(ref)
        return result.stdout

    def set(self, path: SecretPath, value: str) -> None:
        item = self._item(path)
        check = self._run(
            ["op", "item", "get", item, f"--vault={self._vault}", "--format=json"],
            capture_output=True, text=True, check=False,
        )
        if check.returncode == 0:
            self._run(
                ["op", "item", "edit", item, f"--vault={self._vault}", f"{path.key}={value}"],
                capture_output=True, text=True, check=True,
            )
        else:
            self._run(
                ["op", "item", "create", f"--vault={self._vault}", f"--title={item}", f"{path.key}={value}"],
                capture_output=True, text=True, check=True,
            )

    def delete(self, path: SecretPath) -> None:
        item = self._item(path)
        result = self._run(
            ["op", "item", "edit", item, f"--vault={self._vault}", f"{path.key}[delete]"],
            capture_output=True, text=True, check=False,
        )
        # ignore if item not found (returncode != 0 means missing, which is fine)

    def list(self, path: SecretPath) -> list[str]:
        item = self._item(path)
        result = self._run(
            ["op", "item", "get", item, f"--vault={self._vault}", "--format=json"],
            capture_output=True, text=True, check=False,
        )
        if result.returncode != 0:
            return []
        data = json.loads(result.stdout)
        return [f["label"] for f in data.get("fields", []) if f.get("label")]

    def exists(self, path: SecretPath) -> bool:
        item = self._item(path)
        result = self._run(
            ["op", "item", "get", item, f"--vault={self._vault}",
             f"--fields=label={path.key}", "--format=json"],
            capture_output=True, text=True, check=False,
        )
        return result.returncode == 0
