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

type Generator struct {
	kccBuilder  *kcc.Builder
	helmBuilder *helm.Builder
}

func New() *Generator {
	return &Generator{
		kccBuilder:  kcc.NewBuilder(),
		helmBuilder: helm.NewBuilder(),
	}
}

func (g *Generator) Generate(ctx context.Context, cfg *models.ProjectConfig) ([]byte, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "generator.Generate")
	defer span.End()

	span.SetAttributes(
		attribute.String("project.name", cfg.ProjectName),
		attribute.String("project.id", cfg.ProjectID),
		attribute.String("project.region", cfg.Region),
		attribute.String("project.environment", cfg.Environment),
		attribute.Bool("project.has_gke", cfg.GKE != nil),
		attribute.Bool("project.has_cloudsql", cfg.CloudSQL != nil),
	)

	metrics := telemetry.GetMetrics()
	metrics.GenerateTotal.Add(ctx, 1)

	slog.InfoContext(ctx, "generating IAC", "project", cfg.ProjectName, "region", cfg.Region)

	// Generate KCC resource manifests
	resources, err := g.kccBuilder.Build(cfg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "kcc build failed")
		metrics.GenerateErrors.Add(ctx, 1)
		return nil, fmt.Errorf("building KCC resources: %w", err)
	}

	span.SetAttributes(attribute.Int("kcc.resource_count", len(resources)))
	metrics.KCCResourcesGenerated.Add(ctx, int64(len(resources)))

	// Wrap in Helm chart structure
	chart, err := g.helmBuilder.Build(cfg, resources)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "helm build failed")
		metrics.GenerateErrors.Add(ctx, 1)
		return nil, fmt.Errorf("building Helm chart: %w", err)
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

	slog.InfoContext(ctx, "generation complete", "project", cfg.ProjectName, "files", len(chart))
	return buf.Bytes(), nil
}
