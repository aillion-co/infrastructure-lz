package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/aillion-co/infrastructure-lz/internal/pricing"
)

func TestCostEstimateHandler_AllFeatures(t *testing.T) {
	req := models.ActivationRequest{
		Customer: models.CustomerDetails{
			CustomerName: "test-cost",
			ContactEmail: "test@example.com",
		},
		Features: []models.FeatureSelection{
			{FeatureID: models.FeatureBootstrapOrg, Enabled: true},
			{FeatureID: models.FeatureBigQueryAnalytics, Enabled: true},
			{FeatureID: models.FeatureDeveloperPortal, Enabled: true},
			{FeatureID: models.FeatureHardenedImageBakery, Enabled: true},
			{FeatureID: models.FeatureSecureInferencing, Enabled: true},
			{FeatureID: models.FeatureSkaffoldAppDev, Enabled: true},
			{FeatureID: models.FeatureGovernance, Enabled: true},
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-estimate", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	NewCostHandler(pricing.NewStaticProvider()).ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CostEstimateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "USD", resp.Currency)
	assert.Len(t, resp.Features, 7)
	assert.Greater(t, resp.TotalMonthly, 0.0)
	assert.NotEmpty(t, resp.Disclaimer)

	// Verify each feature has items
	for _, f := range resp.Features {
		assert.NotEmpty(t, f.FeatureName, "feature %s missing name", f.FeatureID)
		assert.Greater(t, len(f.Items), 0, "feature %s has no line items", f.FeatureID)
	}
}

func TestCostEstimateHandler_DisabledFeaturesExcluded(t *testing.T) {
	req := models.ActivationRequest{
		Customer: models.CustomerDetails{
			CustomerName: "test-cost",
			ContactEmail: "test@example.com",
		},
		Features: []models.FeatureSelection{
			{FeatureID: models.FeatureBootstrapOrg, Enabled: true},
			{FeatureID: models.FeatureBigQueryAnalytics, Enabled: false},
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-estimate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	NewCostHandler(pricing.NewStaticProvider()).ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CostEstimateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Len(t, resp.Features, 1)
	assert.Equal(t, models.FeatureBootstrapOrg, resp.Features[0].FeatureID)
}

func TestCostEstimateHandler_InvalidJSON(t *testing.T) {
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-estimate", bytes.NewReader([]byte("{")))
	w := httptest.NewRecorder()

	NewCostHandler(pricing.NewStaticProvider()).ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCostEstimateHandler_EmptyFeatures(t *testing.T) {
	req := models.ActivationRequest{
		Customer: models.CustomerDetails{CustomerName: "test"},
	}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-estimate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	NewCostHandler(pricing.NewStaticProvider()).ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CostEstimateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0.0, resp.TotalMonthly)
}
