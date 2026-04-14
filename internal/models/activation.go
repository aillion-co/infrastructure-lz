package models

// ActivationRequest is the top-level payload for generating infrastructure.
// It contains customer details, selected features with their configurations,
// and optional discovery responses.
type ActivationRequest struct {
	Customer  CustomerDetails    `json:"customer"`
	Features  []FeatureSelection `json:"features"`
	Discovery *DiscoveryResponse `json:"discovery,omitempty"`
}

// CustomerDetails identifies the customer requesting the activation.
type CustomerDetails struct {
	CustomerName string `json:"customerName"`
	ContactEmail string `json:"contactEmail"`
	ContactName  string `json:"contactName"`
	Industry     string `json:"industry,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// FeatureID identifies an activation feature type.
type FeatureID string

const (
	FeatureBootstrapOrg        FeatureID = "bootstrap-org"
	FeatureBigQueryAnalytics   FeatureID = "bigquery-analytics"
	FeatureDeveloperPortal     FeatureID = "dynamic-developer-portal"
	FeatureHardenedImageBakery FeatureID = "hardened-image-bakery"
	FeatureSecureInferencing   FeatureID = "secure-inferencing"
	FeatureSkaffoldAppDev      FeatureID = "skaffold-application-development"
)

// AllFeatureIDs returns all available feature identifiers.
func AllFeatureIDs() []FeatureID {
	return []FeatureID{
		FeatureBootstrapOrg,
		FeatureBigQueryAnalytics,
		FeatureDeveloperPortal,
		FeatureHardenedImageBakery,
		FeatureSecureInferencing,
		FeatureSkaffoldAppDev,
	}
}

// FeatureSelection represents a user's choice of a feature and its configuration.
type FeatureSelection struct {
	FeatureID FeatureID   `json:"featureId"`
	Enabled   bool        `json:"enabled"`
	Config    interface{} `json:"config,omitempty"`
}

// FeatureMetadata describes a feature for UI display and dependency resolution.
type FeatureMetadata struct {
	ID          FeatureID   `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Icon        string      `json:"icon"`
	Version     string      `json:"version"`
	Maturity    string      `json:"maturity"` // ga, beta, alpha, stub
	DependsOn   []FeatureID `json:"dependsOn,omitempty"`
}

// FeatureRegistry returns metadata for all available features.
func FeatureRegistry() []FeatureMetadata {
	return []FeatureMetadata{
		{
			ID:          FeatureBootstrapOrg,
			Name:        "Bootstrap Organisation",
			Description: "GCP Landing Zone: org structure, management project, CI/CD pipeline, folder hierarchy, org policies, and workload environments",
			Category:    "foundation",
			Icon:        "building",
			Version:     "2.0.0",
			Maturity:    "ga",
		},
		{
			ID:          FeatureBigQueryAnalytics,
			Name:        "BigQuery Analytics",
			Description: "BigQuery datasets for customer data visibility with CRM or Google Analytics integrations",
			Category:    "data",
			Icon:        "bar-chart-3",
			Version:     "0.1.0",
			Maturity:    "stub",
			DependsOn:   []FeatureID{FeatureBootstrapOrg},
		},
		{
			ID:          FeatureDeveloperPortal,
			Name:        "Dynamic Developer Portal",
			Description: "Backstage developer portal with Config Controller on GKE for GitOps-driven infrastructure management",
			Category:    "developer-experience",
			Icon:        "layout-dashboard",
			Version:     "1.0.0",
			Maturity:    "beta",
			DependsOn:   []FeatureID{FeatureBootstrapOrg},
		},
		{
			ID:          FeatureHardenedImageBakery,
			Name:        "Hardened Image Bakery",
			Description: "CIS-compliant hardened VM images (Ubuntu/Windows) using Packer and Ansible with Cloud Build orchestration",
			Category:    "security",
			Icon:        "shield-check",
			Version:     "1.0.0",
			Maturity:    "ga",
			DependsOn:   []FeatureID{FeatureBootstrapOrg},
		},
		{
			ID:          FeatureSecureInferencing,
			Name:        "Secure Inferencing",
			Description: "Secure AI proxy using LiteLLM for standardised LLM access with audit logging and cost tracking",
			Category:    "ai",
			Icon:        "brain",
			Version:     "0.1.0",
			Maturity:    "stub",
			DependsOn:   []FeatureID{FeatureBootstrapOrg},
		},
		{
			ID:          FeatureSkaffoldAppDev,
			Name:        "Skaffold Application Development",
			Description: "Kubernetes application scaffold with Skaffold, Flask, CIS-compliant security, and multi-environment Kustomize overlays",
			Category:    "application",
			Icon:        "container",
			Version:     "1.0.0",
			Maturity:    "ga",
			DependsOn:   []FeatureID{FeatureBootstrapOrg},
		},
	}
}
