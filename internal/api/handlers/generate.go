package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/aillion-co/infrastructure-lz/pkg/validator"
)

type GenerateHandler struct {
	gen *generator.Generator
}

func NewGenerateHandler(gen *generator.Generator) *GenerateHandler {
	return &GenerateHandler{gen: gen}
}

func (h *GenerateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg models.ProjectConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		slog.Error("failed to decode request", "error", err)
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}

	if errs := validator.ValidateProjectConfig(&cfg); len(errs) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{"errors": errs})
		return
	}

	zipData, err := h.gen.Generate(r.Context(), &cfg)
	if err != nil {
		slog.Error("failed to generate IAC", "error", err, "project", cfg.ProjectName)
		http.Error(w, "generation failed", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("%s-iac.zip", cfg.ProjectName)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(zipData)
}
