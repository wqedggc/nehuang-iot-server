package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
)

// adminRepository defines the data access methods required by admin handlers.
type adminRepository interface {
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
	GetUserByID(ctx context.Context, userID uint64) (*domain.User, error)
	UpdateLastLogin(ctx context.Context, userID uint64, t time.Time) error
	// 管理端用户列表。
	ListUsers(ctx context.Context) ([]domain.User, error)
	// 管理端采集执行器列表（全量）。
	ListCollectors(ctx context.Context) ([]domain.CollectorNode, error)
	// 管理端绑定关系列表（全量）。
	ListBindings(ctx context.Context) ([]domain.BindRelation, error)
	// 创建绑定关系。
	CreateBinding(ctx context.Context, b *domain.BindRelation) error
	// 解绑（软删除，更新 bind_status=2）。
	UnbindRelation(ctx context.Context, bindID uint64) error
	// 管理端原始事件列表。
	ListRawEvents(ctx context.Context, limit int) ([]domain.RawDeviceEvent, error)
	// 管理端控制日志列表。
	ListControlLogs(ctx context.Context, limit int) ([]domain.ControlLogSummary, error)
}

// adminHandler handles all /api/v1/admin/* endpoints.
type adminHandler struct {
	repo   adminRepository
	jwtCfg config.JWTConfig
}

// loginRequest is the JSON body for POST /api/v1/admin/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is the JSON payload returned on successful admin login.
type loginResponse struct {
	Token       string  `json:"token"`
	UserID      uint64  `json:"user_id"`
	Role        uint8   `json:"role"`
	DisplayName string  `json:"display_name"`
}

// login handles POST /api/v1/admin/login.
//
// It deliberately returns the same 401 message for "user not found" and
// "wrong password" to prevent username enumeration.
func (h *adminHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeCode(w, http.StatusBadRequest, 400, "username and password are required")
		return
	}

	user, err := h.repo.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	// User not found — return same message as wrong password.
	if user == nil || user.PasswordHash == nil || !auth.CheckPassword(req.Password, *user.PasswordHash) {
		writeCode(w, http.StatusUnauthorized, 4013, "invalid username or password")
		return
	}

	if user.Status != domain.UserStatusActive {
		writeCode(w, http.StatusForbidden, 4032, "account is disabled or deleted")
		return
	}
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleAdmin {
		writeCode(w, http.StatusForbidden, 4033, "insufficient role for admin login")
		return
	}

	now := time.Now()
	claims := auth.Claims{
		UserID:   user.UserID,
		UserType: user.UserType,
		Role:     user.Role,
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(h.jwtCfg.AdminTokenTTL).UnixMilli(),
	}
	token, err := auth.Sign(claims, []byte(h.jwtCfg.Secret))
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to sign token")
		return
	}

	// Best-effort update; do not fail the login on error.
	_ = h.repo.UpdateLastLogin(r.Context(), user.UserID, now)

	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": loginResponse{
			Token:       token,
			UserID:      user.UserID,
			Role:        uint8(user.Role),
			DisplayName: displayName,
		},
	})
}

// me handles GET /api/v1/admin/me.
// Requires authMiddleware + roleMiddleware(super_admin, admin).
func (h *adminHandler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusUnauthorized, 401, "missing claims")
		return
	}

	user, err := h.repo.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	if user == nil {
		writeCode(w, http.StatusNotFound, 404, "user not found")
		return
	}

	username := ""
	if user.Username != nil {
		username = *user.Username
	}
	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}

	var lastLoginStr *string
	if user.LastLoginAtMS != nil {
		s := user.LastLoginAtMS.UTC().Format("2006-01-02T15:04:05.000Z")
		lastLoginStr = &s
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user_id":          user.UserID,
			"username":         username,
			"role":             uint8(user.Role),
			"display_name":     displayName,
			"last_login_at_ms": lastLoginStr,
		},
	})
}

// handleUsers 处理 GET /api/v1/admin/users。
// 返回所有用户列表。
func (h *adminHandler) handleUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListUsers(r.Context())
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	// 转换为前端期望的格式。
	type userItem struct {
		UserID       uint64 `json:"user_id"`
		Username     string `json:"username"`
		DisplayName  string `json:"display_name"`
		Role         uint8  `json:"role"`
		RoleLabel    string `json:"role_label"`
		UserType     uint8  `json:"user_type"`
		Status       uint8  `json:"status"`
		StatusLabel  string `json:"status_label"`
		BindingCount int    `json:"binding_count"`
	}

	roleLabels := map[uint8]string{1: "超级管理员", 2: "管理员", 3: "农户"}
	statusLabels := map[uint8]string{1: "正常", 2: "已禁用", 3: "已删除"}

	items := make([]userItem, 0, len(users))
	for _, u := range users {
		uname := ""
		if u.Username != nil {
			uname = *u.Username
		}
		dname := ""
		if u.DisplayName != nil {
			dname = *u.DisplayName
		}
		items = append(items, userItem{
			UserID:       u.UserID,
			Username:     uname,
			DisplayName:  dname,
			Role:         uint8(u.Role),
			RoleLabel:    roleLabels[uint8(u.Role)],
			UserType:     uint8(u.UserType),
			Status:       uint8(u.Status),
			StatusLabel:  statusLabels[uint8(u.Status)],
			BindingCount: 0,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

// handleCollectors 处理 GET /api/v1/admin/collectors。
// 返回所有采集执行器列表（全量）。
func (h *adminHandler) handleCollectors(w http.ResponseWriter, r *http.Request) {
	collectors, err := h.repo.ListCollectors(r.Context())
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": collectors})
}

// handleBindings 处理 GET /api/v1/admin/bindings。
// 返回所有绑定关系列表。
func (h *adminHandler) handleBindings(w http.ResponseWriter, r *http.Request) {
	bindings, err := h.repo.ListBindings(r.Context())
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": bindings})
}

// handleCreateBinding 处理 POST /api/v1/admin/bindings。
// 创建新的绑定关系。
func (h *adminHandler) handleCreateBinding(w http.ResponseWriter, r *http.Request) {
	var b domain.BindRelation
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	if err := h.repo.CreateBinding(r.Context(), &b); err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to create binding: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": b})
}

// handleUnbind 处理 DELETE /api/v1/admin/bindings/{id}。
// 解绑（软删除）。
func (h *adminHandler) handleUnbind(w http.ResponseWriter, r *http.Request) {
	// 从路径提取 bind_id：/api/v1/admin/bindings/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/bindings/")
	bindID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid bind id")
		return
	}

	if err := h.repo.UnbindRelation(r.Context(), bindID); err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to unbind: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
}

// handleRawEvents 处理 GET /api/v1/admin/raw-events。
// 返回原始设备事件列表。
func (h *adminHandler) handleRawEvents(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	events, err := h.repo.ListRawEvents(r.Context(), limit)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": events})
}

// handleControlLogs 处理 GET /api/v1/admin/control-logs。
// 返回控制日志列表。
func (h *adminHandler) handleControlLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	logs, err := h.repo.ListControlLogs(r.Context(), limit)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": logs})
}
