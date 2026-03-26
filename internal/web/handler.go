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

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := h.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		slog.Error("failed to render template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// ActivateHandler serves the activation wizard UI.
func (h *Handler) ActivateHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.templates.ExecuteTemplate(w, "activate.html", nil); err != nil {
		slog.Error("failed to render activate template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
