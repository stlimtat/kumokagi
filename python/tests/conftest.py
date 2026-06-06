import pytest
import responses as resp


@pytest.fixture
def mock_vault():
    """Activate responses mock and register standard Vault KV v2 endpoints."""
    with resp.RequestsMock(assert_all_requests_are_fired=False) as rsps:
        base = "https://vault.test:8200/v1"

        rsps.add(
            resp.GET,
            f"{base}/secret/data/prod/myapp/db_password",
            json={"data": {"data": {"value": "s3cr3t"}, "metadata": {}}},
        )
        rsps.add(resp.GET, f"{base}/secret/data/prod/myapp/missing", status=404)
        rsps.add(
            resp.POST,
            f"{base}/secret/data/prod/myapp/newkey",
            json={"data": {}},
        )
        rsps.add(
            resp.DELETE,
            f"{base}/secret/metadata/prod/myapp/db_password",
            status=204,
        )
        rsps.add(
            resp.Response(
                method="LIST",
                url=f"{base}/secret/metadata/prod/myapp",
                json={"data": {"keys": ["db_password", "api_key"]}},
            )
        )
        rsps.add(
            resp.GET,
            f"{base}/secret/metadata/prod/myapp/db_password",
            json={"data": {"versions": {}}},
        )
        rsps.add(resp.GET, f"{base}/secret/metadata/prod/myapp/missing", status=404)

        yield rsps
