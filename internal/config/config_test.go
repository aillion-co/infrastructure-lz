package config_test

import (
	"os"
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
	os.Setenv("PORT", "9090")
	os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("LOG_LEVEL")
	}()

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
	os.Setenv("LOG_LEVEL", "invalid")
	defer os.Unsetenv("LOG_LEVEL")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}
