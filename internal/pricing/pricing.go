// Package pricing provides cost estimation for activation features.
//
// The Provider interface allows pluggable backends — a static provider
// for offline use and tests, and a catalog provider that pulls live
// list pricing from the GCP Cloud Billing Catalog API.
package pricing

import (
	"context"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// LineItem is a single billable component within a feature's cost estimate.
type LineItem struct {
	Resource    string
	Description string
	MonthlyUSD  float64
}

// FeatureCost is the monthly cost estimate for a single feature.
type FeatureCost struct {
	FeatureID   models.FeatureID
	FeatureName string
	Items       []LineItem
	MonthlyUSD  float64
}

// Provider returns cost estimates for activation features.
type Provider interface {
	EstimateFeature(ctx context.Context, featureID models.FeatureID) FeatureCost
}
