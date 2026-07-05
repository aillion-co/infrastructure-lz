package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "valid bearer token", header: "Bearer abc123", want: "abc123"},
		{name: "missing header", header: "", want: ""},
		{name: "wrong scheme", header: "Basic abc123", want: ""},
		{name: "bearer lowercase not accepted", header: "bearer abc123", want: ""},
		{name: "bearer with empty token", header: "Bearer ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			assert.Equal(t, tt.want, extractBearerToken(r))
		})
	}
}

func TestEmailDomain(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{email: "user@example.com", want: "example.com"},
		{email: "user@sub.example.com", want: "sub.example.com"},
		{email: "no-at-sign", want: ""},
		{email: "", want: ""},
		{email: "a@b@c.com", want: "b@c.com"},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			assert.Equal(t, tt.want, emailDomain(tt.email))
		})
	}
}

func TestAuth_SkipPaths_BypassesAuth(t *testing.T) {
	mw := Auth(AuthConfig{Enabled: true, SkipPaths: []string{"/healthz"}})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_Disabled_PassesThrough(t *testing.T) {
	mw := Auth(AuthConfig{Enabled: false})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/features", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_Enabled_MissingToken_Unauthorized(t *testing.T) {
	mw := Auth(AuthConfig{Enabled: true})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached without a token")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/features", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
