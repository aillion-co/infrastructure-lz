package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/aillion-co/infrastructure-lz/internal/pricing"
	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
)

// CostEstimate represents a monthly cost estimate for a single feature.
type CostEstimate struct {
	FeatureID   models.FeatureID `json:"featureId"`
	FeatureName string           `json:"featureName"`
	Items       []CostLineItem   `json:"items"`
	MonthlyUSD  float64          `json:"monthlyUsd"`
}

// CostLineItem is a single billable component within a feature.
type CostLineItem struct {
	Resource    string  `json:"resource"`
	Description string  `json:"description"`
	MonthlyUSD  float64 `json:"monthlyUsd"`
}

// CostEstimateResponse is the full cost estimation response.
type CostEstimateResponse struct {
	Features     []CostEstimate `json:"features"`
	TotalMonthly float64        `json:"totalMonthlyUsd"`
	Currency     string         `json:"currency"`
	Disclaimer   string         `json:"disclaimer"`
}

// CostHandler estimates activation costs using a pluggable pricing provider.
type CostHandler struct {
	provider pricing.Provider
}

// NewCostHandler constructs a CostHandler backed by the given provider.
func NewCostHandler(provider pricing.Provider) *CostHandler {
	return &CostHandler{provider: provider}
}

// ServeHTTP handles POST /api/v1/cost-estimate.
func (h *CostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.Tracer().Start(r.Context(), "handlers.CostEstimate")
	defer span.End()

	var req models.ActivationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid request body")
		slog.ErrorContext(ctx, "failed to decode cost estimate request", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("customer.name", req.Customer.CustomerName),
		attribute.Int("features.count", len(req.Features)),
	)

	var estimates []CostEstimate
	var total float64

	for _, f := range req.Features {
		if !f.Enabled {
			continue
		}
		fc := h.provider.EstimateFeature(ctx, f.FeatureID)
		est := toCostEstimate(fc)
		total += est.MonthlyUSD
		estimates = append(estimates, est)
	}

	span.SetAttributes(attribute.Float64("estimate.total_monthly_usd", total))

	resp := CostEstimateResponse{
		Features:     estimates,
		TotalMonthly: total,
		Currency:     "USD",
		Disclaimer:   "Estimates are approximate and based on list pricing. Actual costs may vary based on usage, committed use discounts, and negotiated pricing.",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(ctx, "failed to encode cost estimate response", "error", err)
	}
}

// toCostEstimate adapts the pricing package's FeatureCost into the
// JSON shape published on the cost-estimate endpoint.
func toCostEstimate(fc pricing.FeatureCost) CostEstimate {
	items := make([]CostLineItem, 0, len(fc.Items))
	for _, i := range fc.Items {
		items = append(items, CostLineItem{
			Resource:    i.Resource,
			Description: i.Description,
			MonthlyUSD:  i.MonthlyUSD,
		})
	}
	return CostEstimate{
		FeatureID:   fc.FeatureID,
		FeatureName: fc.FeatureName,
		Items:       items,
		MonthlyUSD:  fc.MonthlyUSD,
	}
}
