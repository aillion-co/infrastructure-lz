package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/api/handlers"
	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/models"
)

func validProjectConfig() models.ProjectConfig {
	return models.ProjectConfig{
		ProjectName: "test-project",
		ProjectID:   "test-project-123",
		Region:      "us-central1",
		Environment: "development",
		Network: models.NetworkConfig{
			VPCName:       "test-vpc",
			SubnetCIDR:    "10.0.0.0/20",
			EnableNAT:     true,
			EnablePrivate: true,
		},
		IAM: models.IAMConfig{
			AdminGroup:  "admins@example.com",
			ViewerGroup: "viewers@example.com",
		},
	}
}

func TestGenerateHandler_Success(t *testing.T) {
	gen := generator.New()
	h := handlers.NewGenerateHandler(gen)

	cfg := validProjectConfig()
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", ct)
	}

	if rec.Body.Len() == 0 {
		t.Error("expected non-empty zip response")
	}
}

func TestGenerateHandler_InvalidJSON(t *testing.T) {
	gen := generator.New()
	h := handlers.NewGenerateHandler(gen)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/generate", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestGenerateHandler_ValidationErrors(t *testing.T) {
	gen := generator.New()
	h := handlers.NewGenerateHandler(gen)

	cfg := models.ProjectConfig{
		ProjectName: "",
		ProjectID:   "INVALID",
		Region:      "invalid-region",
		Environment: "invalid",
		Network: models.NetworkConfig{
			VPCName:    "INVALID NAME",
			SubnetCIDR: "not-a-cidr",
		},
	}
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGenerateHandler_MethodNotAllowed(t *testing.T) {
	gen := generator.New()
	h := handlers.NewGenerateHandler(gen)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generate", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
}
