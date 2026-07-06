package pricing

import (
	"context"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// StaticProvider returns hardcoded list-price estimates. It is the default
// provider and acts as a fallback for the catalog provider.
type StaticProvider struct{}

// NewStaticProvider constructs a StaticProvider.
func NewStaticProvider() *StaticProvider {
	return &StaticProvider{}
}

// EstimateFeature returns a static cost estimate for the given feature.
func (p *StaticProvider) EstimateFeature(_ context.Context, featureID models.FeatureID) FeatureCost {
	switch featureID {
	case models.FeatureBootstrapOrg:
		return FeatureCost{
			FeatureID:   featureID,
			FeatureName: "Bootstrap Organisation",
			Items: []LineItem{
				{Resource: "GCP Project", Description: "Management project (no direct cost)", MonthlyUSD: 0},
				{Resource: "Cloud NAT", Description: "NAT gateway per environment", MonthlyUSD: 32.12},
				{Resource: "Cloud Router", Description: "Router per environment", MonthlyUSD: 0},
				{Resource: "GKE Autopilot", Description: "Per-environment cluster management fee", MonthlyUSD: 73.00},
				{Resource: "VPC Network", Description: "Shared VPC (no base cost)", MonthlyUSD: 0},
			},
			MonthlyUSD: 105.12,
		}

	case models.FeatureBigQueryAnalytics:
		return FeatureCost{
			FeatureID:   featureID,
			FeatureName: "BigQuery Analytics",
			Items: []LineItem{
				{Resource: "BigQuery Storage", Description: "Active storage (first 10GB free), est. 100GB", MonthlyUSD: 2.00},
				{Resource: "BigQuery Queries", Description: "On-demand queries (first 1TB/mo free)", MonthlyUSD: 0},
				{Resource: "Data Transfer", Description: "Scheduled data transfer job", MonthlyUSD: 0},
			},
			MonthlyUSD: 2.00,
		}

	case models.FeatureDeveloperPortal:
		return FeatureCost{
			FeatureID:   featureID,
			FeatureName: "Dynamic Developer Portal",
			Items: []LineItem{
				{Resource: "Config Controller", Description: "GKE-based Config Controller cluster", MonthlyUSD: 73.00},
				{Resource: "Compute (Backstage)", Description: "Backstage workloads on Config Controller", MonthlyUSD: 50.00},
				{Resource: "Persistent Disk", Description: "PostgreSQL PVC (10GB SSD)", MonthlyUSD: 1.70},
			},
			MonthlyUSD: 124.70,
		}

	case models.FeatureHardenedImageBakery:
		return FeatureCost{
			FeatureID:   featureID,
			FeatureName: "Hardened Image Bakery",
			Items: []LineItem{
				{Resource: "Cloud Build", Description: "Build minutes (first 120 min/day free)", MonthlyUSD: 0},
				{Resource: "Artifact Registry", Description: "Image storage (first 500MB free)", MonthlyUSD: 0.50},
				{Resource: "Compute Engine", Description: "Temporary build VMs (per-build)", MonthlyUSD: 5.00},
			},
			MonthlyUSD: 5.50,
		}

	case models.FeatureSecureInferencing:
		return FeatureCost{
			FeatureID:   featureID,
			FeatureName: "Secure Inferencing",
			Items: []LineItem{
				{Resource: "Cloud Run", Description: "LiteLLM proxy (1 vCPU, 512Mi, min 1 instance)", MonthlyUSD: 25.00},
				{Resource: "Secret Manager", Description: "Master key secret (6 versions)", MonthlyUSD: 0.06},
				{Resource: "Vertex AI", Description: "Gemini API calls (usage-based, varies)", MonthlyUSD: 0},
			},
			MonthlyUSD: 25.06,
		}

	case models.FeatureGovernance:
		return FeatureCost{
			FeatureID:   featureID,
			FeatureName: "Governance Guardrails",
			Items: []LineItem{
				{Resource: "GKE Policy Controller", Description: "Managed OPA Gatekeeper vCPU fee (config cluster)", MonthlyUSD: 8.00},
				{Resource: "Constraint Library", Description: "Regime guardrail policies (no direct cost)", MonthlyUSD: 0},
			},
			MonthlyUSD: 8.00,
		}

	case models.FeatureSkaffoldAppDev:
		return FeatureCost{
			FeatureID:   featureID,
			FeatureName: "Skaffold Application Development",
			Items: []LineItem{
				{Resource: "GKE Autopilot", Description: "Cluster management fee", MonthlyUSD: 73.00},
				{Resource: "Compute", Description: "Autopilot pod resources (e2-standard-4 equiv)", MonthlyUSD: 95.00},
				{Resource: "Cloud SQL", Description: "PostgreSQL db-f1-micro (if enabled)", MonthlyUSD: 10.00},
			},
			MonthlyUSD: 178.00,
		}

	default:
		return FeatureCost{
			FeatureID:   featureID,
			FeatureName: string(featureID),
			MonthlyUSD:  0,
		}
	}
}
