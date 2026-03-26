//go:build e2e

package e2e

import (
	"encoding/json"
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

func TestGenerateEndpoint(t *testing.T) {
	payload := `{
		"projectName": "e2e-test",
		"projectID": "e2e-test-project-123",
		"region": "us-central1",
		"environment": "development",
		"network": {
			"vpcName": "e2e-vpc",
			"subnetCIDR": "10.0.0.0/20",
			"enableNAT": true,
			"enablePrivate": true
		},
		"iam": {
			"adminGroup": "admin@example.com"
		}
	}`

	resp, err := http.Post(
		baseURL()+"/api/v1/generate",
		"application/json",
		strings.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("generate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", ct)
	}
}

func TestGenerateEndpoint_ValidationError(t *testing.T) {
	payload := `{"projectName": "", "projectID": "BAD", "region": "invalid", "environment": "nope", "network": {"vpcName": "", "subnetCIDR": "bad"}}`

	resp, err := http.Post(
		baseURL()+"/api/v1/generate",
		"application/json",
		strings.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestUIEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL() + "/")
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
