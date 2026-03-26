package api

import (
	"net/http"

	"github.com/aillion-co/infrastructure-lz/internal/api/handlers"
	"github.com/aillion-co/infrastructure-lz/internal/api/middleware"
	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/web"
)

func NewRouter(gen *generator.Generator) http.Handler {
	mux := http.NewServeMux()

	// Health checks
	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.HandleFunc("GET /readyz", handlers.Readyz)

	// API
	generateHandler := handlers.NewGenerateHandler(gen)
	mux.Handle("POST /api/v1/generate", generateHandler)

	// Web UI
	webHandler := web.NewHandler()
	mux.Handle("GET /", webHandler)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("internal/web/static"))))

	// Apply middleware (order: outermost runs first)
	var handler http.Handler = mux
	handler = middleware.Logging(handler)
	handler = middleware.Telemetry(handler)
	handler = middleware.Recovery(handler)

	return handler
}
