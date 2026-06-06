import textwrap

import pytest

from kumokagi.config import load_config


def test_load_full_config(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text(textwrap.dedent("""
        backend: vault
        mount: secret
        app: myapp
        env: prod
        keys:
          - db_password
          - api_key
        vault:
          address: https://vault.example.com
    """))
    cfg = load_config(str(f))
    assert cfg.backend == "vault"
    assert cfg.mount == "secret"
    assert cfg.app == "myapp"
    assert cfg.env == "prod"
    assert cfg.keys == ["db_password", "api_key"]
    assert cfg.vault.address == "https://vault.example.com"


def test_load_defaults_mount(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: vault\napp: myapp\nenv: prod\n")
    cfg = load_config(str(f))
    assert cfg.mount == "secret"


def test_env_var_overrides_config_env(tmp_path, monkeypatch):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: vault\napp: myapp\nenv: prod\n")
    monkeypatch.setenv("KUMOKAGI_ENV", "staging")
    cfg = load_config(str(f))
    assert cfg.env == "staging"


def test_load_missing_file():
    with pytest.raises(FileNotFoundError):
        load_config("/nonexistent/.kumokagi.yaml")


def test_validate_missing_backend(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("app: myapp\nenv: prod\n")
    cfg = load_config(str(f))
    with pytest.raises(ValueError, match="backend"):
        cfg.validate()


def test_validate_missing_app(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: vault\nenv: prod\n")
    cfg = load_config(str(f))
    with pytest.raises(ValueError, match="app"):
        cfg.validate()


def test_validate_missing_env(tmp_path):
    f = tmp_path / ".kumokagi.yaml"
    f.write_text("backend: vault\napp: myapp\n")
    cfg = load_config(str(f))
    with pytest.raises(ValueError, match="env"):
        cfg.validate()
