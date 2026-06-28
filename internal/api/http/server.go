package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/web"
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
	adapterRepository
}

// Server is the HTTP API server, holding dependencies for all handlers.
type Server struct {
	repo         repository
	jwtCfg       config.JWTConfig
	adapterToken string
}

// NewServer constructs a Server with the given repository and JWT configuration.
func NewServer(repo repository, cfg config.Config) *Server {
	return &Server{
		repo:         repo,
		jwtCfg:       cfg.JWT,
		adapterToken: cfg.AdapterToken,
	}
}

// Routes builds the http.Handler with all routes and middleware applied.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Health probes — no auth required (Go 1.20 compatible: check Method inside handler).
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
			return
		}
		s.healthz(w, r)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
			return
		}
		s.readyz(w, r)
	})

	secret := []byte(s.jwtCfg.Secret)
	adminRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin)
	farmerRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleFarmer)

	ah := &adminHandler{repo: s.repo, jwtCfg: s.jwtCfg}
	mh := &miniHandler{repo: s.repo, jwtCfg: s.jwtCfg}
	adh := &adapterHandler{repo: s.repo, adapterToken: s.adapterToken}

	// ─── Adapter API（X-Adapter-Token 认证，不走用户 JWT） ───
	mux.HandleFunc("/api/v1/adapter/device-events", adh.handleDeviceEvents)

	// ─── Admin auth — public (Go 1.20 compatible) ───
	mux.HandleFunc("/api/v1/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
			return
		}
		ah.login(w, r)
	})

	// ─── Admin protected ───
	mux.Handle("/api/v1/admin/me",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.me))))
	mux.Handle("/api/v1/admin/dashboard",
		authMiddleware(secret, adminRoles(http.HandlerFunc(s.dashboard))))
	mux.Handle("/api/v1/admin/users",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.handleUsers))))
	mux.Handle("/api/v1/admin/collectors",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.handleCollectors))))
	mux.Handle("/api/v1/admin/bindings",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.handleBindings))))
	mux.Handle("/api/v1/admin/bindings/",
		authMiddleware(secret, adminRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				ah.handleUnbind(w, r)
			} else if r.Method == http.MethodPost {
				ah.handleCreateBinding(w, r)
			} else {
				writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
			}
		}))))
	mux.Handle("/api/v1/admin/raw-events",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.handleRawEvents))))
	mux.Handle("/api/v1/admin/control-logs",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.handleControlLogs))))
	mux.Handle("/api/v1/admin/device-tree",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.handleDeviceTree))))
	mux.Handle("/api/v1/admin/farmers/",
		authMiddleware(secret, adminRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 路由分发：
			// GET  /api/v1/admin/farmers/{user_id}/bindings → 农户绑定列表
			// GET  /api/v1/admin/farmers/{user_id}          → 农户详情
			// PATCH /api/v1/admin/farmers/{user_id}         → 编辑农户资料
			path := r.URL.Path
			if r.Method == http.MethodGet {
				if hasSuffix(path, "/bindings") {
					ah.handleFarmerBindings(w, r)
				} else {
					ah.handleFarmerDetail(w, r)
				}
			} else if r.Method == http.MethodPatch {
				ah.handleFarmerUpdate(w, r)
			} else {
				writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
			}
		}))))
	// GET /api/v1/admin/farmers（不带尾随路径）→ 农户列表
	mux.Handle("/api/v1/admin/farmers",
		authMiddleware(secret, adminRoles(http.HandlerFunc(ah.handleFarmers))))

	// ─── Mini-program auth — public (dev/test only, Go 1.20 compatible) ───
	mux.HandleFunc("/api/v1/mini/mock-login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
			return
		}
		mh.mockLogin(w, r)
	})

	// ─── Mini-program protected ───
	mux.Handle("/api/v1/mini/me",
		authMiddleware(secret, farmerRoles(http.HandlerFunc(mh.handleMe))))
	mux.Handle("/api/v1/mini/devices",
		authMiddleware(secret, farmerRoles(http.HandlerFunc(mh.handleDevices))))
	mux.Handle("/api/v1/mini/devices/",
		authMiddleware(secret, farmerRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 路由分发：/api/v1/mini/devices/{id}/realtime 或 /history 或 /control
			path := r.URL.Path
			if r.Method == http.MethodGet {
				if hasSuffix(path, "/realtime") {
					mh.handleDeviceRealtime(w, r)
				} else if hasSuffix(path, "/history") {
					mh.handleDeviceHistory(w, r)
				} else {
					writeCode(w, http.StatusNotFound, 404, "not found")
				}
			} else if r.Method == http.MethodPost && hasSuffix(path, "/control") {
				mh.handleControl(w, r)
			} else {
				writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
			}
		}))))

	// ─── Gateway list — protected ───
	mux.Handle("/api/v1/gateways",
		authMiddleware(secret, adminRoles(http.HandlerFunc(s.listGateways))))

	// ─── Static frontend files (embedded in binary via //go:embed) ───
	// web.Static embeds static/admin, static/mini, static/mock.
	// Prefix-rewriting handler maps /admin/... → static/admin/... etc.
	fileServer := http.FileServer(http.FS(web.Static))
	servePrefixed := func(prefix string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = prefix + r.URL.Path
			fileServer.ServeHTTP(w, r)
		})
	}
	mux.Handle("/admin/", servePrefixed("static"))
	mux.Handle("/mini/", servePrefixed("static"))
	mux.Handle("/mock/", servePrefixed("static"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})

	return corsMiddleware(loggingMiddleware(mux))
}

func hasSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
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
