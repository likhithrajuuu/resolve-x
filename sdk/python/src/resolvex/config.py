"""Configuration resolution: explicit arguments beat environment variables."""
from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import Mapping, Optional


@dataclass(frozen=True)
class Config:
    api_key: str
    service_name: str
    environment: str
    version: Optional[str]
    endpoint: str
    api_url: str
    auto_instrument: bool
    capture_logging: bool
    metric_interval_ms: int
    resource_attributes: Mapping[str, str] = field(default_factory=dict)


def _truthy(v: Optional[str]) -> bool:
    return v in ("1", "true", "True")


def resolve(
    api_key: Optional[str] = None,
    service_name: Optional[str] = None,
    environment: Optional[str] = None,
    version: Optional[str] = None,
    endpoint: Optional[str] = None,
    api_url: Optional[str] = None,
    auto_instrument: Optional[bool] = None,
    capture_logging: Optional[bool] = None,
    metric_interval_ms: int = 15000,
    resource_attributes: Optional[Mapping[str, str]] = None,
    env: Optional[Mapping[str, str]] = None,
) -> Config:
    env = os.environ if env is None else env
    key = api_key or env.get("RESOLVEX_API_KEY")
    if not key:
        raise ValueError("resolvex: api_key is required (pass api_key= or set RESOLVEX_API_KEY)")
    name = service_name or env.get("RESOLVEX_SERVICE") or env.get("OTEL_SERVICE_NAME")
    if not name:
        raise ValueError("resolvex: service_name is required (pass service_name= or set RESOLVEX_SERVICE)")
    return Config(
        api_key=key,
        service_name=name,
        environment=environment or env.get("RESOLVEX_ENVIRONMENT") or "development",
        version=version or env.get("RESOLVEX_VERSION"),
        endpoint=(endpoint or env.get("RESOLVEX_ENDPOINT") or "http://localhost:4318").rstrip("/"),
        api_url=(api_url or env.get("RESOLVEX_API_URL") or "http://localhost:8000").rstrip("/"),
        auto_instrument=auto_instrument if auto_instrument is not None else env.get("RESOLVEX_AUTO_INSTRUMENT") != "false",
        capture_logging=capture_logging if capture_logging is not None else env.get("RESOLVEX_CAPTURE_LOGGING") != "false",
        metric_interval_ms=metric_interval_ms,
        resource_attributes=dict(resource_attributes or {}),
    )
