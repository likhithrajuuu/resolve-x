"""Resolve-X SDK for Python.

    import resolvex
    resolvex.init(api_key="rx_...", service_name="checkout", version="1.4.2")

Sends traces, metrics and logs to Resolve-X over standard OTLP/HTTP. Every
installed OpenTelemetry instrumentation (Flask, FastAPI, requests, SQLAlchemy,
psycopg2, redis, ...) is enabled automatically.
"""
from __future__ import annotations

import functools
import json
import logging
import threading
import urllib.request
from typing import Any, Callable, Mapping, Optional, TypeVar

from opentelemetry import metrics, trace
from opentelemetry._logs import set_logger_provider
from opentelemetry.exporter.otlp.proto.http._log_exporter import OTLPLogExporter
from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.trace import Status, StatusCode

from .config import Config, resolve

__all__ = ["init", "shutdown", "mark_deployment", "traced", "span", "get_logger", "Config"]
__version__ = "0.1.0"

_lock = threading.Lock()
_state: dict[str, Any] = {}
_log = logging.getLogger("resolvex")


def init(
    api_key: Optional[str] = None,
    service_name: Optional[str] = None,
    *,
    environment: Optional[str] = None,
    version: Optional[str] = None,
    endpoint: Optional[str] = None,
    api_url: Optional[str] = None,
    auto_instrument: Optional[bool] = None,
    capture_logging: Optional[bool] = None,
    mark_deployment_on_start: bool = False,
    metric_interval_ms: int = 15000,
    resource_attributes: Optional[Mapping[str, str]] = None,
) -> Config:
    """Start telemetry. Idempotent: a second call returns the active config."""
    with _lock:
        if "config" in _state:
            return _state["config"]
        cfg = resolve(api_key, service_name, environment, version, endpoint, api_url, auto_instrument,
                      capture_logging, metric_interval_ms, resource_attributes)
        headers = {"x-resolvex-key": cfg.api_key}

        attrs = {
            "service.name": cfg.service_name,
            "deployment.environment": cfg.environment,
            "telemetry.distro.name": "resolve-x",
            "telemetry.distro.version": __version__,
            **cfg.resource_attributes,
        }
        if cfg.version:
            attrs["service.version"] = cfg.version
        resource = Resource.create(attrs)

        tracer_provider = TracerProvider(resource=resource)
        tracer_provider.add_span_processor(
            BatchSpanProcessor(OTLPSpanExporter(endpoint=f"{cfg.endpoint}/v1/traces", headers=headers)))
        trace.set_tracer_provider(tracer_provider)

        reader = PeriodicExportingMetricReader(
            OTLPMetricExporter(endpoint=f"{cfg.endpoint}/v1/metrics", headers=headers),
            export_interval_millis=cfg.metric_interval_ms)
        meter_provider = MeterProvider(resource=resource, metric_readers=[reader])
        metrics.set_meter_provider(meter_provider)

        logger_provider = LoggerProvider(resource=resource)
        logger_provider.add_log_record_processor(
            BatchLogRecordProcessor(OTLPLogExporter(endpoint=f"{cfg.endpoint}/v1/logs", headers=headers)))
        set_logger_provider(logger_provider)
        handler = None
        if cfg.capture_logging:
            # Standard `logging` calls become log records linked to the active trace.
            handler = LoggingHandler(level=logging.INFO, logger_provider=logger_provider)
            logging.getLogger().addHandler(handler)
            if logging.getLogger().level in (logging.NOTSET, logging.WARNING):
                logging.getLogger().setLevel(logging.INFO)

        _state.update(config=cfg, tracer_provider=tracer_provider, meter_provider=meter_provider,
                      logger_provider=logger_provider, handler=handler)
        if cfg.auto_instrument:
            _instrument_all()

    if mark_deployment_on_start and cfg.version:
        try:
            mark_deployment()
        except Exception as e:  # never break the app because telemetry failed
            _log.warning("deployment marker failed: %s", e)
    return cfg


def _instrument_all() -> None:
    """Enable every installed OpenTelemetry instrumentation via its entry point."""
    from importlib.metadata import entry_points

    try:
        eps = entry_points(group="opentelemetry_instrumentor")
    except TypeError:  # Python < 3.10 API
        eps = entry_points().get("opentelemetry_instrumentor", [])
    enabled = []
    for ep in eps:
        try:
            ep.load()().instrument()
            enabled.append(ep.name)
        except Exception as e:
            _log.debug("skipping instrumentation %s: %s", ep.name, e)
    _state["instrumented"] = enabled


def shutdown() -> None:
    """Flush buffered telemetry. Called automatically at interpreter exit."""
    with _lock:
        if "config" not in _state:
            return
        state = dict(_state)
        _state.clear()
    if state.get("handler"):
        logging.getLogger().removeHandler(state["handler"])
    for key in ("tracer_provider", "meter_provider", "logger_provider"):
        try:
            state[key].shutdown()
        except Exception as e:
            _log.debug("shutdown %s: %s", key, e)


import atexit  # noqa: E402

atexit.register(shutdown)


def mark_deployment(service: Optional[str] = None, version: Optional[str] = None, environment: Optional[str] = None) -> None:
    """Record a deployment so incident analysis can connect a degradation to a release."""
    cfg: Optional[Config] = _state.get("config")
    if cfg is None:
        raise RuntimeError("resolvex: call init() first")
    body = {"service": service or cfg.service_name, "version": version or cfg.version, "environment": environment or cfg.environment}
    if not body["version"]:
        raise ValueError("resolvex: mark_deployment needs a version")
    req = urllib.request.Request(
        f"{cfg.api_url}/api/v1/webhooks/deployments", data=json.dumps(body).encode(), method="POST",
        headers={"content-type": "application/json", "x-resolvex-key": cfg.api_key})
    with urllib.request.urlopen(req, timeout=5) as resp:
        if resp.status >= 300:
            raise RuntimeError(f"resolvex: deployment marker rejected: HTTP {resp.status}")


def get_logger(name: str = "app") -> logging.Logger:
    """A standard logger; records are exported and linked to the current trace."""
    return logging.getLogger(name)


F = TypeVar("F", bound=Callable[..., Any])


def span(name: str, **attributes: Any):
    """Context manager: `with resolvex.span("charge-card", order_id=1): ...`"""
    return trace.get_tracer("resolvex").start_as_current_span(name, attributes=attributes or None,
                                                              record_exception=True, set_status_on_exception=True)


def traced(name: Optional[str] = None) -> Callable[[F], F]:
    """Decorator that wraps a function in a span."""
    def deco(fn: F) -> F:
        @functools.wraps(fn)
        def wrapper(*a: Any, **kw: Any):
            with span(name or fn.__qualname__):
                return fn(*a, **kw)
        return wrapper  # type: ignore[return-value]
    return deco
