package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/protocolsummary"
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
	// 按 gateway_id 查询 gateway 记录。
	FindGatewayByID(ctx context.Context, gatewayID uint64) (*domain.Gateway, error)
	// 按 user_id + gateway_id 查询 active gateway 绑定。
	FindActiveBindingByUserAndGateway(ctx context.Context, userID uint64, gatewayID uint64) (*domain.BindRelation, error)
	// 按 gateway_id 查询任意 active gateway 绑定。
	FindActiveBindingByGateway(ctx context.Context, gatewayID uint64) (*domain.BindRelation, error)
	// 按 bind_id 查询绑定快照。
	FindBindingByID(ctx context.Context, bindID uint64) (*domain.BindRelation, error)
	// 解绑（软删除，更新 bind_status=2）。
	UnbindRelation(ctx context.Context, bindID uint64) error
	// 写入后台操作审计。
	CreateOperationRecord(ctx context.Context, record *domain.OperationRecord) error
	// 管理端原始事件列表。
	ListRawEvents(ctx context.Context, limit int) ([]domain.RawDeviceEvent, error)
	// 管理端控制日志列表。
	ListControlLogs(ctx context.Context, limit int) ([]domain.ControlLogSummary, error)
	// 管理端操作日志列表。
	ListOperationLogs(ctx context.Context, limit int) ([]domain.OperationRecord, error)
	// 设备树查询。
	GetDeviceTree(ctx context.Context) ([]domain.DeviceTreeNode, error)
	// 按 point_id 查询功能点。
	FindDevicePointByID(ctx context.Context, pointID uint64) (*domain.DevicePoint, error)
	// 按 node_id 查询采集执行器节点。
	FindCollectorNodeByID(ctx context.Context, nodeID uint64) (*domain.CollectorNode, error)
	// 按 user_id + point_id 查询 active 绑定。
	FindActiveBindingByUserAndPoint(ctx context.Context, userID uint64, pointID uint64) (*domain.BindRelation, error)
	// 按 point_id 查询 active 绑定（用于冲突检测：检查 point 是否已被任意用户绑定）。
	FindActiveBindingByPoint(ctx context.Context, pointID uint64) (*domain.BindRelation, error)
	// 管理端设备点位搜索。
	SearchDevicePoints(ctx context.Context, keyword string, bindStatus string) ([]domain.DevicePointSearchResult, error)
	// 管理端未绑定 gateway 候选列表。
	ListGatewayBindCandidates(ctx context.Context, keyword string) ([]domain.GatewayBindCandidate, error)
	// 管理端设备点位配置更新。
	UpdateDevicePointConfig(ctx context.Context, pointID uint64, input domain.DevicePointUpdateInput) error
	// 管理端为已注册设备网关联调创建采集执行器/功能点。
	SeedGatewayDevicePoints(ctx context.Context, gatewayID uint64, input domain.GatewayDeviceSeedInput) (*domain.GatewayDeviceSeedResult, error)
	// 管理端创建控制命令。
	CreateControlCommandForPoint(ctx context.Context, input domain.CreateControlCommandInput) (*domain.ControlCommand, error)
	// Payload 排查页面。
	ListPayloadEvents(ctx context.Context, params domain.PayloadEventQuery) ([]domain.PayloadEventListItem, int, error)
	GetPayloadEventDetail(ctx context.Context, rawEventID uint64) (*domain.PayloadEventDetail, error)
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
	Token       string `json:"token"`
	UserID      string `json:"user_id"`
	Role        uint8  `json:"role"`
	DisplayName string `json:"display_name"`
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
			UserID:      idStr(user.UserID),
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
		BindingCount string `json:"binding_count"`
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
			BindingCount: "0",
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
	BindingType string `json:"binding_type"`
	UserID      string `json:"user_id"`
	GatewayID   string `json:"gateway_id"`
	PointID     string `json:"point_id"`
}

