import pytest

from kumokagi.provider import SecretNotFoundError, SecretPath
from kumokagi.vault import VaultProvider


@pytest.fixture
def vault_client(mock_vault):
    return VaultProvider(address="https://vault.test:8200", token="test-token")


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="secret", env="prod", app="myapp", key=key)


def test_get_existing_secret(vault_client):
    assert vault_client.get(make_path("db_password")) == "s3cr3t"


def test_get_missing_secret_raises(vault_client):
    with pytest.raises(SecretNotFoundError):
        vault_client.get(make_path("missing"))


def test_set_secret(vault_client):
    vault_client.set(make_path("newkey"), "newvalue")


def test_delete_secret(vault_client):
    vault_client.delete(make_path("db_password"))


def test_list_secrets(vault_client):
    keys = vault_client.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


def test_exists_true(vault_client):
    assert vault_client.exists(make_path("db_password")) is True


def test_exists_false(vault_client):
    assert vault_client.exists(make_path("missing")) is False
