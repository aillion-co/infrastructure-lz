//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func baseURL() string {
	if u := os.Getenv("E2E_BASE_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL() + "/healthz")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}

// TestActivateEndpoint smoke-tests the activation API with a minimal valid
// bootstrap-org payload.
func TestActivateEndpoint(t *testing.T) {
	req := map[string]any{
		"customer": map[string]any{
			"customerName": "e2e-smoke",
			"contactEmail": "e2e@example.com",
			"contactName":  "E2E",
		},
		"features": []map[string]any{
			{
				"featureId": "bootstrap-org",
				"enabled":   true,
				"config": map[string]any{
					"customerName":   "e2e",
					"workloadName":   "platform",
					"rootLevel":      "organization",
					"rootId":         "123456789012",
					"billingAccount": "AAAAAA-BBBBBB-CCCCCC",
					"region":         "us-central1",
					"envs":           "dev,prod",
				},
			},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp, err := http.Post(
		baseURL()+"/api/v1/activate",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("activate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", ct)
	}
}

// TestActivateEndpoint_ValidationError verifies field-level config validation
// surfaces as HTTP 400 via the kcc.ValidationError path.
func TestActivateEndpoint_ValidationError(t *testing.T) {
	req := map[string]any{
		"customer": map[string]any{
			"customerName": "e2e-bad",
			"contactEmail": "e2e@example.com",
		},
		"features": []map[string]any{
			{
				"featureId": "bootstrap-org",
				"enabled":   true,
				"config": map[string]any{
					"customerName":   "e2e",
					"workloadName":   "platform",
					"rootLevel":      "organization",
					"rootId":         "123456789012",
					"billingAccount": "AAAAAA-BBBBBB-CCCCCC",
					"region":         "us-central1",
					"envs":           "",
				},
			},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp, err := http.Post(
		baseURL()+"/api/v1/activate",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(respBody))
	}
}

// TestUIEndpoint verifies the wizard page is served at /activate.
func TestUIEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL() + "/activate")
	if err != nil {
		t.Fatalf("UI request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html, got %q", ct)
	}
}
