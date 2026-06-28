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

// miniRepository defines the data access methods required by mini-program handlers.
type miniRepository interface {
	GetUserByOpenID(ctx context.Context, openid string) (*domain.User, error)
	CreateUser(ctx context.Context, u *domain.User) error
	UpdateLastLogin(ctx context.Context, userID uint64, t time.Time) error
	GetUserByID(ctx context.Context, userID uint64) (*domain.User, error)
	// 获取用户绑定的功能点列表（C 端设备列表）。
	ListMiniDevices(ctx context.Context, userID uint64) ([]domain.MiniDevice, error)
	// 获取单个功能点详情（实时数据）。
	GetMiniDeviceDetail(ctx context.Context, pointID uint64) (*domain.MiniDeviceDetail, error)
	// 获取功能点历史曲线数据。
	GetMiniDeviceHistory(ctx context.Context, pointID uint64, rangeType string) ([]domain.MiniHistoryPoint, error)
	// 检查用户是否对某个功能点有 active 绑定关系。
	CheckBindRelation(ctx context.Context, userID uint64, pointID uint64) (bool, error)
}

// miniHandler handles all /api/v1/mini/* endpoints.
type miniHandler struct {
	repo   miniRepository
	jwtCfg config.JWTConfig
}

// mockLoginRequest is the JSON body for POST /api/v1/mini/mock-login.
type mockLoginRequest struct {
	MockOpenID   string `json:"mock_openid"`
	MockNickname string `json:"mock_nickname"`
}

// mockLogin handles POST /api/v1/mini/mock-login.
// This endpoint is for development/testing only and should be disabled in production.
func (h *miniHandler) mockLogin(w http.ResponseWriter, r *http.Request) {
	var req mockLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.MockOpenID == "" {
		writeCode(w, http.StatusBadRequest, 400, "mock_openid is required")
		return
	}

	user, err := h.repo.GetUserByOpenID(r.Context(), req.MockOpenID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}

	isNewUser := false
	if user == nil {
		// User does not exist yet — create a new farmer account.
		isNewUser = true
		displayName := req.MockNickname
		openID := req.MockOpenID
		newUser := &domain.User{
			UserType:     domain.UserTypeFarmer,
			Role:         domain.RoleFarmer,
			WechatOpenID: &openID,
			DisplayName:  &displayName,
			Status:       domain.UserStatusActive,
		}
		if err := h.repo.CreateUser(r.Context(), newUser); err != nil {
			writeCode(w, http.StatusInternalServerError, 500, "failed to create user")
			return
		}
		user = newUser
	}

	now := time.Now()
	claims := auth.Claims{
		UserID:   user.UserID,
		UserType: user.UserType,
		Role:     user.Role,
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(h.jwtCfg.FarmerTokenTTL).UnixMilli(),
	}
	token, err := auth.Sign(claims, []byte(h.jwtCfg.Secret))
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to sign token")
		return
	}

	// Best-effort update; do not fail the login on error.
	_ = h.repo.UpdateLastLogin(r.Context(), user.UserID, now)

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"token":       token,
			"user_id":     user.UserID,
			"is_new_user": isNewUser,
		},
	})
}

// handleMe 处理 GET /api/v1/mini/me。
func (h *miniHandler) handleMe(w http.ResponseWriter, r *http.Request) {
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

	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}

	// 统计绑定点位数量。
	devices, _ := h.repo.ListMiniDevices(r.Context(), claims.UserID)
	bindingCount := len(devices)

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user_id":       user.UserID,
			"display_name":  displayName,
			"role":          uint8(user.Role),
			"binding_count": bindingCount,
		},
	})
}

// handleDevices 处理 GET /api/v1/mini/devices。
// 返回当前用户绑定的所有功能点设备列表。
func (h *miniHandler) handleDevices(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusUnauthorized, 401, "missing claims")
		return
	}

	devices, err := h.repo.ListMiniDevices(r.Context(), claims.UserID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": devices})
}

// handleDeviceRealtime 处理 GET /api/v1/mini/devices/{id}/realtime。
// 返回指定功能点的实时数据详情。
func (h *miniHandler) handleDeviceRealtime(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusUnauthorized, 401, "missing claims")
		return
	}

	// 从路径中提取 point_id：/api/v1/mini/devices/{id}/realtime
	pointID, err := extractPointID(r.URL.Path, "/api/v1/mini/devices/", "/realtime")
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid point id")
		return
	}

	// 权限检查：用户只能查看自己绑定的功能点。
	bound, err := h.repo.CheckBindRelation(r.Context(), claims.UserID, pointID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	if !bound {
		writeCode(w, http.StatusForbidden, 403, "device not bound to current user")
		return
	}

	detail, err := h.repo.GetMiniDeviceDetail(r.Context(), pointID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	if detail == nil {
		writeCode(w, http.StatusNotFound, 404, "device not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}

// handleDeviceHistory 处理 GET /api/v1/mini/devices/{id}/history。
// 返回指定功能点的历史曲线数据。
func (h *miniHandler) handleDeviceHistory(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusUnauthorized, 401, "missing claims")
		return
	}

	// 从路径中提取 point_id：/api/v1/mini/devices/{id}/history
	pointID, err := extractPointID(r.URL.Path, "/api/v1/mini/devices/", "/history")
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid point id")
		return
	}

	// 权限检查。
	bound, err := h.repo.CheckBindRelation(r.Context(), claims.UserID, pointID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	if !bound {
		writeCode(w, http.StatusForbidden, 403, "device not bound to current user")
		return
	}

	// 先取功能点基本信息。
	detail, err := h.repo.GetMiniDeviceDetail(r.Context(), pointID)
	if err != nil || detail == nil {
		writeCode(w, http.StatusNotFound, 404, "device not found")
		return
	}

	// 读取 range 查询参数。
	rangeType := r.URL.Query().Get("range")
	if rangeType == "" {
		rangeType = "24h"
	}

	points, err := h.repo.GetMiniDeviceHistory(r.Context(), pointID, rangeType)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}

	history := domain.MiniDeviceHistory{
		PointID:  detail.PointID,
		FuncName: detail.FunctionName,
		Unit:     detail.Unit,
		BestMin:  detail.BestMin,
		BestMax:  detail.BestMax,
		Points:   points,
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": history})
}

// handleControl 处理 POST /api/v1/mini/devices/{id}/control。
// 控制设备（开/停/关）。当前阶段为占位实现，后续对接控制链路。
func (h *miniHandler) handleControl(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusUnauthorized, 401, "missing claims")
		return
	}

	pointID, err := extractPointID(r.URL.Path, "/api/v1/mini/devices/", "/control")
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid point id")
		return
	}

	// 权限检查。
	bound, err := h.repo.CheckBindRelation(r.Context(), claims.UserID, pointID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	if !bound {
		writeCode(w, http.StatusForbidden, 403, "device not bound to current user")
		return
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	// 控制命令占位：后续实现控制链路后再对接 Gateway。
	actionLabel := map[string]string{"start": "开启", "stop": "停止", "close": "关闭"}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"command_id":   time.Now().UnixMilli(),
			"action":       req.Action,
			"action_label": actionLabel[req.Action],
			"status":       "created",
			"message":      "控制命令已接收（控制链路开发中）",
		},
	})
}

// extractPointID 从路径中提取 {id} 参数。
// 例如：/api/v1/mini/devices/123/realtime → 123
func extractPointID(path, prefix, suffix string) (uint64, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, suffix)
	return strconv.ParseUint(trimmed, 10, 64)
}
