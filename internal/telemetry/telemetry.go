package telemetry

import (
	"context"
	"log/slog"
)

// Setup initializes OpenTelemetry providers.
// When otlpEndpoint is empty, telemetry is disabled (noop).
func Setup(ctx context.Context, serviceName, otlpEndpoint string) (shutdown func(context.Context) error, err error) {
	if otlpEndpoint == "" {
		slog.Info("telemetry disabled (no OTEL_EXPORTER_OTLP_ENDPOINT set)")
		return func(context.Context) error { return nil }, nil
	}

	// TODO: Initialize OTLP exporter, tracer provider, meter provider
	// This is a placeholder — the full implementation will use:
	//   go.opentelemetry.io/otel
	//   go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
	//   go.opentelemetry.io/otel/sdk/trace
	//   go.opentelemetry.io/otel/sdk/metric

	slog.Info("telemetry initialized", "endpoint", otlpEndpoint, "service", serviceName)
	return func(context.Context) error { return nil }, nil
}
