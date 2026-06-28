package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
)

// gatewayRepository defines the data access methods required by gateway handlers.
type gatewayRepository interface {
	Ping() error
	ListGateways(ctx context.Context, limit int) ([]domain.Gateway, error)
	Dashboard(ctx context.Context) (map[string]any, error)
}

// repository aggregates all repository capabilities needed by the HTTP server.
// The concrete type (*mysql.Repository) satisfies all four embedded interfaces.
type repository interface {
	gatewayRepository
	adminRepository
	miniRepository
}

// Server is the HTTP API server, holding dependencies for all handlers.
type Server struct {
	repo   repository
	jwtCfg config.JWTConfig
}

// NewServer constructs a Server with the given repository and JWT configuration.
func NewServer(repo repository, jwtCfg config.JWTConfig) *Server {
	return &Server{repo: repo, jwtCfg: jwtCfg}
}

// Routes builds the http.Handler with all routes and middleware applied.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Health probes — no auth required.
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)

	secret := []byte(s.jwtCfg.Secret)
	adminRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin)

	ah := &adminHandler{repo: s.repo, jwtCfg: s.jwtCfg}
	mh := &miniHandler{repo: s.repo, jwtCfg: s.jwtCfg}

	// Admin auth — public.
	mux.HandleFunc("/api/v1/admin/login", ah.login)

	// Admin protected.
	mux.Handle("/api/v1/admin/me",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.me))))
	mux.Handle("/api/v1/admin/dashboard",
		authMiddleware(secret, adminRoles(http.HandlerFunc(s.dashboard))))

	// Mini-program auth — public (dev/test only).
	mux.HandleFunc("/api/v1/mini/mock-login", mh.mockLogin)

	// Gateway list — protected.
	mux.Handle("/api/v1/gateways",
		authMiddleware(secret, adminRoles(http.HandlerFunc(s.listGateways))))

	return corsMiddleware(loggingMiddleware(mux))
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.Ping(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unavailable",
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
	})
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	data, err := s.repo.Dashboard(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) listGateways(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code":    400,
				"message": "invalid limit",
			})
			return
		}
		limit = parsed
	}

	gateways, err := s.repo.ListGateways(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"items": gateways},
	})
}

// writeError writes a unified error envelope using the error's message string.
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"code":    status,
		"message": err.Error(),
	})
}

// writeJSON serialises value as JSON and writes it with the given HTTP status.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
