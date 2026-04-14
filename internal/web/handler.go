package web

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

type Handler struct {
	templates *template.Template
}

func NewHandler() *Handler {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	return &Handler{templates: tmpl}
}

// ActivateHandler serves the activation wizard UI.
func (h *Handler) ActivateHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.templates.ExecuteTemplate(w, "activate.html", nil); err != nil {
		slog.Error("failed to render activate template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
