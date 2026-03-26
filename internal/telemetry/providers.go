package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/aillion-co/infrastructure-lz"

// Tracer returns the application tracer. Use this in all packages to create spans.
func Tracer() trace.Tracer {
	return otel.Tracer(instrumentationName)
}

// Meter returns the application meter. Use this in all packages to record metrics.
func Meter() metric.Meter {
	return otel.Meter(instrumentationName)
}
