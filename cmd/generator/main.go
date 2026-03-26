package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aillion-co/infrastructure-lz/internal/config"
	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/models"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: generator <config.json> [output.zip]")
	}

	inputPath := os.Args[1]
	outputPath := "output.zip"
	if len(os.Args) >= 3 {
		outputPath = os.Args[2]
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	var cfg models.ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	gen := generator.New()
	zipData, err := gen.Generate(context.Background(), &cfg)
	if err != nil {
		return fmt.Errorf("generating: %w", err)
	}

	if err := os.WriteFile(outputPath, zipData, 0o644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	slog.Info("generated",
		"output", outputPath,
		"version", config.Version,
		"project", cfg.ProjectName,
	)
	return nil
}
