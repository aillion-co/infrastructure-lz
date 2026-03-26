package generator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/aillion-co/infrastructure-lz/internal/generator/helm"
	"github.com/aillion-co/infrastructure-lz/internal/generator/kcc"
	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
	"github.com/aillion-co/infrastructure-lz/pkg/archive"
)

// ActivationGenerator handles generation for the full activation system with
// multiple feature types and per-feature Helm sub-charts.
type ActivationGenerator struct {
	registry        *kcc.FeatureBuilderRegistry
	activationBuilder *helm.ActivationBuilder
}

// NewActivationGenerator creates an ActivationGenerator with all feature builders registered.
func NewActivationGenerator() *ActivationGenerator {
	return &ActivationGenerator{
		registry:          kcc.NewFeatureBuilderRegistry(),
		activationBuilder: helm.NewActivationBuilder(),
	}
}

// GenerateActivation generates a zip containing Helm charts for all enabled features.
func (g *ActivationGenerator) GenerateActivation(ctx context.Context, req *models.ActivationRequest) ([]byte, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "generator.GenerateActivation")
	defer span.End()

	enabledCount := 0
	for _, f := range req.Features {
		if f.Enabled {
			enabledCount++
		}
	}

	span.SetAttributes(
		attribute.String("customer.name", req.Customer.CustomerName),
		attribute.Int("features.total", len(req.Features)),
		attribute.Int("features.enabled", enabledCount),
	)

	metrics := telemetry.GetMetrics()
	metrics.GenerateTotal.Add(ctx, 1)

	slog.InfoContext(ctx, "generating activation",
		"customer", req.Customer.CustomerName,
		"features_enabled", enabledCount,
	)

	// Build KCC resources for each enabled feature
	resourcesByFeature, err := g.registry.BuildAll(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "kcc build failed")
		metrics.GenerateErrors.Add(ctx, 1)
		return nil, fmt.Errorf("building KCC resources: %w", err)
	}

	totalResources := 0
	for _, resources := range resourcesByFeature {
		totalResources += len(resources)
	}
	span.SetAttributes(attribute.Int("kcc.resource_count", totalResources))
	metrics.KCCResourcesGenerated.Add(ctx, int64(totalResources))

	// Build Helm charts (umbrella + sub-charts per feature)
	chart, err := g.activationBuilder.BuildActivation(req, resourcesByFeature)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "helm build failed")
		metrics.GenerateErrors.Add(ctx, 1)
		return nil, fmt.Errorf("building Helm charts: %w", err)
	}

	// Package as zip
	var buf bytes.Buffer
	if err := archive.WriteZip(&buf, chart); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "zip creation failed")
		metrics.GenerateErrors.Add(ctx, 1)
		return nil, fmt.Errorf("creating zip archive: %w", err)
	}

	span.SetAttributes(
		attribute.Int("output.file_count", len(chart)),
		attribute.Int("output.zip_bytes", buf.Len()),
	)
	metrics.GenerateZipSize.Record(ctx, int64(buf.Len()))

	slog.InfoContext(ctx, "activation generation complete",
		"customer", req.Customer.CustomerName,
		"features", enabledCount,
		"files", len(chart),
	)

	return buf.Bytes(), nil
}
