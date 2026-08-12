package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/debiangee/lindol-api/internal/config"
	"github.com/debiangee/lindol-api/internal/services"
)

// Server holds dependencies for the HTTP server.
type Server struct {
	cfg       *config.Config
	logger    *slog.Logger
	mux       *http.ServeMux
	eqService *services.EarthquakeService
	health    *services.HealthTracker
}

// New creates a new Server with routes configured.
func New(cfg *config.Config, logger *slog.Logger, eqService *services.EarthquakeService, health *services.HealthTracker) *Server {
	s := &Server{
		cfg:       cfg,
		logger:    logger,
		mux:       http.NewServeMux(),
		eqService: eqService,
		health:    health,
	}
	s.routes()
	return s
}

// Router returns the HTTP handler with CORS and rate limiting middleware.
func (s *Server) Router() http.Handler {
	// Rate limit: 1 request per minute per IP
	rateLimiter := NewRateLimiter(60 * time.Second)

	// Apply middleware chain: CORS → Rate Limit → Routes
	return corsMiddleware(rateLimiter.Middleware(s.mux))
}

// corsMiddleware adds CORS headers for frontend consumers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("X-Lindol-API-Status", "testing")
		w.Header().Set("X-Lindol-API-Notice", "Testing deployment; not for production use.")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
