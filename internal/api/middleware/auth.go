package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
)

type contextKey string

const userContextKey contextKey = "user"

// UserInfo holds authenticated user information extracted from the ID token.
type UserInfo struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Sub    string `json:"sub"`
	Issuer string `json:"iss"`
}

// UserFromContext retrieves the authenticated user from the request context.
func UserFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value(userContextKey).(*UserInfo)
	return user, ok
}

// AuthConfig configures the authentication middleware.
type AuthConfig struct {
	// AllowedDomains restricts authentication to specific email domains.
	// Empty means all authenticated users are allowed.
	AllowedDomains []string

	// SkipPaths are paths that bypass authentication (e.g., /healthz).
	SkipPaths []string

	// Enabled controls whether auth is enforced. When false, requests pass through.
	Enabled bool
}

// Auth returns middleware that validates Google Identity Platform tokens.
// It extracts the Bearer token from the Authorization header and validates it
// using Google's tokeninfo endpoint.
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	skipSet := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health checks and other excluded paths
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth if disabled
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			ctx, span := telemetry.Tracer().Start(r.Context(), "middleware.Auth")
			defer span.End()

			token := extractBearerToken(r)
			if token == "" {
				span.SetStatus(codes.Error, "missing token")
				http.Error(w, `{"error":"authorization required"}`, http.StatusUnauthorized)
				return
			}

			user, err := validateToken(ctx, token)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "invalid token")
				slog.WarnContext(ctx, "auth failed", "error", err)
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Check domain restriction
			if len(cfg.AllowedDomains) > 0 {
				domain := emailDomain(user.Email)
				allowed := false
				for _, d := range cfg.AllowedDomains {
					if strings.EqualFold(domain, d) {
						allowed = true
						break
					}
				}
				if !allowed {
					span.SetAttributes(attribute.String("auth.rejected_domain", domain))
					span.SetStatus(codes.Error, "domain not allowed")
					slog.WarnContext(ctx, "domain rejected", "email", user.Email, "domain", domain)
					http.Error(w, `{"error":"email domain not authorized"}`, http.StatusForbidden)
					return
				}
			}

			span.SetAttributes(
				attribute.String("auth.email", user.Email),
				attribute.String("auth.subject", user.Sub),
			)

			ctx = context.WithValue(ctx, userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// validateToken validates a Google ID token using the tokeninfo endpoint.
func validateToken(ctx context.Context, token string) (*UserInfo, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "middleware.Auth.validateToken")
	defer span.End()

	client := &http.Client{Timeout: 5 * time.Second}
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building token validation request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token validation request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token validation failed with status %d", resp.StatusCode)
	}

	var info struct {
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Sub           string `json:"sub"`
		Issuer        string `json:"iss"`
		Audience      string `json:"aud"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding token info: %w", err)
	}

	if info.EmailVerified != "true" {
		return nil, fmt.Errorf("email not verified: %s", info.Email)
	}

	return &UserInfo{
		Email:  info.Email,
		Name:   info.Name,
		Sub:    info.Sub,
		Issuer: info.Issuer,
	}, nil
}

func emailDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}
