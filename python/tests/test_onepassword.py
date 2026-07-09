import json
import subprocess

import pytest

from kumokagi.config import Config
from kumokagi.onepassword import OnePasswordProvider
from kumokagi.provider import SecretNotFoundError, SecretPath


def make_path(key: str = "") -> SecretPath:
    return SecretPath(mount="TestVault", env="prod", app="myapp", key=key)


def make_provider(responses: dict):
    """Build a provider with a fake _run that dispatches by command pattern."""

    def fake_run(cmd, *, capture_output, text, check=False):
        key = tuple(cmd)
        for pattern, result in responses.items():
            if all(p in cmd for p in pattern):
                rc, out = result
                if check and rc != 0:
                    raise subprocess.CalledProcessError(rc, cmd)
                return subprocess.CompletedProcess(cmd, rc, stdout=out, stderr="")
        rc = 1 if not check else (_ for _ in ()).throw(subprocess.CalledProcessError(1, cmd))
        return subprocess.CompletedProcess(cmd, 1, stdout="", stderr="item not found")

    cfg = Config(backend="onepassword", mount="TestVault", app="myapp", env="prod")
    return OnePasswordProvider(cfg, _run=fake_run)


def test_get_existing():
    provider = make_provider({
        ("op", "read", "op://TestVault/prod--myapp/db_password"): (0, "s3cr3t"),
    })
    assert provider.get(make_path("db_password")) == "s3cr3t"


def test_get_missing_raises():
    provider = make_provider({})
    with pytest.raises(SecretNotFoundError):
        provider.get(make_path("missing"))


def test_set_creates_new_item():
    calls = []

    def fake_run(cmd, *, capture_output, text, check=False):
        calls.append(list(cmd))
        if "get" in cmd:
            return subprocess.CompletedProcess(cmd, 1, stdout="", stderr="not found")
        return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

    cfg = Config(backend="onepassword", mount="TestVault", app="myapp", env="prod")
    provider = OnePasswordProvider(cfg, _run=fake_run)
    provider.set(make_path("new_key"), "val")
    assert any("create" in c for c in calls)


def test_set_updates_existing_item():
    calls = []

    def fake_run(cmd, *, capture_output, text, check=False):
        calls.append(list(cmd))
        if "get" in cmd:
            return subprocess.CompletedProcess(cmd, 0, stdout='{"fields":[]}', stderr="")
        return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

    cfg = Config(backend="onepassword", mount="TestVault", app="myapp", env="prod")
    provider = OnePasswordProvider(cfg, _run=fake_run)
    provider.set(make_path("db_password"), "updated")
    assert any("edit" in c for c in calls)


def test_delete():
    calls = []

    def fake_run(cmd, *, capture_output, text, check=False):
        calls.append(list(cmd))
        return subprocess.CompletedProcess(cmd, 0, stdout="", stderr="")

    cfg = Config(backend="onepassword", mount="TestVault", app="myapp", env="prod")
    provider = OnePasswordProvider(cfg, _run=fake_run)
    provider.delete(make_path("db_password"))
    assert any("edit" in c and "db_password[delete]" in c for c in calls)


def test_list():
    fields = [{"label": "db_password"}, {"label": "api_key"}]
    item_json = json.dumps({"fields": fields})

    def fake_run(cmd, *, capture_output, text, check=False):
        return subprocess.CompletedProcess(cmd, 0, stdout=item_json, stderr="")

    cfg = Config(backend="onepassword", mount="TestVault", app="myapp", env="prod")
    provider = OnePasswordProvider(cfg, _run=fake_run)
    keys = provider.list(make_path())
    assert sorted(keys) == ["api_key", "db_password"]


def test_exists_true():
    def fake_run(cmd, *, capture_output, text, check=False):
        return subprocess.CompletedProcess(cmd, 0, stdout='{"value":"s3cr3t"}', stderr="")

    cfg = Config(backend="onepassword", mount="TestVault", app="myapp", env="prod")
    provider = OnePasswordProvider(cfg, _run=fake_run)
    assert provider.exists(make_path("db_password")) is True


def test_exists_false():
    def fake_run(cmd, *, capture_output, text, check=False):
        return subprocess.CompletedProcess(cmd, 1, stdout="", stderr="not found")

    cfg = Config(backend="onepassword", mount="TestVault", app="myapp", env="prod")
    provider = OnePasswordProvider(cfg, _run=fake_run)
    assert provider.exists(make_path("missing")) is False
