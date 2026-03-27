package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/aillion-co/infrastructure-lz/internal/models"
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

// CostEstimateHandler handles cost estimation requests.
func CostEstimateHandler(w http.ResponseWriter, r *http.Request) {
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
		est := estimateFeature(f.FeatureID, f.Config)
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

// estimateFeature returns a cost estimate for a given feature based on typical GCP pricing.
func estimateFeature(featureID models.FeatureID, _ interface{}) CostEstimate {
	switch featureID {
	case models.FeatureBootstrapOrg:
		return CostEstimate{
			FeatureID:   featureID,
			FeatureName: "Bootstrap Organisation",
			Items: []CostLineItem{
				{Resource: "GCP Project", Description: "Management project (no direct cost)", MonthlyUSD: 0},
				{Resource: "Cloud NAT", Description: "NAT gateway per environment", MonthlyUSD: 32.12},
				{Resource: "Cloud Router", Description: "Router per environment", MonthlyUSD: 0},
				{Resource: "GKE Autopilot", Description: "Per-environment cluster management fee", MonthlyUSD: 73.00},
				{Resource: "VPC Network", Description: "Shared VPC (no base cost)", MonthlyUSD: 0},
			},
			MonthlyUSD: 105.12,
		}

	case models.FeatureBigQueryAnalytics:
		return CostEstimate{
			FeatureID:   featureID,
			FeatureName: "BigQuery Analytics",
			Items: []CostLineItem{
				{Resource: "BigQuery Storage", Description: "Active storage (first 10GB free), est. 100GB", MonthlyUSD: 2.00},
				{Resource: "BigQuery Queries", Description: "On-demand queries (first 1TB/mo free)", MonthlyUSD: 0},
				{Resource: "Data Transfer", Description: "Scheduled data transfer job", MonthlyUSD: 0},
			},
			MonthlyUSD: 2.00,
		}

	case models.FeatureDeveloperPortal:
		return CostEstimate{
			FeatureID:   featureID,
			FeatureName: "Dynamic Developer Portal",
			Items: []CostLineItem{
				{Resource: "Config Controller", Description: "GKE-based Config Controller cluster", MonthlyUSD: 73.00},
				{Resource: "Compute (Backstage)", Description: "Backstage workloads on Config Controller", MonthlyUSD: 50.00},
				{Resource: "Persistent Disk", Description: "PostgreSQL PVC (10GB SSD)", MonthlyUSD: 1.70},
			},
			MonthlyUSD: 124.70,
		}

	case models.FeatureHardenedImageBakery:
		return CostEstimate{
			FeatureID:   featureID,
			FeatureName: "Hardened Image Bakery",
			Items: []CostLineItem{
				{Resource: "Cloud Build", Description: "Build minutes (first 120 min/day free)", MonthlyUSD: 0},
				{Resource: "Artifact Registry", Description: "Image storage (first 500MB free)", MonthlyUSD: 0.50},
				{Resource: "Compute Engine", Description: "Temporary build VMs (per-build)", MonthlyUSD: 5.00},
			},
			MonthlyUSD: 5.50,
		}

	case models.FeatureSecureInferencing:
		return CostEstimate{
			FeatureID:   featureID,
			FeatureName: "Secure Inferencing",
			Items: []CostLineItem{
				{Resource: "Cloud Run", Description: "LiteLLM proxy (1 vCPU, 512Mi, min 1 instance)", MonthlyUSD: 25.00},
				{Resource: "Secret Manager", Description: "Master key secret (6 versions)", MonthlyUSD: 0.06},
				{Resource: "Vertex AI", Description: "Gemini API calls (usage-based, varies)", MonthlyUSD: 0},
			},
			MonthlyUSD: 25.06,
		}

	case models.FeatureSkaffoldAppDev:
		return CostEstimate{
			FeatureID:   featureID,
			FeatureName: "Skaffold Application Development",
			Items: []CostLineItem{
				{Resource: "GKE Autopilot", Description: "Cluster management fee", MonthlyUSD: 73.00},
				{Resource: "Compute", Description: "Autopilot pod resources (e2-standard-4 equiv)", MonthlyUSD: 95.00},
				{Resource: "Cloud SQL", Description: "PostgreSQL db-f1-micro (if enabled)", MonthlyUSD: 10.00},
			},
			MonthlyUSD: 178.00,
		}

	default:
		return CostEstimate{
			FeatureID:   featureID,
			FeatureName: string(featureID),
			MonthlyUSD:  0,
		}
	}
}
