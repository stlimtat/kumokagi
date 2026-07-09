from unittest.mock import MagicMock

import pytest

azure_keyvault = pytest.importorskip("azure.keyvault.secrets", reason="azure-keyvault-secrets not installed")

from azure.core.exceptions import ResourceNotFoundError  # noqa: E402

from kumokagi.azure import AzureProvider  # noqa: E402
from kumokagi.config import AzureConfig, Config  # noqa: E402
from kumokagi.provider import SecretNotFoundError, SecretPath  # noqa: E402


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="https://test.vault.azure.net", env="prod", app="myapp", key=key)


@pytest.fixture
def azure_provider(mocker):
    mock_client = MagicMock()
    mocker.patch("kumokagi.azure.SecretClient", return_value=mock_client)
    mocker.patch("kumokagi.azure.DefaultAzureCredential", return_value=MagicMock())
    cfg = Config(
        backend="azure",
        mount="https://test.vault.azure.net",
        app="myapp",
        env="prod",
        azure=AzureConfig(vault_url="https://test.vault.azure.net"),
    )
    return AzureProvider(cfg), mock_client


def test_get_existing(azure_provider):
    provider, client = azure_provider
    client.get_secret.return_value.value = "s3cr3t"
    assert provider.get(make_path("db_password")) == "s3cr3t"
    client.get_secret.assert_called_once_with("prod--myapp--db_password")


def test_get_missing_raises(azure_provider):
    provider, client = azure_provider
    client.get_secret.side_effect = ResourceNotFoundError("not found")
    with pytest.raises(SecretNotFoundError):
        provider.get(make_path("missing"))


def test_set(azure_provider):
    provider, client = azure_provider
    provider.set(make_path("db_password"), "newval")
    client.set_secret.assert_called_once_with("prod--myapp--db_password", "newval")


def test_delete(azure_provider):
    provider, client = azure_provider
    provider.delete(make_path("db_password"))
    client.begin_delete_secret.assert_called_once_with("prod--myapp--db_password")
    client.purge_deleted_secret.assert_called_once_with("prod--myapp--db_password")


def test_delete_missing_is_idempotent(azure_provider):
    provider, client = azure_provider
    client.begin_delete_secret.side_effect = ResourceNotFoundError("not found")
    provider.delete(make_path("missing"))  # should not raise


def test_list(azure_provider):
    provider, client = azure_provider
    props = [MagicMock(name=None), MagicMock()]
    props[0].name = "prod--myapp--db_password"
    props[1].name = "prod--myapp--api_key"
    client.list_properties_of_secrets.return_value = props
    keys = provider.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


def test_exists_true(azure_provider):
    provider, client = azure_provider
    client.get_secret_properties.return_value = MagicMock()
    assert provider.exists(make_path("db_password")) is True


def test_exists_false(azure_provider):
    provider, client = azure_provider
    client.get_secret_properties.side_effect = ResourceNotFoundError("not found")
    assert provider.exists(make_path("missing")) is False