func (h *adminHandler) handleCreateBinding(w http.ResponseWriter, r *http.Request) {
	var req createBindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	bindingType := domain.BindingType(strings.ToLower(strings.TrimSpace(req.BindingType)))
	if bindingType == "" {
		if strings.TrimSpace(req.PointID) != "" {
			bindingType = domain.BindingTypePoint
		} else if strings.TrimSpace(req.GatewayID) != "" {
			bindingType = domain.BindingTypeGateway
		}
	}
	if bindingType != domain.BindingTypeGateway && bindingType != domain.BindingTypePoint {
		writeCode(w, http.StatusBadRequest, 400, "binding_type must be gateway or point")
		return
	}
	if req.UserID == "" {
		writeCode(w, http.StatusBadRequest, 400, "user_id is required")
		return
	}
	if bindingType == domain.BindingTypeGateway && strings.TrimSpace(req.GatewayID) == "" {
		writeCode(w, http.StatusBadRequest, 400, "gateway_id is required for gateway binding")
		return
	}
	if bindingType == domain.BindingTypePoint && strings.TrimSpace(req.PointID) == "" {
		writeCode(w, http.StatusBadRequest, 400, "point_id is required for point binding")
		return
	}

	userID, err := strconv.ParseUint(req.UserID, 10, 64)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid user_id")
		return
	}
	ctx := r.Context()
	user, err := h.repo.GetUserByID(ctx, userID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if user == nil {
		writeCode(w, http.StatusNotFound, 404, "user not found")
		return
	}

	adminUserID := uint64(0)
	adminRole := uint8(0)
	if claims, ok := ClaimsFromContext(ctx); ok {
		adminUserID = claims.UserID
		adminRole = uint8(claims.Role)
	}

	bindUserName := ""
	if user.DisplayName != nil {
		bindUserName = *user.DisplayName
	}

	switch bindingType {
	case domain.BindingTypeGateway:
		gatewayID, err := strconv.ParseUint(req.GatewayID, 10, 64)
		if err != nil {
			writeCode(w, http.StatusBadRequest, 400, "invalid gateway_id")
			return
		}
		existingByGateway, err := h.repo.FindActiveBindingByGateway(ctx, gatewayID)
		if err != nil {
			writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
			return
		}
		if existingByGateway != nil {
			existingUserID := uint64(0)
			if existingByGateway.UserID != nil {
				existingUserID = *existingByGateway.UserID
			}
			if existingUserID == userID {
				writeJSON(w, http.StatusOK, map[string]any{"data": existingByGateway})
				return
			}
			writeJSON(w, http.StatusConflict, map[string]any{
				"code":    409,
				"message": "gateway already bound to another user",
				"data": map[string]any{
					"existing_user_id":   idStr(existingUserID),
					"existing_bind_id":   idStr(existingByGateway.BindID),
					"existing_user_name": existingByGateway.BindUserName,
				},
			})
			return
		}

		gateway, err := h.repo.FindGatewayByID(ctx, gatewayID)
		if err != nil {
			writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
			return
		}
		if gateway == nil {
			writeCode(w, http.StatusNotFound, 404, "gateway not found")
			return
		}

		b := &domain.BindRelation{
			UserID:            &userID,
			BindUserName:      bindUserName,
			BindingType:       string(domain.BindingTypeGateway),
			GatewayID:         gateway.GatewayID,
			GatewayName:       gateway.GatewayName,
			GatewayExternalID: gateway.GatewayExternalID,
			ShedNo:            "",
			Frequency:         0,
			DeviceExternalID:  "",
			FunctionType:      0,
			CreatedBy:         uint64Ptr(adminUserID),
		}
		if err := h.repo.CreateBinding(ctx, b); err != nil {
			if errors.Is(err, domain.ErrBindingConflict) {
				existingByGateway, _ := h.repo.FindActiveBindingByGateway(ctx, gatewayID)
				existingUserID := userID
				existingBindID := b.BindID
				existingUserName := b.BindUserName
				if existingByGateway != nil {
					existingBindID = existingByGateway.BindID
					existingUserName = existingByGateway.BindUserName
					if existingByGateway.UserID != nil {
						existingUserID = *existingByGateway.UserID
					}
				}
				writeJSON(w, http.StatusConflict, map[string]any{
					"code":    409,
					"message": "gateway already bound to another user",
					"data": map[string]any{
						"existing_user_id":   idStr(existingUserID),
						"existing_bind_id":   idStr(existingBindID),
						"existing_user_name": existingUserName,
					},
				})
				return
			}
			log.Printf("[handleCreateBinding] repo.CreateBinding failed: user_id=%d gateway_id=%d err=%v", userID, gatewayID, err)
			writeCode(w, http.StatusInternalServerError, 500, "failed to create binding: "+err.Error())
			return
		}
		if err := h.recordBindingOperation(ctx, domain.OpTypeBind, b, adminUserID, adminRole, "绑定设备网关"); err != nil {
			log.Printf("[handleCreateBinding] record bind operation failed: bind_id=%d err=%v", b.BindID, err)
			writeCode(w, http.StatusInternalServerError, 500, "failed to record binding operation: "+err.Error())
			return
		}

		log.Printf("[handleCreateBinding] success: bind_id=%d user_id=%d gateway_id=%d", b.BindID, userID, gatewayID)
		writeJSON(w, http.StatusOK, map[string]any{"data": b})
		return

	case domain.BindingTypePoint:
		pointID, err := strconv.ParseUint(req.PointID, 10, 64)
		if err != nil {
			writeCode(w, http.StatusBadRequest, 400, "invalid point_id")
			return
		}

		existingByPoint, err := h.repo.FindActiveBindingByPoint(ctx, pointID)
		if err != nil {
			writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
			return
		}
		if existingByPoint != nil {
			existingUserID := uint64(0)
			if existingByPoint.UserID != nil {
				existingUserID = *existingByPoint.UserID
			}
			if existingUserID == userID {
				writeJSON(w, http.StatusOK, map[string]any{"data": existingByPoint})
				return
			}
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

		point, err := h.repo.FindDevicePointByID(ctx, pointID)
		if err != nil {
			writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
			return
		}
		if point == nil {
			writeCode(w, http.StatusNotFound, 404, "device point not found")
			return
		}
		gateway, err := h.repo.FindGatewayByID(ctx, point.GatewayID)
		if err != nil {
			writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
			return
		}
		if gateway == nil {
			writeCode(w, http.StatusNotFound, 404, "gateway not found")
			return
		}
		collectorNode, err := h.repo.FindCollectorNodeByID(ctx, point.NodeID)
		if err != nil {
			writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
			return
		}
		collector := &domain.BindingCollectorSummary{
			PointID:          point.PointID,
			NodeID:           point.NodeID,
			PointName:        point.PointName,
			FunctionType:     point.FunctionType,
			FunctionName:     point.FunctionType.FunctionTypeName(),
			ChannelNo:        point.ChannelNo,
			ShedNo:           "",
			Frequency:        point.Frequency,
			DeviceExternalID: point.DeviceExternalID,
		}
		if collectorNode != nil {
			collector.ShedNo = collectorNode.ShedNo
		}
		b := &domain.BindRelation{
			UserID:            &userID,
			BindUserName:      bindUserName,
			BindingType:       string(domain.BindingTypePoint),
			GatewayID:         point.GatewayID,
			GatewayName:       gateway.GatewayName,
			GatewayExternalID: gateway.GatewayExternalID,
			NodeID:            &point.NodeID,
			PointID:           &point.PointID,
			ShedNo:            "",
			Frequency:         point.Frequency,
			DeviceExternalID:  point.DeviceExternalID,
			FunctionType:      point.FunctionType,
			Collector:         collector,
			CreatedBy:         uint64Ptr(adminUserID),
		}

		if err := h.repo.CreateBinding(ctx, b); err != nil {
			log.Printf("[handleCreateBinding] repo.CreateBinding failed: user_id=%d point_id=%d err=%v", userID, pointID, err)
			writeCode(w, http.StatusInternalServerError, 500, "failed to create binding: "+err.Error())
			return
		}
		if err := h.recordBindingOperation(ctx, domain.OpTypeBind, b, adminUserID, adminRole, "绑定功能点"); err != nil {
			log.Printf("[handleCreateBinding] record bind operation failed: bind_id=%d err=%v", b.BindID, err)
			writeCode(w, http.StatusInternalServerError, 500, "failed to record binding operation: "+err.Error())
			return
		}

		log.Printf("[handleCreateBinding] success: bind_id=%d user_id=%d point_id=%d", b.BindID, userID, pointID)
		writeJSON(w, http.StatusOK, map[string]any{"data": b})
		return
	}
}

// handleUnbind 处理 DELETE /api/v1/admin/bindings/{id}。
// 解绑（软删除）。
func (h *adminHandler) handleUnbind(w http.ResponseWriter, r *http.Request) {
	// 从路径提取 bind_id：/api/v1/admin/bindings/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/bindings/")
	path = strings.TrimSpace(path)
	bindID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid bind id")
		return
	}

	ctx := r.Context()
	binding, err := h.repo.FindBindingByID(ctx, bindID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to load binding: "+err.Error())
		return
	}
	if binding == nil {
		writeCode(w, http.StatusNotFound, 404, "binding not found")
		return
	}

	if err := h.repo.UnbindRelation(ctx, bindID); err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to unbind: "+err.Error())
		return
	}
	adminUserID := uint64(0)
	adminRole := uint8(0)
	if claims, ok := ClaimsFromContext(ctx); ok {
		adminUserID = claims.UserID
		adminRole = uint8(claims.Role)
	}
	if err := h.recordBindingOperation(ctx, domain.OpTypeUnbind, binding, adminUserID, adminRole, "解除绑定"); err != nil {
		log.Printf("[handleUnbind] record unbind operation failed: bind_id=%d err=%v", bindID, err)
		writeCode(w, http.StatusInternalServerError, 500, "failed to record unbind operation: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
}

func (h *adminHandler) recordBindingOperation(ctx context.Context, opType domain.OpType, b *domain.BindRelation, operatorID uint64, operatorRole uint8, action string) error {
	if b == nil {
		return nil
	}
	target := "设备网关"
	if b.PointID != nil {
		target = "功能点"
	}
	content := fmt.Sprintf("%s：%s %s，农户：%s，绑定ID：%s",
		action,
		target,
		b.GatewayName,
		b.BindUserName,
		idStr(b.BindID),
	)
	if b.PointID != nil {
		content = fmt.Sprintf("%s，功能点ID：%s", content, idStr(*b.PointID))
	}
	record := &domain.OperationRecord{
		GatewayID:        b.GatewayID,
		RealtimeID:       b.RealtimeID,
		NodeID:           b.NodeID,
		PointID:          b.PointID,
		UserID:           uint64Ptr(operatorID),
		BindUserName:     b.BindUserName,
		OperatorRole:     operatorRole,
		Frequency:        b.Frequency,
		DeviceExternalID: b.DeviceExternalID,
		RunMode:          1,
		OpType:           opType,
		OpContent:        content,
	}
	return h.repo.CreateOperationRecord(ctx, record)
}

// handleOperationLogs 处理 GET /api/v1/admin/operation-logs。
// 返回绑定、解绑、配置、控制等后台操作审计。
func (h *adminHandler) handleOperationLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	logs, err := h.repo.ListOperationLogs(r.Context(), limit)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": logs})
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

// handleDevicePointSearch 处理 GET /api/v1/admin/device-points。
// 支持 keyword 和 bind_status 查询参数。
func (h *adminHandler) handleDevicePointSearch(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	bindStatus := r.URL.Query().Get("bind_status")

	results, err := h.repo.SearchDevicePoints(r.Context(), keyword, bindStatus)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	// keyword 不为空时，在应用层做 function_name 的二次过滤
	// （因为 function_name 是通过 function_type 映射的中文名，SQL LIKE 无法直接匹配）
	if keyword != "" {
		kw := strings.ToLower(keyword)
		filtered := make([]domain.DevicePointSearchResult, 0, len(results))
		for _, r := range results {
			searchStr := strings.ToLower(r.PointName + r.GatewayName + r.DeviceExternalID + r.ShedNo + r.FunctionName)
			if strings.Contains(searchStr, kw) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": results})
}

// handleGatewayBindCandidates 处理 GET /api/v1/admin/gateway-bind-candidates。
func (h *adminHandler) handleGatewayBindCandidates(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	items, err := h.repo.ListGatewayBindCandidates(r.Context(), keyword)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

// handleGatewaySeedPoints 处理 POST /api/v1/admin/gateways/{gateway_id}/seed-points。
// 这是联调入口：在真实工作包 parser 未冻结前，允许管理员显式给已注册网关创建测试采集执行器和功能点。
func (h *adminHandler) handleGatewaySeedPoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/gateways/")
	path = strings.TrimSuffix(path, "/seed-points")
	path = strings.TrimSpace(path)
	gatewayID, err := strconv.ParseUint(path, 10, 64)
	if err != nil || gatewayID == 0 {
		writeCode(w, http.StatusBadRequest, 400, "invalid gateway id")
		return
	}

	gateway, err := h.repo.FindGatewayByID(r.Context(), gatewayID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if gateway == nil {
		writeCode(w, http.StatusNotFound, 404, "gateway not found")
		return
	}

	input, err := decodeGatewaySeedInput(r)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	result, err := h.repo.SeedGatewayDevicePoints(r.Context(), gatewayID, input)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to seed gateway points: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// handleDevicePointUpdate 处理 PATCH /api/v1/admin/device-points/{point_id}。
func (h *adminHandler) handleDevicePointUpdate(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/device-points/")
	path = strings.TrimSpace(path)
	pointID, err := strconv.ParseUint(path, 10, 64)
	if err != nil || pointID == 0 {
		writeCode(w, http.StatusBadRequest, 400, "invalid point id")
		return
	}

	point, err := h.repo.FindDevicePointByID(r.Context(), pointID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if point == nil {
		writeCode(w, http.StatusNotFound, 404, "device point not found")
		return
	}

	input, err := decodeDevicePointUpdateInput(r, point)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if !hasDevicePointUpdate(input) {
		writeCode(w, http.StatusBadRequest, 400, "no fields to update")
		return
	}

	if err := h.repo.UpdateDevicePointConfig(r.Context(), pointID, input); err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to update device point: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"point_id": idStr(pointID),
			"updated":  true,
		},
	})
}

// handleDevicePointControl 处理 POST /api/v1/admin/device-points/{point_id}/control。
func (h *adminHandler) handleDevicePointControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/device-points/")
	path = strings.TrimSuffix(path, "/control")
	path = strings.TrimSpace(path)
	pointID, err := strconv.ParseUint(path, 10, 64)
	if err != nil || pointID == 0 {
		writeCode(w, http.StatusBadRequest, 400, "invalid point id")
		return
	}

	point, err := h.repo.FindDevicePointByID(r.Context(), pointID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if point == nil {
		writeCode(w, http.StatusNotFound, 404, "device point not found")
		return
	}

	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusUnauthorized, 401, "missing claims")
		return
	}

	var req struct {
		Action     string `json:"action"`
		CommandHex string `json:"command_hex"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	input, actionLabel, err := buildAdminControlCommandInput(pointID, claims.UserID, req.Action, req.CommandHex)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	cmd, err := h.repo.CreateControlCommandForPoint(r.Context(), input)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to create control command")
		return
	}
	if cmd == nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to create control command")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"data": map[string]any{
			"command_id":   idStr(cmd.CommandID),
			"point_id":     idStr(point.PointID),
			"action":       req.Action,
			"action_label": actionLabel,
			"status":       cmd.CommandStatus.Label(),
			"command_hex":  cmd.CommandHex,
		},
	})
}

func buildAdminControlCommandInput(pointID uint64, userID uint64, action string, commandHex string) (domain.CreateControlCommandInput, string, error) {
	reqBy := userID
	extra := `{"source":"admin_control"}`
	input := domain.CreateControlCommandInput{
		PointID:       pointID,
		RequestedBy:   &reqBy,
		CommandSource: domain.CommandSourceAdmin,
		Extra:         &extra,
	}
	trimmedHex := normalizeHexCommand(commandHex)
	if trimmedHex != "" {
		if !isHexCommand(trimmedHex) {
			return input, "", fmt.Errorf("command_hex must be hex")
		}
		input.CommandType = domain.CommandTypeCustomHex
		input.CommandHex = trimmedHex
		return input, "自定义", nil
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open":
		rel := uint8(1)
		input.CommandType = domain.CommandTypeRelayOpen
		input.CommandHex = "F0"
		input.Rel1TargetStatus = &rel
		return input, "开启", nil
	case "close":
		rel := uint8(1)
		input.CommandType = domain.CommandTypeRelayClose
		input.CommandHex = "0F"
		input.Rel2TargetStatus = &rel
		return input, "关闭", nil
	case "stop":
		input.CommandType = domain.CommandTypeRelayStop
		input.CommandHex = "00"
		return input, "停止", nil
	default:
		return input, "", fmt.Errorf("action must be open, close, stop, or command_hex must be provided")
	}
}

type devicePointUpdateRequest struct {
	PointName          *string         `json:"point_name"`
	ShedNo             *string         `json:"shed_no"`
	Unit               *string         `json:"unit"`
	BestMin            *float64        `json:"best_min"`
	BestMax            *float64        `json:"best_max"`
	RunMode            *domain.RunMode `json:"run_mode"`
	ControllableStatus *uint8          `json:"controllable_status"`
	PollInterval       *int            `json:"poll_interval"`
	PollCmd            *string         `json:"poll_cmd"`
	PointStatus        *uint8          `json:"point_status"`
	Remark             *string         `json:"remark"`
}

func decodeGatewaySeedInput(r *http.Request) (domain.GatewayDeviceSeedInput, error) {
	var input domain.GatewayDeviceSeedInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, fmt.Errorf("invalid request body")
	}
	input.DeviceExternalID = strings.TrimSpace(input.DeviceExternalID)
	input.NodeName = strings.TrimSpace(input.NodeName)
	input.ShedNo = strings.TrimSpace(input.ShedNo)
	input.Remark = strings.TrimSpace(input.Remark)
	if input.Frequency <= 0 {
		input.Frequency = 1
	}
	if input.Frequency > 1000000 {
		return input, fmt.Errorf("frequency is too large")
	}
	if input.DeviceExternalID == "" {
		return input, fmt.Errorf("device_external_id is required")
	}
	if len(input.DeviceExternalID) > 64 {
		return input, fmt.Errorf("device_external_id is too long")
	}
	if input.NodeName == "" {
		input.NodeName = "联调采集执行器 " + input.DeviceExternalID
	}
	if input.ShedNo == "" {
		input.ShedNo = "联调棚"
	}
	if len(input.Points) == 0 {
		input.Points = defaultGatewaySeedPoints()
	}
	if len(input.Points) > 20 {
		return input, fmt.Errorf("points cannot exceed 20")
	}
	for i := range input.Points {
		p := &input.Points[i]
		p.PointName = strings.TrimSpace(p.PointName)
		p.Unit = strings.TrimSpace(p.Unit)
		p.PollCmd = normalizeHexCommand(p.PollCmd)
		p.Remark = strings.TrimSpace(p.Remark)
		if p.FunctionType < domain.FuncTypeTemperature || p.FunctionType > domain.FuncTypeFan {
			return input, fmt.Errorf("invalid function_type")
		}
		if p.ChannelNo <= 0 {
			p.ChannelNo = 1
		}
		if p.ChannelNo > 64 {
			return input, fmt.Errorf("channel_no is too large")
		}
		if p.PointName == "" {
			p.PointName = p.FunctionType.FunctionTypeName() + "·通道" + strconv.Itoa(p.ChannelNo)
		}
		if p.Unit == "" {
			p.Unit = defaultSeedUnit(p.FunctionType)
		}
		if p.ValueStatus == 0 {
			p.ValueStatus = domain.ValueStatusOffline
		}
		if p.RunMode == 0 {
			p.RunMode = domain.RunModeAuto
		}
		if p.RunMode != domain.RunModeAuto && p.RunMode != domain.RunModeManu {
			return input, fmt.Errorf("invalid run_mode")
		}
		if p.ControllableStatus > 1 {
			return input, fmt.Errorf("invalid controllable_status")
		}
		if p.PollInterval == 0 {
			p.PollInterval = 300
		}
		if p.PollInterval < 5 || p.PollInterval > 86400 {
			return input, fmt.Errorf("poll_interval must be between 5 and 86400")
		}
		if p.PointStatus == 0 {
			p.PointStatus = 1
		}
		if p.PointStatus > 3 {
			return input, fmt.Errorf("invalid point_status")
		}
		if p.BestMin != nil && p.BestMax != nil && *p.BestMin > *p.BestMax {
			return input, fmt.Errorf("best_min cannot be greater than best_max")
		}
	}
	return input, nil
}

func defaultGatewaySeedPoints() []domain.GatewayDeviceSeedPointInput {
	tempMin, tempMax, tempValue := 18.0, 32.0, 26.4
	soilMin, soilMax, soilValue := 35.0, 75.0, 54.9
	return []domain.GatewayDeviceSeedPointInput{
		{
			FunctionType: domain.FuncTypeTemperature,
			ChannelNo:    1,
			PointName:    "联调温度",
			Unit:         "C",
			BestMin:      &tempMin,
			BestMax:      &tempMax,
			RealValue:    &tempValue,
			ValueStatus:  domain.ValueStatusOffline,
			PollInterval: 300,
			PollCmd:      "03030000000185E8",
		},
		{
			FunctionType: domain.FuncTypeSoil,
			ChannelNo:    1,
			PointName:    "联调墒情",
			Unit:         "%",
			BestMin:      &soilMin,
			BestMax:      &soilMax,
			RealValue:    &soilValue,
			ValueStatus:  domain.ValueStatusOffline,
			PollInterval: 300,
		},
		{
			FunctionType:       domain.FuncTypePump,
			ChannelNo:          1,
			PointName:          "联调水泵",
			ValueStatus:        domain.ValueStatusOffline,
			RunMode:            domain.RunModeManu,
			ControllableStatus: 1,
			PollInterval:       300,
			PollCmd:            "F0",
		},
		{
			FunctionType:       domain.FuncTypeFan,
			ChannelNo:          1,
			PointName:          "联调风机",
			ValueStatus:        domain.ValueStatusOffline,
			RunMode:            domain.RunModeManu,
			ControllableStatus: 1,
			PollInterval:       300,
			PollCmd:            "0F",
		},
	}
}

func defaultSeedUnit(functionType domain.FunctionType) string {
	switch functionType {
	case domain.FuncTypeTemperature:
		return "C"
	case domain.FuncTypeSoil:
		return "%"
	default:
		return ""
	}
}

func decodeDevicePointUpdateInput(r *http.Request, existing *domain.DevicePoint) (domain.DevicePointUpdateInput, error) {
	var req devicePointUpdateRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return domain.DevicePointUpdateInput{}, err
	}

	input := domain.DevicePointUpdateInput{
		PointName:          trimStringPtr(req.PointName),
		ShedNo:             trimStringPtr(req.ShedNo),
		Unit:               trimStringPtr(req.Unit),
		BestMin:            req.BestMin,
		BestMax:            req.BestMax,
		RunMode:            req.RunMode,
		ControllableStatus: req.ControllableStatus,
		PollInterval:       req.PollInterval,
		PollCmd:            trimStringPtr(req.PollCmd),
		PointStatus:        req.PointStatus,
		Remark:             trimStringPtr(req.Remark),
	}

	if input.PointName != nil {
		if *input.PointName == "" || len([]rune(*input.PointName)) > 64 {
			return input, fmt.Errorf("point_name must be 1-64 characters")
		}
	}
	if input.ShedNo != nil && len([]rune(*input.ShedNo)) > 64 {
		return input, fmt.Errorf("shed_no must be at most 64 characters")
	}
	if input.Unit != nil && len([]rune(*input.Unit)) > 16 {
		return input, fmt.Errorf("unit must be at most 16 characters")
	}
	bestMin := existing.BestMin
	bestMax := existing.BestMax
	if input.BestMin != nil {
		bestMin = input.BestMin
	}
	if input.BestMax != nil {
		bestMax = input.BestMax
	}
	if bestMin != nil && bestMax != nil && *bestMin > *bestMax {
		return input, fmt.Errorf("best_min must be less than or equal to best_max")
	}
	if input.RunMode != nil && *input.RunMode != domain.RunModeAuto && *input.RunMode != domain.RunModeManu {
		return input, fmt.Errorf("run_mode must be 1 or 2")
	}
	if input.ControllableStatus != nil && *input.ControllableStatus > 1 {
		return input, fmt.Errorf("controllable_status must be 0 or 1")
	}
	if input.PollInterval != nil && (*input.PollInterval < 5 || *input.PollInterval > 86400) {
		return input, fmt.Errorf("poll_interval must be between 5 and 86400")
	}
	if input.PointStatus != nil && (*input.PointStatus < 1 || *input.PointStatus > 3) {
		return input, fmt.Errorf("point_status must be 1, 2, or 3")
	}
	if input.PollCmd != nil && !isHexCommand(*input.PollCmd) {
		return input, fmt.Errorf("poll_cmd must be an even-length hex string")
	}
	if input.Remark != nil && len([]rune(*input.Remark)) > 512 {
		return input, fmt.Errorf("remark must be at most 512 characters")
	}

	return input, nil
}

func hasDevicePointUpdate(input domain.DevicePointUpdateInput) bool {
	return input.PointName != nil ||
		input.ShedNo != nil ||
		input.Unit != nil ||
		input.BestMin != nil ||
		input.BestMax != nil ||
		input.RunMode != nil ||
		input.ControllableStatus != nil ||
		input.PollInterval != nil ||
		input.PollCmd != nil ||
		input.PointStatus != nil ||
		input.Remark != nil
}

func trimStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	return &trimmed
}

func uint64Ptr(v uint64) *uint64 {
	if v == 0 {
		return nil
	}
	return &v
}

func isHexCommand(v string) bool {
	v = normalizeHexCommand(v)
	if v == "" {
		return true
	}
	if len(v)%2 != 0 {
		return false
	}
	for _, r := range v {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}

func normalizeHexCommand(v string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(v)), ""))
}

// handleFarmerBindings 处理 GET /api/v1/admin/farmers/{user_id}/bindings。
// 返回指定农户的所有绑定关系（含 active 和已解绑）。
func (h *adminHandler) handleFarmerBindings(w http.ResponseWriter, r *http.Request) {
	// 从路径提取 user_id：/api/v1/admin/farmers/{user_id}/bindings
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/farmers/")
	path = strings.TrimSuffix(path, "/bindings")
	path = strings.TrimSpace(path)
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
	path = strings.TrimSpace(path)
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
			"binding_count": idStr(uint64(bindingCount)),
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
	path = strings.TrimSpace(path)
	userID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid user id")
		return
	}

	var req struct {
		DisplayName *string            `json:"display_name"`
		Mobile      *string            `json:"mobile"`
		Address     *string            `json:"address"`
		Remark      *string            `json:"remark"`
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
		BindingCount string `json:"binding_count"`
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
			BindingCount: idStr(uint64(bc)),
			CreatedAt:    u.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			UpdatedAt:    u.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

// handlePayloadEvents 处理 GET /api/v1/admin/payload-events。
// 分页查询 payload 事件列表，支持 event_type/parse_status/时间范围/keyword 筛选。
func (h *adminHandler) handlePayloadEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := domain.PayloadEventQuery{
		Page: 1,
		Size: 20,
	}
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			params.Page = n
		}
	}
	if v := q.Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			params.Size = n
		}
	}
	if v := q.Get("event_type"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			n8 := uint8(n)
			params.EventType = &n8
		}
	}
	if v := q.Get("parse_status"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			n8 := uint8(n)
			params.ParseStatus = &n8
		}
	}
	if v := q.Get("start"); v != "" {
		params.Start = &v
	}
	if v := q.Get("end"); v != "" {
		params.End = &v
	}
	if v := q.Get("keyword"); v != "" {
		params.Keyword = &v
	}

	items, total, err := h.repo.ListPayloadEvents(r.Context(), params)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"items": items,
			"total": total,
			"page":  params.Page,
			"size":  params.Size,
		},
	})
}

// handlePayloadEventDetail 处理 GET /api/v1/admin/payload-events/{id}。
// 返回单条 payload 事件的完整详情。
func (h *adminHandler) handlePayloadEventDetail(w http.ResponseWriter, r *http.Request) {
	// 从路径提取 raw_event_id：/api/v1/admin/payload-events/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/payload-events/")
	path = strings.TrimSpace(path)
	rawEventID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid raw event id")
		return
	}

	detail, err := h.repo.GetPayloadEventDetail(r.Context(), rawEventID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error: "+err.Error())
		return
	}
	if detail == nil {
		writeCode(w, http.StatusNotFound, 404, "payload event not found")
		return
	}
	if detail.PayloadHex != "" {
		detail.ProtocolSummary = protocolsummary.Build(detail.PayloadHex)
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}
