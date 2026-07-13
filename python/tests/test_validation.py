"""Injection-hardening regression tests: SecretPath validation, the validating
provider chokepoint, config rejection of hostile paths, and the op CLI
end-of-options terminator."""

from __future__ import annotations

import textwrap

import pytest

from kumokagi.config import load_config
from kumokagi.onepassword import OnePasswordProvider
from kumokagi.provider import (
    InvalidPathError,
    Provider,
    SecretPath,
    ValidatingProvider,
)


@pytest.mark.parametrize(
    "path",
    [
        SecretPath(mount="secret", env="prod", app="myapp", key="db_password"),
        SecretPath(mount="secret", env="prod", app="myapp"),  # empty key (list path)
        SecretPath(mount="https://x.vault.azure.net", env="prod", app="my.app", key="k-1"),
        SecretPath(mount="", env="prod", app="app", key="k"),  # empty mount (AWS)
    ],
)
def test_valid_paths(path):
    path.validate()  # must not raise


@pytest.mark.parametrize(
    "path",
    [
        SecretPath(mount="secret", env="prod", app="app", key="../../metadata/prod/other/k"),
        SecretPath(mount="secret", env="prod", app="a/b", key="k"),
        SecretPath(mount="v", env="prod", app="app", key="--vault=evil"),
        SecretPath(mount="v", env="prod", app="app", key="password[password]"),
        SecretPath(mount="secret", env="-x", app="app", key="k"),
        SecretPath(mount="secret/..", env="prod", app="app", key="k"),
        SecretPath(mount="secret", env="prod", app="app\nx", key="k"),
        # trailing newline: re.match with "$" accepts these, fullmatch rejects
        SecretPath(mount="secret", env="prod", app="app", key="db_password\n"),
        SecretPath(mount="secret", env="prod\n", app="app", key="k"),
        # "." / ".." are reserved path segments a router can collapse
        SecretPath(mount="secret", env="prod", app="app", key=".."),
        SecretPath(mount="secret", env="prod", app="..", key="k"),
        SecretPath(mount="secret", env=".", app="app", key="k"),
        SecretPath(mount="secret", env="", app="app", key="k"),
    ],
)
def test_invalid_paths(path):
    with pytest.raises(InvalidPathError):
        path.validate()


class _Stub(Provider):
    def __init__(self):
        self.got = None

    def get(self, path):
        self.got = path
        return "value"

    def set(self, path, value):
        self.got = path

    def delete(self, path):
        self.got = path

    def list(self, path):
        self.got = path
        return []

    def exists(self, path):
        self.got = path
        return True


def test_validating_blocks_injection_before_backend():
    stub = _Stub()
    provider = ValidatingProvider(stub)
    with pytest.raises(InvalidPathError):
        provider.get(SecretPath(mount="secret", env="prod", app="app", key="--vault=evil"))
    assert stub.got is None, "malicious path must not reach the backend"

    clean = SecretPath(mount="secret", env="prod", app="app", key="db_password")
    assert provider.get(clean) == "value"
    assert stub.got == clean


def test_config_rejects_hostile_key(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text(textwrap.dedent("""
        backend: vault
        app: myapp
        env: prod
        keys:
          - "../../metadata/prod/other/leak"
    """))
    cfg = load_config(str(f))
    with pytest.raises(InvalidPathError):
        cfg.validate()


def test_config_rejects_oversized_file(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("app: myapp\nenv: prod\nbackend: vault\n# " + "a" * (1 << 20))
    with pytest.raises(ValueError, match="too large"):
        load_config(str(f))


def test_config_rejects_yaml_alias_bomb(tmp_path):
    # A billion-laughs alias bomb stays under the 1 MiB size cap but expands
    # enormously; the no-alias loader rejects it at parse time.
    f = tmp_path / ".kumokagi.yaml"
    f.write_text(textwrap.dedent("""
        backend: vault
        app: &a myapp
        env: *a
        keys: [*a, *a, *a]
    """))
    with pytest.raises(Exception):  # yaml.YAMLError
        load_config(str(f))


def test_config_rejects_onepassword_vault_slash():
    from kumokagi.config import Config

    cfg = Config(backend="onepassword", mount="Team/evil", app="myapp", env="prod")
    with pytest.raises(ValueError, match="must not contain"):
        cfg.validate()


def test_check_host_allowed(monkeypatch):
    from kumokagi.config import ENV_ALLOWED_VAULT_ADDRS, check_host_allowed

    # Unset => opt-in, anything allowed.
    monkeypatch.delenv(ENV_ALLOWED_VAULT_ADDRS, raising=False)
    check_host_allowed(ENV_ALLOWED_VAULT_ADDRS, "https://attacker.example.com")

    monkeypatch.setenv(ENV_ALLOWED_VAULT_ADDRS, "https://vault.corp.example.com, vault2.corp")
    check_host_allowed(ENV_ALLOWED_VAULT_ADDRS, "https://vault.corp.example.com/v1")
    check_host_allowed(ENV_ALLOWED_VAULT_ADDRS, "https://VAULT.corp.example.com")  # case-insensitive
    with pytest.raises(ValueError):
        check_host_allowed(ENV_ALLOWED_VAULT_ADDRS, "https://attacker.example.com")


def test_check_value_allowed(monkeypatch):
    from kumokagi.config import ENV_ALLOWED_GCP_PROJECTS, check_value_allowed

    monkeypatch.delenv(ENV_ALLOWED_GCP_PROJECTS, raising=False)
    check_value_allowed(ENV_ALLOWED_GCP_PROJECTS, "any-project")

    monkeypatch.setenv(ENV_ALLOWED_GCP_PROJECTS, "prod-secrets, staging-secrets")
    check_value_allowed(ENV_ALLOWED_GCP_PROJECTS, "prod-secrets")
    with pytest.raises(ValueError):
        check_value_allowed(ENV_ALLOWED_GCP_PROJECTS, "attacker-project")


def test_vault_provider_allowlist_blocks_redirect(monkeypatch):
    from kumokagi.config import ENV_ALLOWED_VAULT_ADDRS
    from kumokagi.vault import VaultProvider

    monkeypatch.setenv(ENV_ALLOWED_VAULT_ADDRS, "https://vault.corp.example.com")
    with pytest.raises(ValueError, match="allowlist"):
        VaultProvider(address="https://attacker.example.com")


def test_op_args_end_of_options_terminator():
    """Every op invocation must place a '--' terminator before untrusted
    operands, so an operand can never be parsed as an op flag."""
    calls: list[list[str]] = []

    def fake_run(cmd, *, capture_output, text, check=False):
        calls.append(cmd)
        import subprocess

        # item get succeeds so set/delete/list follow their normal path
        out = '{"fields":[]}' if "get" in cmd else "s3cr3t"
        return subprocess.CompletedProcess(cmd, 0, stdout=out, stderr="")

    from kumokagi.config import Config

    cfg = Config(backend="onepassword", mount="TestVault", app="myapp", env="prod")
    provider = OnePasswordProvider(cfg, _run=fake_run)
    path = SecretPath(mount="TestVault", env="prod", app="myapp", key="db_password")

    provider.get(path)
    provider.set(path, "v")
    provider.delete(path)
    provider.list(path)
    provider.exists(path)

    assert calls
    for cmd in calls:
        assert "--" in cmd, f"missing -- terminator: {cmd}"
        idx = cmd.index("--")
        operands = cmd[idx + 1:]
        assert operands, f"no operand after -- : {cmd}"
        for operand in operands:
            assert not operand.startswith("-"), f"operand looks like a flag: {operand!r}"
