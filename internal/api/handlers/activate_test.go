package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

func TestValidateActivationRequest_Valid_NoErrors(t *testing.T) {
	req := &models.ActivationRequest{
		Customer: models.CustomerDetails{
			CustomerName: "acme",
			ContactEmail: "ops@acme.example",
		},
		Features: []models.FeatureSelection{
			{FeatureID: models.FeatureBootstrapOrg, Enabled: true},
		},
	}

	assert.Empty(t, validateActivationRequest(req))
}

func TestValidateActivationRequest_MissingCustomer_Errors(t *testing.T) {
	req := &models.ActivationRequest{
		Features: []models.FeatureSelection{
			{FeatureID: models.FeatureBootstrapOrg, Enabled: true},
		},
	}

	errs := validateActivationRequest(req)
	assert.Contains(t, errs, "customer name is required")
	assert.Contains(t, errs, "contact email is required")
}

func TestValidateActivationRequest_NoFeatures_Errors(t *testing.T) {
	req := &models.ActivationRequest{
		Customer: models.CustomerDetails{
			CustomerName: "acme",
			ContactEmail: "ops@acme.example",
		},
		Features: []models.FeatureSelection{
			{FeatureID: models.FeatureBootstrapOrg, Enabled: false},
		},
	}

	errs := validateActivationRequest(req)
	assert.Contains(t, errs, "at least one feature must be enabled")
}

func TestValidateActivationRequest_MissingDependency_Errors(t *testing.T) {
	req := &models.ActivationRequest{
		Customer: models.CustomerDetails{
			CustomerName: "acme",
			ContactEmail: "ops@acme.example",
		},
		Features: []models.FeatureSelection{
			{FeatureID: models.FeatureBigQueryAnalytics, Enabled: true},
		},
	}

	errs := validateActivationRequest(req)
	assert.Contains(t, errs,
		`feature "bigquery-analytics" requires feature "bootstrap-org" to be enabled`)
}

func TestValidateActivationRequest_DependencySatisfied_NoErrors(t *testing.T) {
	req := &models.ActivationRequest{
		Customer: models.CustomerDetails{
			CustomerName: "acme",
			ContactEmail: "ops@acme.example",
		},
		Features: []models.FeatureSelection{
			{FeatureID: models.FeatureBootstrapOrg, Enabled: true},
			{FeatureID: models.FeatureBigQueryAnalytics, Enabled: true},
		},
	}

	assert.Empty(t, validateActivationRequest(req))
}
