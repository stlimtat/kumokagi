"""AWS Secrets Manager provider tests.

Uses pytest-mock to avoid the moto->cryptography->Rust build dependency.
Swap in moto if you can install cryptography (needs Rust toolchain):
    @mock_aws
    def test_get_existing():
        client = boto3.client("secretsmanager", region_name="us-east-1")
        client.create_secret(Name="prod/myapp/db_password", SecretString='{"value":"s3cr3t"}')
        ...
"""
from unittest.mock import MagicMock

import pytest
from botocore.exceptions import ClientError

from kumokagi.aws import AWSProvider
from kumokagi.config import AWSConfig, Config
from kumokagi.provider import SecretNotFoundError, SecretPath


def make_cfg() -> Config:
    return Config(backend="aws", mount="", app="myapp", env="prod", aws=AWSConfig(region="us-east-1"))


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="", env="prod", app="myapp", key=key)


def _client_error(code: str) -> ClientError:
    return ClientError({"Error": {"Code": code, "Message": code}}, "op")


@pytest.fixture
def aws_provider(mocker):
    mock_client = MagicMock()
    mocker.patch("kumokagi.aws.boto3.client", return_value=mock_client)
    return AWSProvider(make_cfg()), mock_client


def test_get_existing(aws_provider):
    provider, client = aws_provider
    client.get_secret_value.return_value = {"SecretString": '{"value":"s3cr3t"}'}
    assert provider.get(make_path("db_password")) == "s3cr3t"
    client.get_secret_value.assert_called_once_with(SecretId="prod/myapp/db_password")


def test_get_missing_raises(aws_provider):
    provider, client = aws_provider
    client.get_secret_value.side_effect = _client_error("ResourceNotFoundException")
    with pytest.raises(SecretNotFoundError):
        provider.get(make_path("missing"))


def test_set_create(aws_provider):
    provider, client = aws_provider
    provider.set(make_path("new_key"), "newval")
    client.create_secret.assert_called_once_with(
        Name="prod/myapp/new_key", SecretString='{"value": "newval"}'
    )
    client.put_secret_value.assert_not_called()


def test_set_update(aws_provider):
    provider, client = aws_provider
    client.create_secret.side_effect = _client_error("ResourceExistsException")
    provider.set(make_path("db_password"), "updated")
    client.put_secret_value.assert_called_once_with(
        SecretId="prod/myapp/db_password", SecretString='{"value": "updated"}'
    )


def test_delete(aws_provider):
    provider, client = aws_provider
    provider.delete(make_path("db_password"))
    client.delete_secret.assert_called_once_with(
        SecretId="prod/myapp/db_password", ForceDeleteWithoutRecovery=True
    )


def test_delete_missing_is_idempotent(aws_provider):
    provider, client = aws_provider
    client.delete_secret.side_effect = _client_error("ResourceNotFoundException")
    provider.delete(make_path("missing"))  # should not raise


def test_list(aws_provider):
    provider, client = aws_provider
    page = {"SecretList": [
        {"Name": "prod/myapp/db_password"},
        {"Name": "prod/myapp/api_key"},
    ]}
    mock_paginator = MagicMock()
    mock_paginator.paginate.return_value = [page]
    client.get_paginator.return_value = mock_paginator
    keys = provider.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


def test_exists_true(aws_provider):
    provider, client = aws_provider
    client.describe_secret.return_value = {"Name": "prod/myapp/db_password"}
    assert provider.exists(make_path("db_password")) is True


def test_exists_false(aws_provider):
    provider, client = aws_provider
    client.describe_secret.side_effect = _client_error("ResourceNotFoundException")
    assert provider.exists(make_path("missing")) is False
