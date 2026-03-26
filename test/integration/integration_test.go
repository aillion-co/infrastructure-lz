//go:build integration

package integration

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func baseURL() string {
	if u := os.Getenv("INTEGRATION_BASE_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// TestHealthz verifies the health endpoint returns a healthy status.
func TestHealthz(t *testing.T) {
	client := httpClient()
	resp, err := client.Get(baseURL() + "/healthz")
	if err != nil {
		t.Fatalf("healthz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}

// TestReadyz verifies the readiness endpoint.
func TestReadyz(t *testing.T) {
	client := httpClient()
	resp, err := client.Get(baseURL() + "/readyz")
	if err != nil {
		t.Fatalf("readyz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestGenerate_BasicProject generates a basic project and validates the zip contents.
func TestGenerate_BasicProject(t *testing.T) {
	payload := `{
		"projectName": "integ-basic",
		"projectID": "integ-basic-project-001",
		"region": "us-central1",
		"environment": "development",
		"network": {
			"vpcName": "integ-vpc",
			"subnetCIDR": "10.0.0.0/20",
			"enableNAT": true,
			"enablePrivate": true
		},
		"iam": {
			"adminGroup": "admin@example.com"
		}
	}`

	zipData := doGenerate(t, payload)
	assertValidHelmChart(t, zipData, "integ-basic")
}

// TestGenerate_GKEProject generates a project with GKE config and validates KCC resources.
func TestGenerate_GKEProject(t *testing.T) {
	payload := `{
		"projectName": "integ-gke",
		"projectID": "integ-gke-project-001",
		"region": "us-central1",
		"environment": "staging",
		"network": {
			"vpcName": "integ-gke-vpc",
			"subnetCIDR": "10.1.0.0/20",
			"podCIDR": "10.64.0.0/14",
			"serviceCIDR": "10.68.0.0/20",
			"enableNAT": true,
			"enablePrivate": true
		},
		"gke": {
			"clusterName": "integ-cluster",
			"nodeCount": 3,
			"machineType": "e2-standard-4",
			"enableAutoscaling": true,
			"minNodes": 1,
			"maxNodes": 5
		},
		"iam": {
			"adminGroup": "admin@example.com",
			"viewerGroup": "viewers@example.com"
		}
	}`

	zipData := doGenerate(t, payload)
	assertValidHelmChart(t, zipData, "integ-gke")
	assertZipContains(t, zipData, "containercluster")
}

// TestGenerate_CloudSQLProject generates a project with CloudSQL and validates output.
func TestGenerate_CloudSQLProject(t *testing.T) {
	payload := `{
		"projectName": "integ-sql",
		"projectID": "integ-sql-project-001",
		"region": "us-central1",
		"environment": "production",
		"network": {
			"vpcName": "integ-sql-vpc",
			"subnetCIDR": "10.2.0.0/20",
			"enableNAT": false,
			"enablePrivate": false
		},
		"cloudSQL": {
			"instanceName": "integ-db",
			"databaseVersion": "POSTGRES_15",
			"tier": "db-custom-2-8192",
			"enableHA": true
		},
		"iam": {
			"adminGroup": "admin@example.com"
		}
	}`

	zipData := doGenerate(t, payload)
	assertValidHelmChart(t, zipData, "integ-sql")
	assertZipContains(t, zipData, "sqlinstance")
}

// TestGenerate_FullProject generates a project with all options enabled.
func TestGenerate_FullProject(t *testing.T) {
	payload := `{
		"projectName": "integ-full",
		"projectID": "integ-full-project-001",
		"region": "europe-west1",
		"environment": "production",
		"network": {
			"vpcName": "integ-full-vpc",
			"subnetCIDR": "10.3.0.0/20",
			"podCIDR": "10.72.0.0/14",
			"serviceCIDR": "10.76.0.0/20",
			"enableNAT": true,
			"enablePrivate": true
		},
		"gke": {
			"clusterName": "integ-full-cluster",
			"nodeCount": 3,
			"machineType": "e2-standard-8",
			"enableAutoscaling": true,
			"minNodes": 3,
			"maxNodes": 10
		},
		"cloudSQL": {
			"instanceName": "integ-full-db",
			"databaseVersion": "POSTGRES_15",
			"tier": "db-custom-4-16384",
			"enableHA": true
		},
		"iam": {
			"adminGroup": "admin@example.com",
			"viewerGroup": "viewers@example.com"
		}
	}`

	zipData := doGenerate(t, payload)
	assertValidHelmChart(t, zipData, "integ-full")
	assertZipContains(t, zipData, "containercluster")
	assertZipContains(t, zipData, "sqlinstance")
}

// TestGenerate_ValidationErrors verifies that invalid input returns 422 with error details.
func TestGenerate_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "empty project ID",
			payload: `{"projectName":"test","projectID":"","region":"us-central1","environment":"development","network":{"vpcName":"v","subnetCIDR":"10.0.0.0/20"}}`,
		},
		{
			name:    "invalid region",
			payload: `{"projectName":"test","projectID":"valid-project-123","region":"invalid","environment":"development","network":{"vpcName":"v","subnetCIDR":"10.0.0.0/20"}}`,
		},
		{
			name:    "invalid environment",
			payload: `{"projectName":"test","projectID":"valid-project-123","region":"us-central1","environment":"nope","network":{"vpcName":"v","subnetCIDR":"10.0.0.0/20"}}`,
		},
		{
			name:    "invalid CIDR",
			payload: `{"projectName":"test","projectID":"valid-project-123","region":"us-central1","environment":"development","network":{"vpcName":"v","subnetCIDR":"bad"}}`,
		},
	}

	client := httpClient()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(
				baseURL()+"/api/v1/generate",
				"application/json",
				strings.NewReader(tc.payload),
			)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnprocessableEntity {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected 422, got %d: %s", resp.StatusCode, string(body))
			}

			var errResp map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if _, ok := errResp["errors"]; !ok {
				t.Error("expected 'errors' key in response")
			}
		})
	}
}

// TestGenerate_BadJSON verifies that malformed JSON returns 400.
func TestGenerate_BadJSON(t *testing.T) {
	client := httpClient()
	resp, err := client.Post(
		baseURL()+"/api/v1/generate",
		"application/json",
		strings.NewReader(`{not valid json`),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestGenerate_ConcurrentRequests validates the server handles concurrent generation requests.
func TestGenerate_ConcurrentRequests(t *testing.T) {
	payload := `{
		"projectName": "integ-concurrent-%d",
		"projectID": "integ-concurrent-%d",
		"region": "us-central1",
		"environment": "development",
		"network": {
			"vpcName": "conc-vpc",
			"subnetCIDR": "10.0.0.0/20",
			"enableNAT": true,
			"enablePrivate": true
		},
		"iam": {
			"adminGroup": "admin@example.com"
		}
	}`

	const concurrency = 5
	var wg sync.WaitGroup
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p := fmt.Sprintf(payload, idx, idx)
			client := httpClient()
			resp, err := client.Post(
				baseURL()+"/api/v1/generate",
				"application/json",
				strings.NewReader(p),
			)
			if err != nil {
				errors <- fmt.Errorf("request %d failed: %w", idx, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				errors <- fmt.Errorf("request %d: expected 200, got %d: %s", idx, resp.StatusCode, string(body))
				return
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				errors <- fmt.Errorf("request %d: failed to read body: %w", idx, err)
				return
			}

			if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
				errors <- fmt.Errorf("request %d: invalid zip: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestGenerate_ResponseHeaders verifies correct headers on the zip response.
func TestGenerate_ResponseHeaders(t *testing.T) {
	payload := `{
		"projectName": "integ-headers",
		"projectID": "integ-headers-project-001",
		"region": "us-central1",
		"environment": "development",
		"network": {
			"vpcName": "headers-vpc",
			"subnetCIDR": "10.0.0.0/20",
			"enableNAT": true,
			"enablePrivate": true
		},
		"iam": {
			"adminGroup": "admin@example.com"
		}
	}`

	client := httpClient()
	resp, err := client.Post(
		baseURL()+"/api/v1/generate",
		"application/json",
		strings.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", ct)
	}

	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "integ-headers-iac.zip") {
		t.Errorf("expected Content-Disposition with integ-headers-iac.zip, got %q", cd)
	}
}

// TestWebUI verifies the web UI serves HTML.
func TestWebUI(t *testing.T) {
	client := httpClient()
	resp, err := client.Get(baseURL() + "/")
	if err != nil {
		t.Fatalf("UI request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "IAC Generator") && !strings.Contains(string(body), "iac-generator") && !strings.Contains(string(body), "<form") {
		t.Error("UI response does not contain expected HTML content")
	}
}

// --- helpers ---

func doGenerate(t *testing.T, payload string) []byte {
	t.Helper()
	client := httpClient()
	resp, err := client.Post(
		baseURL()+"/api/v1/generate",
		"application/json",
		strings.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("generate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("empty response body")
	}

	return data
}

func assertValidHelmChart(t *testing.T, zipData []byte, projectName string) {
	t.Helper()

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}

	requiredFiles := map[string]bool{
		"Chart.yaml":  false,
		"values.yaml": false,
	}

	hasTemplates := false
	for _, f := range r.File {
		for req := range requiredFiles {
			if strings.HasSuffix(f.Name, req) {
				requiredFiles[req] = true
			}
		}
		if strings.Contains(f.Name, "templates/") {
			hasTemplates = true
		}
	}

	for name, found := range requiredFiles {
		if !found {
			t.Errorf("required file %q not found in zip", name)
		}
	}

	if !hasTemplates {
		t.Error("no templates/ directory found in zip")
	}
}

func assertZipContains(t *testing.T, zipData []byte, substring string) {
	t.Helper()

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}

	for _, f := range r.File {
		if strings.Contains(strings.ToLower(f.Name), substring) {
			return
		}
		// Also check file contents
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		if strings.Contains(strings.ToLower(string(data)), substring) {
			return
		}
	}

	t.Errorf("zip does not contain any file or content matching %q", substring)
}
