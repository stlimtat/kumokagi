from __future__ import annotations

import json

import boto3
from botocore.exceptions import ClientError

from kumokagi.config import Config
from kumokagi.provider import Provider, SecretNotFoundError, SecretPath


class AWSProvider(Provider):
    """AWS Secrets Manager provider."""

    def __init__(self, cfg: Config) -> None:
        region = cfg.aws.region or None
        self._client = boto3.client("secretsmanager", region_name=region)

    def _name(self, path: SecretPath) -> str:
        return f"{path.env}/{path.app}/{path.key}"

    def get(self, path: SecretPath) -> str:
        try:
            resp = self._client.get_secret_value(SecretId=self._name(path))
        except ClientError as e:
            if e.response["Error"]["Code"] == "ResourceNotFoundException":
                raise SecretNotFoundError(self._name(path)) from e
            raise
        return json.loads(resp["SecretString"])["value"]

    def set(self, path: SecretPath, value: str) -> None:
        payload = json.dumps({"value": value})
        try:
            self._client.create_secret(Name=self._name(path), SecretString=payload)
        except ClientError as e:
            if e.response["Error"]["Code"] == "ResourceExistsException":
                self._client.put_secret_value(SecretId=self._name(path), SecretString=payload)
            else:
                raise

    def delete(self, path: SecretPath) -> None:
        try:
            self._client.delete_secret(SecretId=self._name(path), ForceDeleteWithoutRecovery=True)
        except ClientError as e:
            if e.response["Error"]["Code"] != "ResourceNotFoundException":
                raise

    def list(self, path: SecretPath) -> list[str]:
        prefix = f"{path.env}/{path.app}/"
        paginator = self._client.get_paginator("list_secrets")
        keys: list[str] = []
        for page in paginator.paginate(Filters=[{"Key": "name", "Values": [prefix]}]):
            for secret in page.get("SecretList", []):
                name = secret["Name"]
                if name.startswith(prefix):
                    keys.append(name[len(prefix):])
        return keys

    def exists(self, path: SecretPath) -> bool:
        try:
            self._client.describe_secret(SecretId=self._name(path))
            return True
        except ClientError as e:
            if e.response["Error"]["Code"] == "ResourceNotFoundException":
                return False
            raise
