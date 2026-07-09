from __future__ import annotations

from typing import Any

from pydantic_settings import BaseSettings, PydanticBaseSettingsSource

from kumokagi.config import Config
from kumokagi.provider import Provider, SecretPath


class KumokagiSettingsSource(PydanticBaseSettingsSource):
    """pydantic-settings source that fetches declared secrets from a Provider.

    Only keys listed in config.keys are fetched — undeclared fields are left
    to other sources in the resolution chain.
    """

    def __init__(
        self,
        settings_cls: type[BaseSettings],
        *,
        config: Config,
        provider: Provider | None = None,
    ) -> None:
        super().__init__(settings_cls)
        self._config = config
        if provider is None:
            from kumokagi.factory import new_provider
            provider = new_provider(config)
        self._provider = provider

    def get_field_value(self, field_name: str, field_info: Any) -> tuple[Any, str, bool]:
        if field_name not in self._config.keys:
            return None, field_name, False
        path = SecretPath(
            mount=self._config.mount,
            env=self._config.env,
            app=self._config.app,
            key=field_name,
        )
        value = self._provider.get(path)
        return value, field_name, True

    def __call__(self) -> dict[str, Any]:
        data: dict[str, Any] = {}
        for field_name in self._config.keys:
            path = SecretPath(
                mount=self._config.mount,
                env=self._config.env,
                app=self._config.app,
                key=field_name,
            )
            data[field_name] = self._provider.get(path)
        return data
