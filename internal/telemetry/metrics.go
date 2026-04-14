package telemetry

import (
	"sync"

	"go.opentelemetry.io/otel/metric"
)

// Metrics holds pre-registered application metrics.
// Access via GetMetrics() after telemetry.Setup() has been called.
type Metrics struct {
	// HTTP
	HTTPRequestsTotal   metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram
	HTTPActiveRequests  metric.Int64UpDownCounter

	// Generator
	GenerateTotal    metric.Int64Counter
	GenerateDuration metric.Float64Histogram
	GenerateErrors   metric.Int64Counter
	GenerateZipSize  metric.Int64Histogram

	// KCC
	KCCResourcesGenerated metric.Int64Counter
}

var (
	appMetrics  *Metrics
	metricsOnce sync.Once
)

// GetMetrics returns the singleton application metrics, initializing them on first call.
func GetMetrics() *Metrics {
	metricsOnce.Do(func() {
		m := Meter()
		appMetrics = &Metrics{}

		appMetrics.HTTPRequestsTotal, _ = m.Int64Counter("http.server.request.total",
			metric.WithDescription("Total HTTP requests"),
			metric.WithUnit("{request}"),
		)

		appMetrics.HTTPRequestDuration, _ = m.Float64Histogram("http.server.request.duration",
			metric.WithDescription("HTTP request duration in seconds"),
			metric.WithUnit("s"),
		)

		appMetrics.HTTPActiveRequests, _ = m.Int64UpDownCounter("http.server.active_requests",
			metric.WithDescription("Number of active HTTP requests"),
			metric.WithUnit("{request}"),
		)

		appMetrics.GenerateTotal, _ = m.Int64Counter("generator.generate.total",
			metric.WithDescription("Total IAC generation requests"),
			metric.WithUnit("{request}"),
		)

		appMetrics.GenerateDuration, _ = m.Float64Histogram("generator.generate.duration",
			metric.WithDescription("IAC generation duration in seconds"),
			metric.WithUnit("s"),
		)

		appMetrics.GenerateErrors, _ = m.Int64Counter("generator.generate.errors",
			metric.WithDescription("Total IAC generation errors"),
			metric.WithUnit("{error}"),
		)

		appMetrics.GenerateZipSize, _ = m.Int64Histogram("generator.generate.zip_size",
			metric.WithDescription("Size of generated zip files in bytes"),
			metric.WithUnit("By"),
		)

		appMetrics.KCCResourcesGenerated, _ = m.Int64Counter("generator.kcc.resources_generated",
			metric.WithDescription("Total KCC resources generated"),
			metric.WithUnit("{resource}"),
		)
	})
	return appMetrics
}
