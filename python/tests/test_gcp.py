from unittest.mock import MagicMock

import pytest

secretmanager = pytest.importorskip("google.cloud.secretmanager", reason="google-cloud-secret-manager not installed")

from google.api_core.exceptions import NotFound  # noqa: E402

from kumokagi.config import Config, GCPConfig  # noqa: E402
from kumokagi.gcp import GCPProvider  # noqa: E402
from kumokagi.provider import SecretNotFoundError, SecretPath  # noqa: E402


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="my-project", env="prod", app="myapp", key=key)


@pytest.fixture
def gcp_provider(mocker):
    mock_client = MagicMock()
    mocker.patch("kumokagi.gcp.secretmanager.SecretManagerServiceClient", return_value=mock_client)
    cfg = Config(backend="gcp", mount="my-project", app="myapp", env="prod", gcp=GCPConfig(project="my-project"))
    return GCPProvider(cfg), mock_client


def test_get_existing(gcp_provider):
    provider, client = gcp_provider
    client.access_secret_version.return_value.payload.data = b"s3cr3t"
    assert provider.get(make_path("db_password")) == "s3cr3t"


def test_get_missing_raises(gcp_provider):
    provider, client = gcp_provider
    client.access_secret_version.side_effect = NotFound("not found")
    with pytest.raises(SecretNotFoundError):
        provider.get(make_path("missing"))


def test_set_creates_then_adds_version(gcp_provider):
    provider, client = gcp_provider
    client.get_secret.side_effect = NotFound("not found")
    provider.set(make_path("db_password"), "val")
    client.create_secret.assert_called_once()
    client.add_secret_version.assert_called_once()


def test_set_existing_adds_version(gcp_provider):
    provider, client = gcp_provider
    client.get_secret.return_value = MagicMock()
    provider.set(make_path("db_password"), "val")
    client.create_secret.assert_not_called()
    client.add_secret_version.assert_called_once()


def test_delete(gcp_provider):
    provider, client = gcp_provider
    provider.delete(make_path("db_password"))
    client.delete_secret.assert_called_once()


def test_delete_missing_is_idempotent(gcp_provider):
    provider, client = gcp_provider
    client.delete_secret.side_effect = NotFound("not found")
    provider.delete(make_path("missing"))  # should not raise


def test_list(gcp_provider):
    provider, client = gcp_provider
    s1, s2 = MagicMock(), MagicMock()
    s1.name = "projects/my-project/secrets/prod--myapp--db_password"
    s2.name = "projects/my-project/secrets/prod--myapp--api_key"
    client.list_secrets.return_value = [s1, s2]
    keys = provider.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


def test_exists_true(gcp_provider):
    provider, client = gcp_provider
    client.get_secret.return_value = MagicMock()
    assert provider.exists(make_path("db_password")) is True


def test_exists_false(gcp_provider):
    provider, client = gcp_provider
    client.get_secret.side_effect = NotFound("not found")
    assert provider.exists(make_path("missing")) is False
