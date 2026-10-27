from fastapi import FastAPI

from app import telemetry


def test_setup_is_disabled_without_endpoint(monkeypatch):
    monkeypatch.delenv("OTEL_EXPORTER_OTLP_ENDPOINT", raising=False)
    app = FastAPI()
    assert telemetry.setup(app) is False
    assert not getattr(app, "_is_instrumented_by_opentelemetry", False)
