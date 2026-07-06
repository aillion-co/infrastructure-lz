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
	"path/filepath"
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

// noFollowClient returns an HTTP client that does not follow redirects, so
// tests can inspect 30x responses directly.
func noFollowClient() *http.Client {
	c := httpClient()
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return c
}

// TestHealthz verifies the health endpoint returns a healthy status.
func TestHealthz(t *testing.T) {
	resp, err := httpClient().Get(baseURL() + "/healthz")
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
	resp, err := httpClient().Get(baseURL() + "/readyz")
	if err != nil {
		t.Fatalf("readyz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestActivate_BootstrapOrgOnly generates an activation containing only the
// foundational bootstrap-org feature and validates the resulting umbrella
// Helm chart.
func TestActivate_BootstrapOrgOnly(t *testing.T) {
	req := newActivationRequest("integ-bootstrap", []map[string]any{
		bootstrapOrgFeature(true, validBootstrapOrgConfig()),
	})

	zipData := doActivate(t, req)
	assertValidUmbrellaChart(t, zipData, "integ-bootstrap", []string{"bootstrap-org"})
}

// TestActivate_AllFeatures enables all seven features with valid config and
// asserts the resulting zip contains a sub-chart per feature plus the expected
// KCC kinds.
func TestActivate_AllFeatures(t *testing.T) {
	req := newActivationRequest("integ-full", []map[string]any{
		bootstrapOrgFeature(true, validBootstrapOrgConfig()),
		{
			"featureId": "bigquery-analytics",
			"enabled":   true,
			"config": map[string]any{
				"projectName":     "integ-bq",
				"projectId":       "integ-bq-001",
				"region":          "us-central1",
				"datasetId":       "integ_events",
				"dataViewerGroup": "viewers@example.com",
			},
		},
		{
			"featureId": "dynamic-developer-portal",
			"enabled":   true,
			"config": map[string]any{
				"projectName":   "integ-portal",
				"gcpProjectId":  "integ-portal-001",
				"gcpRegion":     "us-central1",
				"gcpNetwork":    "shared-vpc",
				"gcpSubnet":     "portal-subnet",
				"gitRepoSshUrl": "git@github.com:integ/config.git",
			},
		},
		{
			"featureId": "hardened-image-bakery",
			"enabled":   true,
			"config": map[string]any{
				"projectName":    "integ-bakery",
				"projectId":      "integ-bakery-001",
				"region":         "us-central1",
				"zone":           "us-central1-a",
				"network":        "shared-vpc",
				"subnetwork":     "bakery-subnet",
				"buildSa":        "image-builder",
				"packerVersion":  "1.9.4",
				"ansibleVersion": "2.15.0",
			},
		},
		{
			"featureId": "governance-guardrails",
			"enabled":   true,
			"config": map[string]any{
				"projectId":       "integ-mgmt-001",
				"regimes":         []string{"cis", "gdpr", "pci-dss", "iso27001", "nis2", "eucra"},
				"enforcementMode": "deny",
			},
		},
		{
			"featureId": "secure-inferencing",
			"enabled":   true,
			"config": map[string]any{
				"projectName":        "integ-ai",
				"projectId":          "integ-ai-001",
				"region":             "us-central1",
				"enableGemini":       true,
				"geminiModel":        "gemini-2.0-flash",
				"enableAuditLogging": true,
			},
		},
		{
			"featureId": "skaffold-application-development",
			"enabled":   true,
			"config": map[string]any{
				"projectName":    "integ-app",
				"serviceName":    "api",
				"region":         "us-central1",
				"clusterType":    "autopilot",
				"releaseChannel": "REGULAR",
				"sqlDb":          "no",
				"allowIngress":   "no",
			},
		},
	})

	zipData := doActivate(t, req)
	assertValidUmbrellaChart(t, zipData, "integ-full", []string{
		"bootstrap-org",
		"bigquery-analytics",
		"dynamic-developer-portal",
		"governance-guardrails",
		"hardened-image-bakery",
		"secure-inferencing",
		"skaffold-application-development",
	})

	// Sanity-check that signature KCC kinds end up in the templates of each
	// sub-chart.
	assertZipContains(t, zipData, "BigQueryDataset")
	assertZipContains(t, zipData, "ContainerCluster")
	assertZipContains(t, zipData, "RunService")
	assertZipContains(t, zipData, "ConstraintTemplate")
	assertZipContains(t, zipData, "GKEHubFeature")
	assertZipContains(t, zipData, "scaffolder.backstage.io/v1beta3")
	assertZipContains(t, zipData, "backstage-golden-patterns")
}

// TestActivate_FeatureValidationError exercises the ValidationError -> HTTP 400
// path. An empty envs string triggers bootstrap-org's required-field validator.
func TestActivate_FeatureValidationError(t *testing.T) {
	cfg := validBootstrapOrgConfig()
	cfg["envs"] = ""

	req := newActivationRequest("integ-bad-envs", []map[string]any{
		bootstrapOrgFeature(true, cfg),
	})

	resp := postActivate(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(body))
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body["feature"] != "bootstrap-org" {
		t.Errorf("expected feature bootstrap-org, got %v", body["feature"])
	}
	if body["field"] != "envs" {
		t.Errorf("expected field envs, got %v", body["field"])
	}
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' key in response")
	}
}

// TestActivate_RequestValidationError exercises the request-level validator
// (no enabled features) which returns 422.
func TestActivate_RequestValidationError(t *testing.T) {
	req := newActivationRequest("integ-empty", []map[string]any{
		bootstrapOrgFeature(false, validBootstrapOrgConfig()),
	})

	resp := postActivate(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422, got %d: %s", resp.StatusCode, string(body))
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if _, ok := body["errors"]; !ok {
		t.Error("expected 'errors' key in 422 response")
	}
}

// TestActivate_BadJSON verifies that malformed JSON returns 400.
func TestActivate_BadJSON(t *testing.T) {
	resp, err := httpClient().Post(
		baseURL()+"/api/v1/activate",
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

// TestActivate_ConcurrentRequests validates the server handles concurrent
// activation requests without races or zip corruption.
func TestActivate_ConcurrentRequests(t *testing.T) {
	const concurrency = 5
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := newActivationRequest(
				fmt.Sprintf("integ-conc-%d", idx),
				[]map[string]any{bootstrapOrgFeature(true, validBootstrapOrgConfig())},
			)
			body, err := json.Marshal(req)
			if err != nil {
				errCh <- fmt.Errorf("request %d marshal: %w", idx, err)
				return
			}
			resp, err := httpClient().Post(
				baseURL()+"/api/v1/activate",
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				errCh <- fmt.Errorf("request %d post: %w", idx, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				errCh <- fmt.Errorf("request %d: expected 200, got %d: %s", idx, resp.StatusCode, string(b))
				return
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				errCh <- fmt.Errorf("request %d read: %w", idx, err)
				return
			}
			if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
				errCh <- fmt.Errorf("request %d invalid zip: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

// TestActivate_ResponseHeaders verifies the Content-Type and
// Content-Disposition headers on the zip response.
func TestActivate_ResponseHeaders(t *testing.T) {
	req := newActivationRequest("integ-headers", []map[string]any{
		bootstrapOrgFeature(true, validBootstrapOrgConfig()),
	})

	resp := postActivate(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "integ-headers-activation.zip") {
		t.Errorf("expected Content-Disposition with integ-headers-activation.zip, got %q", cd)
	}
}

// TestWebUI_Activate verifies the wizard page is served at /activate.
func TestWebUI_Activate(t *testing.T) {
	resp, err := httpClient().Get(baseURL() + "/activate")
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
	body, _ := io.ReadAll(resp.Body)
	lower := strings.ToLower(string(body))
	if !strings.Contains(lower, "activation") && !strings.Contains(lower, "<form") {
		t.Error("activate page does not contain expected HTML content")
	}
}

// TestWebUI_RootRedirects verifies GET / redirects to /activate.
func TestWebUI_RootRedirects(t *testing.T) {
	resp, err := noFollowClient().Get(baseURL() + "/")
	if err != nil {
		t.Fatalf("root request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/activate" {
		t.Errorf("expected Location: /activate, got %q", loc)
	}
}

// TestFeatures_API confirms the feature registry endpoint returns the six
// known features.
func TestFeatures_API(t *testing.T) {
	resp, err := httpClient().Get(baseURL() + "/api/v1/features")
	if err != nil {
		t.Fatalf("features request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var features []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&features); err != nil {
		t.Fatalf("failed to decode features: %v", err)
	}

	expected := map[string]bool{
		"bootstrap-org":                    false,
		"bigquery-analytics":               false,
		"dynamic-developer-portal":         false,
		"governance-guardrails":            false,
		"hardened-image-bakery":            false,
		"secure-inferencing":               false,
		"skaffold-application-development": false,
	}
	for _, f := range features {
		id, _ := f["id"].(string)
		if _, ok := expected[id]; ok {
			expected[id] = true
		}
	}
	for id, found := range expected {
		if !found {
			t.Errorf("missing feature %q in registry response", id)
		}
	}
}

// --- helpers ---

func newActivationRequest(customer string, features []map[string]any) map[string]any {
	return map[string]any{
		"customer": map[string]any{
			"customerName": customer,
			"contactEmail": "integ@example.com",
			"contactName":  "Integration Test",
		},
		"features": features,
	}
}

func validBootstrapOrgConfig() map[string]any {
	return map[string]any{
		"customerName":    "integtest",
		"workloadName":    "platform",
		"rootLevel":       "organization",
		"rootId":          "123456789012",
		"billingAccount":  "AAAAAA-BBBBBB-CCCCCC",
		"region":          "us-central1",
		"zone":            "us-central1-a",
		"orgPolicies":     true,
		"sharedVpc":       true,
		"serviceProjects": true,
		"vcs":             "github",
		"vcsOrg":          "integ-corp",
		"pipeline":        "cloudbuild",
		"envs":            "dev,prod",
		"envFolders":      true,
		"gkeCluster":      true,
	}
}

func bootstrapOrgFeature(enabled bool, cfg map[string]any) map[string]any {
	return map[string]any{
		"featureId": "bootstrap-org",
		"enabled":   enabled,
		"config":    cfg,
	}
}

func postActivate(t *testing.T, req map[string]any) *http.Response {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := httpClient().Post(
		baseURL()+"/api/v1/activate",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("activate request failed: %v", err)
	}
	return resp
}

func doActivate(t *testing.T, req map[string]any) []byte {
	t.Helper()
	resp := postActivate(t, req)
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

// assertValidUmbrellaChart inspects the activation zip and verifies it
// contains a valid Helm umbrella chart structure with the expected sub-charts.
//
// Expected layout (per internal/generator/helm/activation_builder.go):
//
//	<customer>-activation/Chart.yaml
//	<customer>-activation/values.yaml
//	<customer>-activation/charts/<feature-id>/Chart.yaml
//	<customer>-activation/charts/<feature-id>/values.yaml
//	<customer>-activation/charts/<feature-id>/templates/_helpers.tpl
//	<customer>-activation/charts/<feature-id>/templates/<resource>.yaml
func assertValidUmbrellaChart(t *testing.T, zipData []byte, customer string, expectedFeatures []string) {
	t.Helper()

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}

	chartRoot := customer + "-activation"
	requiredTop := map[string]bool{
		chartRoot + "/Chart.yaml":  false,
		chartRoot + "/values.yaml": false,
	}
	subCharts := make(map[string]bool, len(expectedFeatures))
	for _, fid := range expectedFeatures {
		subCharts[fid] = false
	}
	subChartTemplates := make(map[string]bool, len(expectedFeatures))

	for _, f := range r.File {
		if _, ok := requiredTop[f.Name]; ok {
			requiredTop[f.Name] = true
			continue
		}
		for fid := range subCharts {
			prefix := chartRoot + "/charts/" + fid + "/"
			if !strings.HasPrefix(f.Name, prefix) {
				continue
			}
			rest := strings.TrimPrefix(f.Name, prefix)
			if rest == "Chart.yaml" {
				subCharts[fid] = true
			}
			if strings.HasPrefix(rest, "templates/") && strings.HasSuffix(rest, ".yaml") {
				subChartTemplates[fid] = true
			}
		}
	}

	for path, found := range requiredTop {
		if !found {
			t.Errorf("required umbrella file %q not found in zip", path)
		}
	}
	for fid, found := range subCharts {
		if !found {
			t.Errorf("expected sub-chart %q Chart.yaml not found", fid)
		}
		if !subChartTemplates[fid] {
			t.Errorf("expected sub-chart %q to contain at least one templates/*.yaml", fid)
		}
	}
}

// assertZipContains scans every file in the zip and fails if no file's
// content contains the given substring.
func assertZipContains(t *testing.T, zipData []byte, substring string) {
	t.Helper()

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		if strings.Contains(string(data), substring) {
			return
		}
	}

	t.Errorf("zip does not contain any file with content matching %q", substring)
}

// TestActivate_Fixtures posts each example request in test/fixtures to keep
// the documented fixture files honest against the live API.
func TestActivate_Fixtures(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("..", "fixtures", "activation-*.json"))
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no activation-*.json fixtures found in test/fixtures")
	}

	for _, fx := range fixtures {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			raw, err := os.ReadFile(fx)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			var req map[string]any
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatalf("fixture is not valid JSON: %v", err)
			}

			zipData := doActivate(t, req)
			if _, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData))); err != nil {
				t.Fatalf("fixture activation did not produce a valid zip: %v", err)
			}
		})
	}
}
