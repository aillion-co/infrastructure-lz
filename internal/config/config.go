package config

import (
	"fmt"
	"os"
	"strings"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Config struct {
	Port            string
	LogLevel        string
	OTLPEndpoint    string
	PricingProvider string

	// AuthEnabled turns on Google ID token verification for API endpoints.
	AuthEnabled bool
	// AuthAllowedDomains restricts access to these email domains (comma-
	// separated env var). Empty means any authenticated user.
	AuthAllowedDomains []string
	// AuthAllowedAudiences is the set of OAuth client IDs (aud claim) the
	// server accepts. Required when AuthEnabled is true.
	AuthAllowedAudiences []string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                 envOrDefault("PORT", "8080"),
		LogLevel:             envOrDefault("LOG_LEVEL", "info"),
		OTLPEndpoint:         envOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		PricingProvider:      envOrDefault("PRICING_PROVIDER", "static"),
		AuthEnabled:          strings.EqualFold(envOrDefault("AUTH_ENABLED", "false"), "true"),
		AuthAllowedDomains:   splitList(os.Getenv("AUTH_ALLOWED_DOMAINS")),
		AuthAllowedAudiences: splitList(os.Getenv("AUTH_ALLOWED_AUDIENCES")),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}
	validProviders := map[string]bool{"static": true, "catalog": true}
	if !validProviders[strings.ToLower(c.PricingProvider)] {
		return fmt.Errorf("invalid pricing provider: %s (want static or catalog)", c.PricingProvider)
	}
	if c.AuthEnabled && len(c.AuthAllowedAudiences) == 0 {
		return fmt.Errorf("AUTH_ALLOWED_AUDIENCES is required when AUTH_ENABLED is true")
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitList parses a comma-separated env var into a trimmed, non-empty list.
func splitList(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
