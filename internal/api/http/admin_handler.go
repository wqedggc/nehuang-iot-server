package http

import (
	"context"
	"encoding/json"
	"log"
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
	// 管理端农户详情（含绑定数）。
	GetFarmerByID(ctx context.Context, userID uint64) (*domain.User, int, error)
	// 管理端编辑农户资料。
	UpdateFarmer(ctx context.Context, userID uint64, updates map[string]any) error
	// 管理端农户列表（user_type=2, status<>3, keyword 模糊搜索）。
	ListFarmers(ctx context.Context, keyword string) ([]domain.User, []int, error)
	// 管理端采集执行器列表（全量）。
	ListCollectors(ctx context.Context) ([]domain.CollectorNode, error)
	// 管理端绑定关系列表（全量）。
	ListBindings(ctx context.Context) ([]domain.BindRelation, error)
	// 按用户 ID 查询绑定关系。
	ListBindingsByUserID(ctx context.Context, userID uint64) ([]domain.BindRelation, error)
	// 创建绑定关系。
	CreateBinding(ctx context.Context, b *domain.BindRelation) error
	// 解绑（软删除，更新 bind_status=2）。
	UnbindRelation(ctx context.Context, bindID uint64) error
	// 管理端原始事件列表。
	ListRawEvents(ctx context.Context, limit int) ([]domain.RawDeviceEvent, error)
	// 管理端控制日志列表。
	ListControlLogs(ctx context.Context, limit int) ([]domain.ControlLogSummary, error)
	// 设备树查询。
	GetDeviceTree(ctx context.Context) ([]domain.DeviceTreeNode, error)
	// 按 point_id 查询功能点。
	FindDevicePointByID(ctx context.Context, pointID uint64) (*domain.DevicePoint, error)
	// 按 user_id + point_id 查询 active 绑定。
	FindActiveBindingByUserAndPoint(ctx context.Context, userID uint64, pointID uint64) (*domain.BindRelation, error)
	// 按 point_id 查询 active 绑定（用于冲突检测：检查 point 是否已被任意用户绑定）。
	FindActiveBindingByPoint(ctx context.Context, pointID uint64) (*domain.BindRelation, error)
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
		Aud:      "admin",
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
			"user_id":          idStr(user.UserID),
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
		UserID       string `json:"user_id"`
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
			UserID:       idStr(u.UserID),
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
		log.Printf("[handleBindings] ListBindings failed: %v", err)
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	log.Printf("[handleBindings] returned %d bindings", len(bindings))
	writeJSON(w, http.StatusOK, map[string]any{"data": bindings})
}

// createBindingRequest is the JSON body for POST /api/v1/admin/bindings.
type createBindingRequest struct {
	UserID  uint64 `json:"user_id"`
	PointID uint64 `json:"point_id"`
}

