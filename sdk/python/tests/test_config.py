import pytest

from resolvex.config import resolve


def test_requires_key_and_service():
    with pytest.raises(ValueError, match="api_key"):
        resolve(env={})
    with pytest.raises(ValueError, match="service_name"):
        resolve(api_key="k", env={})


def test_env_and_defaults():
    c = resolve(env={"RESOLVEX_API_KEY": "k", "RESOLVEX_SERVICE": "svc", "RESOLVEX_ENDPOINT": "https://i.example.com//"})
    assert c.endpoint == "https://i.example.com"
    assert c.environment == "development" and c.auto_instrument and c.capture_logging


def test_explicit_beats_env():
    c = resolve(api_key="a", service_name="s", environment="prod", env={"RESOLVEX_API_KEY": "b", "RESOLVEX_ENVIRONMENT": "x"})
    assert (c.api_key, c.environment) == ("a", "prod")
