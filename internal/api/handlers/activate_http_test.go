package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/generator"
)

func newActivateHandler() *ActivateHandler {
	return NewActivateHandler(generator.NewActivationGenerator())
}

func validBootstrapPayload() map[string]any {
	return map[string]any{
		"customer": map[string]any{
			"customerName": "acme",
			"contactEmail": "ops@acme.example",
		},
		"features": []map[string]any{
			{
				"featureId": "bootstrap-org",
				"enabled":   true,
				"config": map[string]any{
					"customerName":   "acme",
					"workloadName":   "web",
					"rootLevel":      "organization",
					"rootId":         "123456789012",
					"billingAccount": "AAAAAA-BBBBBB-CCCCCC",
					"region":         "us-central1",
					"envs":           "dev,prod",
				},
			},
		},
	}
}

func postJSON(t *testing.T, h http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case string:
		reader = strings.NewReader(b)
	default:
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/activate", reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestActivateHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/activate", nil)
	w := httptest.NewRecorder()
	newActivateHandler().ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestActivateHandler_MalformedJSON(t *testing.T) {
	w := postJSON(t, newActivateHandler(), `{not json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestActivateHandler_NoFeatures_422(t *testing.T) {
	payload := validBootstrapPayload()
	payload["features"] = []map[string]any{}
	w := postJSON(t, newActivateHandler(), payload)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["errors"]; !ok {
		t.Error("expected errors array in 422 body")
	}
}

func TestActivateHandler_FeatureValidationError_400(t *testing.T) {
	payload := validBootstrapPayload()
	feats := payload["features"].([]map[string]any)
	feats[0]["config"].(map[string]any)["envs"] = ""
	w := postJSON(t, newActivateHandler(), payload)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["feature"] != "bootstrap-org" || body["field"] != "envs" {
		t.Errorf("expected feature/field bootstrap-org/envs, got %v/%v", body["feature"], body["field"])
	}
}

func TestActivateHandler_InvalidCustomerName_422(t *testing.T) {
	payload := validBootstrapPayload()
	payload["customer"].(map[string]any)["customerName"] = "Acme Corp!"
	w := postJSON(t, newActivateHandler(), payload)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid customer name, got %d", w.Code)
	}
}

func TestActivateHandler_Success_ReturnsZip(t *testing.T) {
	w := postJSON(t, newActivateHandler(), validBootstrapPayload())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected application/zip, got %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "acme-activation.zip") {
		t.Errorf("expected filename in Content-Disposition, got %q", cd)
	}
	if _, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len())); err != nil {
		t.Errorf("response is not a valid zip: %v", err)
	}
}

func TestActivateHandler_OversizedBody_Rejected(t *testing.T) {
	// A body over the 1 MiB cap must fail rather than allocate unbounded.
	huge := strings.Repeat("a", maxRequestBodyBytes+1024)
	payload := validBootstrapPayload()
	payload["customer"].(map[string]any)["notes"] = huge
	w := postJSON(t, newActivateHandler(), payload)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized body, got %d", w.Code)
	}
}

func TestActivateHandler_DuplicateFeatureIDs(t *testing.T) {
	payload := validBootstrapPayload()
	feats := payload["features"].([]map[string]any)
	// A second bootstrap-org selection; current behavior collapses duplicates.
	payload["features"] = append(feats, map[string]any{
		"featureId": "bootstrap-org",
		"enabled":   true,
		"config":    feats[0]["config"],
	})
	w := postJSON(t, newActivateHandler(), payload)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for duplicate feature ids (last wins), got %d", w.Code)
	}
}
