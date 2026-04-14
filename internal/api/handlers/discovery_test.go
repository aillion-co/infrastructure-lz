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
)

func TestHandleDiscoveryEvaluate_BootstrapOrgAlwaysRecommended(t *testing.T) {
	body, _ := json.Marshal(models.DiscoveryResponse{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/evaluate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HandleDiscoveryEvaluate().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp DiscoveryEvaluationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Len(t, resp.Recommendations, 6)
	// bootstrap-org should always be recommended
	for _, r := range resp.Recommendations {
		if r.FeatureID == models.FeatureBootstrapOrg {
			assert.True(t, r.Recommended)
			assert.Equal(t, "high", r.Confidence)
			return
		}
	}
	t.Fatal("bootstrap-org recommendation not found")
}

func TestHandleDiscoveryEvaluate_AIEnablementRecommendsInferencing(t *testing.T) {
	discovery := models.DiscoveryResponse{
		AIEnablement: models.DiscoveryAIEnablement{
			EnableVertexAI: true,
			GeminiModels:   []string{"gemini-2.0-flash"},
			EnableLiteLLM:  true,
		},
	}
	body, _ := json.Marshal(discovery)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/evaluate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HandleDiscoveryEvaluate().ServeHTTP(w, req)

	var resp DiscoveryEvaluationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	for _, r := range resp.Recommendations {
		if r.FeatureID == models.FeatureSecureInferencing {
			assert.True(t, r.Recommended)
			assert.Equal(t, "high", r.Confidence)
			return
		}
	}
	t.Fatal("secure-inferencing recommendation not found")
}

func TestHandleDiscoveryEvaluate_VMSecurityRecommendsImageBakery(t *testing.T) {
	discovery := models.DiscoveryResponse{
		Security: models.DiscoverySecurity{
			VMSecurity: []string{"hardened-images", "shielded-vms"},
		},
	}
	body, _ := json.Marshal(discovery)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/evaluate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HandleDiscoveryEvaluate().ServeHTTP(w, req)

	var resp DiscoveryEvaluationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	for _, r := range resp.Recommendations {
		if r.FeatureID == models.FeatureHardenedImageBakery {
			assert.True(t, r.Recommended)
			assert.Equal(t, "high", r.Confidence)
			return
		}
	}
	t.Fatal("hardened-image-bakery recommendation not found")
}

func TestHandleDiscoveryEvaluate_BQMLRecommendsBigQuery(t *testing.T) {
	discovery := models.DiscoveryResponse{
		AIEnablement: models.DiscoveryAIEnablement{
			EnableBQML: true,
		},
	}
	body, _ := json.Marshal(discovery)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/evaluate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HandleDiscoveryEvaluate().ServeHTTP(w, req)

	var resp DiscoveryEvaluationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	for _, r := range resp.Recommendations {
		if r.FeatureID == models.FeatureBigQueryAnalytics {
			assert.True(t, r.Recommended)
			return
		}
	}
	t.Fatal("bigquery-analytics recommendation not found")
}

func TestHandleDiscoveryEvaluate_SkaffoldPipelineRecommendsAppDev(t *testing.T) {
	discovery := models.DiscoveryResponse{
		IACCICD: models.DiscoveryIACCICD{
			BuildPipeline: "skaffold",
		},
	}
	body, _ := json.Marshal(discovery)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/evaluate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HandleDiscoveryEvaluate().ServeHTTP(w, req)

	var resp DiscoveryEvaluationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	for _, r := range resp.Recommendations {
		if r.FeatureID == models.FeatureSkaffoldAppDev {
			assert.True(t, r.Recommended)
			return
		}
	}
	t.Fatal("skaffold-app-dev recommendation not found")
}

func TestHandleDiscoveryEvaluate_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/evaluate", bytes.NewReader([]byte("{")))
	w := httptest.NewRecorder()

	HandleDiscoveryEvaluate().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleDiscoverySections_ReturnsAllSections(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/discovery/sections", nil)
	w := httptest.NewRecorder()

	HandleDiscoverySections().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var sections []models.DiscoverySectionMeta
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sections))

	assert.GreaterOrEqual(t, len(sections), 10)
	for _, s := range sections {
		assert.NotEmpty(t, s.Key, "section %q missing Key", s.Title)
		assert.NotEmpty(t, s.Title)
		assert.NotEmpty(t, s.Fields)
	}
}