// handleCreateBinding 处理 POST /api/v1/admin/bindings。
// 以 point_id 为核心创建绑定关系。
// 规则：
//   - point 已 active 绑定给同一用户 → 幂等返回 200（已有绑定）
//   - point 已 active 绑定给其他用户 → 返回 409 conflict
//   - point 无 active 绑定 → 创建新绑定
func (h *adminHandler) handleCreateBinding(w http.ResponseWriter, r *http.Request) {
	var req createBindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.UserID == 0 || req.PointID == 0 {
		writeCode(w, http.StatusBadRequest, 400, "user_id and point_id are required")
		return
	}

	ctx := r.Context()

	// Step 1: 检查 point 是否已被任意用户 active 绑定（冲突检测）。
	existingByPoint, err := h.repo.FindActiveBindingByPoint(ctx, req.PointID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if existingByPoint != nil {
		existingUserID := uint64(0)
		if existingByPoint.UserID != nil {
			existingUserID = *existingByPoint.UserID
		}
		if existingUserID == req.UserID {
			// 同一用户重复绑定 → 幂等返回
			writeJSON(w, http.StatusOK, map[string]any{"data": existingByPoint})
			return
		}
		// 其他用户已绑定 → conflict
		writeJSON(w, http.StatusConflict, map[string]any{
			"code":    409,
			"message": "device point already bound to another user",
			"data": map[string]any{
				"existing_user_id":   idStr(existingUserID),
				"existing_bind_id":   idStr(existingByPoint.BindID),
				"existing_user_name": existingByPoint.BindUserName,
			},
		})
		return
	}

	// Step 2: 查询 device_point 补全字段。
	point, err := h.repo.FindDevicePointByID(ctx, req.PointID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if point == nil {
		writeCode(w, http.StatusNotFound, 404, "device point not found")
		return
	}

	// Step 3: 查询用户信息获取 bind_user_name。
	user, err := h.repo.GetUserByID(ctx, req.UserID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if user == nil {
		writeCode(w, http.StatusNotFound, 404, "user not found")
		return
	}

	bindUserName := ""
	if user.DisplayName != nil {
		bindUserName = *user.DisplayName
	}

	b := &domain.BindRelation{
		UserID:           &req.UserID,
		BindUserName:     bindUserName,
		GatewayID:        point.GatewayID,
		NodeID:           &point.NodeID,
		PointID:          &point.PointID,
		ShedNo:           "",
		Frequency:        point.Frequency,
		DeviceExternalID: point.DeviceExternalID,
		FunctionType:     point.FunctionType,
	}

	if err := h.repo.CreateBinding(ctx, b); err != nil {
		log.Printf("[handleCreateBinding] repo.CreateBinding failed: user_id=%d point_id=%d err=%v", req.UserID, req.PointID, err)
		writeCode(w, http.StatusInternalServerError, 500, "failed to create binding: "+err.Error())
		return
	}

	log.Printf("[handleCreateBinding] success: bind_id=%d user_id=%d point_id=%d", b.BindID, req.UserID, req.PointID)

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

// handleDeviceTree 处理 GET /api/v1/admin/device-tree。
// 返回设备树：gateway → collector → point 三层结构。
func (h *adminHandler) handleDeviceTree(w http.ResponseWriter, r *http.Request) {
	tree, err := h.repo.GetDeviceTree(r.Context())
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": tree})
}

// handleFarmerBindings 处理 GET /api/v1/admin/farmers/{user_id}/bindings。
// 返回指定农户的所有绑定关系（含 active 和已解绑）。
func (h *adminHandler) handleFarmerBindings(w http.ResponseWriter, r *http.Request) {
	// 从路径提取 user_id：/api/v1/admin/farmers/{user_id}/bindings
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/farmers/")
	path = strings.TrimSuffix(path, "/bindings")
	userID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid user id")
		return
	}

	bindings, err := h.repo.ListBindingsByUserID(r.Context(), userID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": bindings})
}

// handleFarmerDetail 处理 GET /api/v1/admin/farmers/{user_id}。
// 返回农户详情，含 binding_count。
func (h *adminHandler) handleFarmerDetail(w http.ResponseWriter, r *http.Request) {
	// 从路径提取 user_id：/api/v1/admin/farmers/{user_id}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/farmers/")
	// 可能带有 /bindings 后缀，需要处理
	path = strings.TrimSuffix(path, "/bindings")
	userID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid user id")
		return
	}

	user, bindingCount, err := h.repo.GetFarmerByID(r.Context(), userID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if user == nil {
		writeCode(w, http.StatusNotFound, 404, "farmer not found")
		return
	}

	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}
	mobile := ""
	if user.Mobile != nil {
		mobile = *user.Mobile
	}
	address := ""
	if user.Address != nil {
		address = *user.Address
	}
	remark := ""
	if user.Remark != nil {
		remark = *user.Remark
	}
	wechatOpenID := ""
	if user.WechatOpenID != nil {
		wechatOpenID = *user.WechatOpenID
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user_id":       idStr(user.UserID),
			"display_name":  displayName,
			"mobile":        mobile,
			"address":       address,
			"remark":        remark,
			"wechat_openid": wechatOpenID,
			"status":        uint8(user.Status),
			"binding_count": bindingCount,
			"created_at":    user.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			"updated_at":    user.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		},
	})
}

// handleFarmerUpdate 处理 PATCH /api/v1/admin/farmers/{user_id}。
// 编辑农户资料：display_name, mobile, address, remark, status。
func (h *adminHandler) handleFarmerUpdate(w http.ResponseWriter, r *http.Request) {
	// 从路径提取 user_id：/api/v1/admin/farmers/{user_id}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/farmers/")
	userID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid user id")
		return
	}

	var req struct {
		DisplayName *string           `json:"display_name"`
		Mobile      *string           `json:"mobile"`
		Address     *string           `json:"address"`
		Remark      *string           `json:"remark"`
		Status      *domain.UserStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	updates := map[string]any{
		"display_name": nil,
		"mobile":       nil,
		"address":      nil,
		"remark":       nil,
		"status":       nil,
	}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Mobile != nil {
		updates["mobile"] = *req.Mobile
	}
	if req.Address != nil {
		updates["address"] = *req.Address
	}
	if req.Remark != nil {
		updates["remark"] = *req.Remark
	}
	if req.Status != nil {
		updates["status"] = uint8(*req.Status)
	}

	if err := h.repo.UpdateFarmer(r.Context(), userID, updates); err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to update farmer: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"updated": true}})
}

// handleFarmers 处理 GET /api/v1/admin/farmers。
// 返回农户列表（user_type=2, status<>3），支持 keyword 搜索。
func (h *adminHandler) handleFarmers(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")

	users, bindingCounts, err := h.repo.ListFarmers(r.Context(), keyword)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	type farmerItem struct {
		UserID       string `json:"user_id"`
		DisplayName  string `json:"display_name"`
		Mobile       string `json:"mobile"`
		WechatOpenID string `json:"wechat_openid"`
		Status       uint8  `json:"status"`
		StatusLabel  string `json:"status_label"`
		BindingCount int    `json:"binding_count"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}

	statusLabels := map[uint8]string{1: "正常", 2: "已禁用", 3: "已删除"}

	items := make([]farmerItem, 0, len(users))
	for i, u := range users {
		dname := ""
		if u.DisplayName != nil {
			dname = *u.DisplayName
		}
		mobile := ""
		if u.Mobile != nil {
			mobile = *u.Mobile
		}
		openID := ""
		if u.WechatOpenID != nil {
			openID = *u.WechatOpenID
		}
		bc := 0
		if i < len(bindingCounts) {
			bc = bindingCounts[i]
		}
		items = append(items, farmerItem{
			UserID:       idStr(u.UserID),
			DisplayName:  dname,
			Mobile:       mobile,
			WechatOpenID: openID,
			Status:       uint8(u.Status),
			StatusLabel:  statusLabels[uint8(u.Status)],
			BindingCount: bc,
			CreatedAt:    u.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			UpdatedAt:    u.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}
