package api

import (
	"net/http"

	"github.com/aillion-co/infrastructure-lz/internal/api/handlers"
	"github.com/aillion-co/infrastructure-lz/internal/api/middleware"
	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/pricing"
	"github.com/aillion-co/infrastructure-lz/internal/web"
)

// RouterConfig holds configuration for the API router.
type RouterConfig struct {
	AuthEnabled    bool
	AllowedDomains []string
}

func NewRouter(activationGen *generator.ActivationGenerator, pricer pricing.Provider, cfg ...RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Health checks
	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.HandleFunc("GET /readyz", handlers.Readyz)

	// Activation API
	activateHandler := handlers.NewActivateHandler(activationGen)
	mux.Handle("POST /api/v1/activate", activateHandler)
	mux.HandleFunc("GET /api/v1/features", handlers.FeaturesHandler)
	mux.Handle("POST /api/v1/cost-estimate", handlers.NewCostHandler(pricer))

	// Discovery API
	mux.HandleFunc("POST /api/v1/discovery/evaluate", handlers.HandleDiscoveryEvaluate())
	mux.HandleFunc("GET /api/v1/discovery/sections", handlers.HandleDiscoverySections())

	// Web UI
	webHandler := web.NewHandler()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/activate", http.StatusFound)
	})
	mux.HandleFunc("GET /activate", webHandler.ActivateHandler)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("internal/web/static"))))

	// Build middleware chain (order: outermost runs first)
	var handler http.Handler = mux
	handler = middleware.Logging(handler)

	// Auth middleware (optional, configurable)
	var authCfg middleware.AuthConfig
	if len(cfg) > 0 {
		authCfg = middleware.AuthConfig{
			Enabled:        cfg[0].AuthEnabled,
			AllowedDomains: cfg[0].AllowedDomains,
			SkipPaths:      []string{"/healthz", "/readyz", "/", "/activate", "/static/"},
		}
	}
	handler = middleware.Auth(authCfg)(handler)

	handler = middleware.Telemetry(handler)
	handler = middleware.Recovery(handler)

	return handler
}
