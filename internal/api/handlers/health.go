package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/aillion-co/infrastructure-lz/internal/config"
)

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// Healthz reports liveness with the build version and commit.
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:  "ok",
		Version: config.Version,
		Commit:  config.Commit,
	})
}

// Readyz reports readiness to serve traffic.
func Readyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:  "ready",
		Version: config.Version,
		Commit:  config.Commit,
	})
}
