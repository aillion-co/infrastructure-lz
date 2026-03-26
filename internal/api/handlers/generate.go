package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
	"github.com/aillion-co/infrastructure-lz/pkg/validator"
)

type GenerateHandler struct {
	gen *generator.Generator
}

func NewGenerateHandler(gen *generator.Generator) *GenerateHandler {
	return &GenerateHandler{gen: gen}
}

func (h *GenerateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.Tracer().Start(r.Context(), "handler.Generate")
	defer span.End()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg models.ProjectConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid request body")
		slog.ErrorContext(ctx, "failed to decode request", "error", err)
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("project.name", cfg.ProjectName),
		attribute.String("project.id", cfg.ProjectID),
	)

	if errs := validator.ValidateProjectConfig(&cfg); len(errs) > 0 {
		span.SetAttributes(attribute.Int("validation.error_count", len(errs)))
		span.SetStatus(codes.Error, "validation failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{"errors": errs})
		return
	}

	zipData, err := h.gen.Generate(ctx, &cfg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "generation failed")
		slog.ErrorContext(ctx, "failed to generate IAC", "error", err, "project", cfg.ProjectName)
		http.Error(w, "generation failed", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("%s-iac.zip", cfg.ProjectName)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(zipData)
}
