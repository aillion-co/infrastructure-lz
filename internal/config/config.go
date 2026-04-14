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
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:            envOrDefault("PORT", "8080"),
		LogLevel:        envOrDefault("LOG_LEVEL", "info"),
		OTLPEndpoint:    envOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		PricingProvider: envOrDefault("PRICING_PROVIDER", "static"),
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
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
