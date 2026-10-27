import logging
import os

from fastapi import FastAPI
from opentelemetry import metrics, trace
from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

logger = logging.getLogger(__name__)


def setup(app: FastAPI, service_name: str = "generator") -> bool:
    """Exports traces and metrics with OTLP/HTTP when OTEL_EXPORTER_OTLP_ENDPOINT is set.

    The exporters read the standard OTEL_* environment variables. Without an endpoint nothing is installed
    and the OpenTelemetry API stays a no-op. Returns whether telemetry was enabled.
    """
    if not os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT"):
        return False

    # OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES override the default service name
    resource = Resource.create({"service.name": os.getenv("OTEL_SERVICE_NAME", service_name)})

    tracer_provider = TracerProvider(resource=resource)
    tracer_provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
    trace.set_tracer_provider(tracer_provider)

    meter_provider = MeterProvider(resource=resource, metric_readers=[PeriodicExportingMetricReader(OTLPMetricExporter())])
    metrics.set_meter_provider(meter_provider)

    # Probes hit /health every few seconds and would drown the useful traces
    # (and the "http send"/"http receive" internal spans add nothing to the request span)
    FastAPIInstrumentor.instrument_app(
        app,
        excluded_urls="health",
        exclude_spans=["send", "receive"],
        tracer_provider=tracer_provider,
        meter_provider=meter_provider,
    )
    # Instruments every httpx client created from now on, propagating the trace context to the backend
    HTTPXClientInstrumentor().instrument(tracer_provider=tracer_provider, meter_provider=meter_provider)

    logger.info("Telemetry enabled: endpoint=%s", os.environ["OTEL_EXPORTER_OTLP_ENDPOINT"])
    return True


def shutdown() -> None:
    """Flushes the pending spans and metrics."""
    for provider in (trace.get_tracer_provider(), metrics.get_meter_provider()):
        if hasattr(provider, "shutdown"):
            provider.shutdown()
