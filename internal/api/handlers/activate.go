package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/generator/kcc"
	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
)

// ActivateHandler handles activation generation requests.
type ActivateHandler struct {
	gen *generator.ActivationGenerator
}

// NewActivateHandler creates an ActivateHandler.
func NewActivateHandler(gen *generator.ActivationGenerator) *ActivateHandler {
	return &ActivateHandler{gen: gen}
}

func (h *ActivateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.Tracer().Start(r.Context(), "handler.Activate")
	defer span.End()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.ActivationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid request body")
		slog.ErrorContext(ctx, "failed to decode activation request", "error", err)
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("customer.name", req.Customer.CustomerName),
		attribute.Int("features.count", len(req.Features)),
	)

	// Validate request
	if errs := validateActivationRequest(&req); len(errs) > 0 {
		span.SetAttributes(attribute.Int("validation.error_count", len(errs)))
		span.SetStatus(codes.Error, "validation failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": errs})
		return
	}

	zipData, err := h.gen.GenerateActivation(ctx, &req)
	if err != nil {
		span.RecordError(err)
		var validationErr *kcc.ValidationError
		if errors.As(err, &validationErr) {
			span.SetStatus(codes.Error, "feature config validation failed")
			slog.WarnContext(ctx, "activation feature config invalid", "error", err, "customer", req.Customer.CustomerName)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":   err.Error(),
				"feature": validationErr.Feature,
				"field":   validationErr.Field,
			})
			return
		}
		span.SetStatus(codes.Error, "activation generation failed")
		slog.ErrorContext(ctx, "failed to generate activation", "error", err, "customer", req.Customer.CustomerName)
		http.Error(w, "activation generation failed", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("%s-activation.zip", req.Customer.CustomerName)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if _, err := w.Write(zipData); err != nil {
		slog.WarnContext(ctx, "failed to write activation zip", "error", err)
	}
}

func validateActivationRequest(req *models.ActivationRequest) []string {
	var errs []string

	if req.Customer.CustomerName == "" {
		errs = append(errs, "customer name is required")
	}
	if req.Customer.ContactEmail == "" {
		errs = append(errs, "contact email is required")
	}

	enabled := make(map[models.FeatureID]bool, len(req.Features))
	for _, f := range req.Features {
		if f.Enabled {
			enabled[f.FeatureID] = true
		}
	}
	if len(enabled) == 0 {
		errs = append(errs, "at least one feature must be enabled")
	}

	// Enforce declared feature dependencies (e.g. everything depends on
	// bootstrap-org) so generated charts never contain dangling references.
	for _, meta := range models.FeatureRegistry() {
		if !enabled[meta.ID] {
			continue
		}
		for _, dep := range meta.DependsOn {
			if !enabled[dep] {
				errs = append(errs, fmt.Sprintf("feature %q requires feature %q to be enabled", meta.ID, dep))
			}
		}
	}

	return errs
}

// FeaturesHandler returns the available feature registry.
func FeaturesHandler(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.Tracer().Start(r.Context(), "handler.Features")
	defer span.End()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models.FeatureRegistry())
}
