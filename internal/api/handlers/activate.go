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
		json.NewEncoder(w).Encode(map[string]any{"errors": errs})
		return
	}

	zipData, err := h.gen.GenerateActivation(ctx, &req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "activation generation failed")
		slog.ErrorContext(ctx, "failed to generate activation", "error", err, "customer", req.Customer.CustomerName)
		http.Error(w, "activation generation failed", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("%s-activation.zip", req.Customer.CustomerName)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(zipData)
}

func validateActivationRequest(req *models.ActivationRequest) []string {
	var errs []string

	if req.Customer.CustomerName == "" {
		errs = append(errs, "customer name is required")
	}
	if req.Customer.ContactEmail == "" {
		errs = append(errs, "contact email is required")
	}

	enabledCount := 0
	for _, f := range req.Features {
		if f.Enabled {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		errs = append(errs, "at least one feature must be enabled")
	}

	return errs
}

// FeaturesHandler returns the available feature registry.
func FeaturesHandler(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.Tracer().Start(r.Context(), "handler.Features")
	defer span.End()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.FeatureRegistry())
}
