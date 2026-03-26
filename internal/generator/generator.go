package generator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/aillion-co/infrastructure-lz/internal/generator/helm"
	"github.com/aillion-co/infrastructure-lz/internal/generator/kcc"
	"github.com/aillion-co/infrastructure-lz/internal/models"
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
	slog.Info("generating IAC", "project", cfg.ProjectName, "region", cfg.Region)

	// Generate KCC resource manifests
	resources, err := g.kccBuilder.Build(cfg)
	if err != nil {
		return nil, fmt.Errorf("building KCC resources: %w", err)
	}

	// Wrap in Helm chart structure
	chart, err := g.helmBuilder.Build(cfg, resources)
	if err != nil {
		return nil, fmt.Errorf("building Helm chart: %w", err)
	}

	// Package as zip
	var buf bytes.Buffer
	if err := archive.WriteZip(&buf, chart); err != nil {
		return nil, fmt.Errorf("creating zip archive: %w", err)
	}

	slog.Info("generation complete", "project", cfg.ProjectName, "files", len(chart))
	return buf.Bytes(), nil
}
