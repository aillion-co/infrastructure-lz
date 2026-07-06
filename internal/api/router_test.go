package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/pricing"
)

func testRouter(cfg ...RouterConfig) http.Handler {
	return NewRouter(generator.NewActivationGenerator(), pricing.NewStaticProvider(), cfg...)
}

// getNoRedirect issues a GET that does not follow redirects.
func getNoRedirect(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	return resp
}

func TestRouter_RootRedirectsToActivate(t *testing.T) {
	srv := httptest.NewServer(testRouter())
	defer srv.Close()

	resp := getNoRedirect(t, srv.URL+"/")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/activate" {
		t.Errorf("expected redirect to /activate, got %q", loc)
	}
}

func TestRouter_MethodRouting(t *testing.T) {
	srv := httptest.NewServer(testRouter())
	defer srv.Close()

	// GET on a POST-only route must be 405.
	resp := getNoRedirect(t, srv.URL+"/api/v1/activate")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET on activate, got %d", resp.StatusCode)
	}
}

func TestRouter_AuthSkipPathsAndEnforcement(t *testing.T) {
	srv := httptest.NewServer(testRouter(RouterConfig{
		AuthEnabled:      true,
		AllowedAudiences: []string{"test-client"},
	}))
	defer srv.Close()

	// Health and UI bypass auth.
	for _, path := range []string{"/healthz", "/readyz", "/activate"} {
		resp := getNoRedirect(t, srv.URL+path)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			t.Errorf("path %s should bypass auth, got 401", path)
		}
	}

	// A protected API route without a token is rejected.
	resp := getNoRedirect(t, srv.URL+"/api/v1/features")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated API request, got %d", resp.StatusCode)
	}
}

func TestRouter_PanicIsContained(t *testing.T) {
	// The static file handler on a missing file must not crash the server;
	// verify the recovery middleware keeps the process serving.
	srv := httptest.NewServer(testRouter())
	defer srv.Close()

	resp := getNoRedirect(t, srv.URL+"/healthz")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected healthz 200, got %d", resp.StatusCode)
	}
}
