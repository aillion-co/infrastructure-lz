package telemetry

import (
	"errors"
	"log/slog"
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

		var errs []error
		var err error

		appMetrics.HTTPRequestsTotal, err = m.Int64Counter("http.server.request.total",
			metric.WithDescription("Total HTTP requests"),
			metric.WithUnit("{request}"),
		)
		errs = append(errs, err)

		appMetrics.HTTPRequestDuration, err = m.Float64Histogram("http.server.request.duration",
			metric.WithDescription("HTTP request duration in seconds"),
			metric.WithUnit("s"),
		)
		errs = append(errs, err)

		appMetrics.HTTPActiveRequests, err = m.Int64UpDownCounter("http.server.active_requests",
			metric.WithDescription("Number of active HTTP requests"),
			metric.WithUnit("{request}"),
		)
		errs = append(errs, err)

		appMetrics.GenerateTotal, err = m.Int64Counter("generator.generate.total",
			metric.WithDescription("Total IAC generation requests"),
			metric.WithUnit("{request}"),
		)
		errs = append(errs, err)

		appMetrics.GenerateDuration, err = m.Float64Histogram("generator.generate.duration",
			metric.WithDescription("IAC generation duration in seconds"),
			metric.WithUnit("s"),
		)
		errs = append(errs, err)

		appMetrics.GenerateErrors, err = m.Int64Counter("generator.generate.errors",
			metric.WithDescription("Total IAC generation errors"),
			metric.WithUnit("{error}"),
		)
		errs = append(errs, err)

		appMetrics.GenerateZipSize, err = m.Int64Histogram("generator.generate.zip_size",
			metric.WithDescription("Size of generated zip files in bytes"),
			metric.WithUnit("By"),
		)
		errs = append(errs, err)

		appMetrics.KCCResourcesGenerated, err = m.Int64Counter("generator.kcc.resources_generated",
			metric.WithDescription("Total KCC resources generated"),
			metric.WithUnit("{resource}"),
		)
		errs = append(errs, err)

		// The OTEL API returns usable no-op instruments alongside any
		// error, so failures degrade metrics rather than crash — but
		// they should not be silent.
		if err := errors.Join(errs...); err != nil {
			slog.Warn("failed to register some metrics", "error", err)
		}
	})
	return appMetrics
}
