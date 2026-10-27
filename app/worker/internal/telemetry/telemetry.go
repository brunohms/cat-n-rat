// Package telemetry sets up the OpenTelemetry traces and metrics of the worker.
//
// Telemetry is only enabled when OTEL_EXPORTER_OTLP_ENDPOINT is set; otherwise the global providers stay
// no-ops. The exporters read the standard OTEL_* environment variables (endpoint, headers, temporality, ...).
package telemetry

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Setup installs the global tracer and meter providers and returns a function that flushes and stops them.
func Setup(ctx context.Context, serviceName string) (shutdown func(context.Context) error, enabled bool, err error) {
	// The propagator is installed even without an exporter, so trace context still flows between services
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	noop := func(context.Context) error { return nil }
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return noop, false, nil
	}

	// OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES override the default service name
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	)
	if err != nil {
		return noop, false, err
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return noop, false, err
	}
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter), sdktrace.WithResource(res))

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return noop, false, errors.Join(err, tracerProvider.Shutdown(ctx))
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	return func(ctx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}, true, nil
}
