package config_test

import (
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level info, got %s", cfg.LogLevel)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.LogLevel)
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "invalid")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestLoad_AuthRequiresAudiences(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "true")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when AUTH_ENABLED without AUTH_ALLOWED_AUDIENCES")
	}
}

func TestLoad_AuthEnabledWithAudiences(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_ALLOWED_AUDIENCES", "client-a.apps.googleusercontent.com, client-b")
	t.Setenv("AUTH_ALLOWED_DOMAINS", "example.com")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AuthEnabled {
		t.Error("expected AuthEnabled true")
	}
	if len(cfg.AuthAllowedAudiences) != 2 {
		t.Errorf("expected 2 audiences, got %d", len(cfg.AuthAllowedAudiences))
	}
	if len(cfg.AuthAllowedDomains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(cfg.AuthAllowedDomains))
	}
}
