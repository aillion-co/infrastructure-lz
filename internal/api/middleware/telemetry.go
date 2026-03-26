package middleware

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
)

// Telemetry creates spans, records metrics, and propagates trace context for every HTTP request.
func Telemetry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from incoming headers
		ctx := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		tracer := telemetry.Tracer()
		spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := tracer.Start(ctx, spanName)
		defer span.End()

		// Set standard HTTP span attributes
		span.SetAttributes(
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.URLPath(r.URL.Path),
			semconv.ServerAddress(r.Host),
			semconv.UserAgentOriginal(r.UserAgent()),
		)

		metrics := telemetry.GetMetrics()
		metrics.HTTPActiveRequests.Add(ctx, 1)
		defer metrics.HTTPActiveRequests.Add(ctx, -1)

		start := time.Now()
		wrapped := &wrappedWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r.WithContext(ctx))

		duration := time.Since(start).Seconds()
		statusCode := wrapped.statusCode

		// Record span status
		span.SetAttributes(semconv.HTTPResponseStatusCode(statusCode))
		if statusCode >= 500 {
			span.SetStatus(codes.Error, http.StatusText(statusCode))
		}

		// Record metrics
		attrOpt := otelmetric.WithAttributes(
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.HTTPResponseStatusCode(statusCode),
			attribute.String("http.route", r.URL.Path),
		)
		metrics.HTTPRequestsTotal.Add(ctx, 1, attrOpt)
		metrics.HTTPRequestDuration.Record(ctx, duration, attrOpt)

		// Inject trace context into response headers
		propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(w.Header()))
	})
}
