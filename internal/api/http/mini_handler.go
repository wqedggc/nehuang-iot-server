package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	// 更新农户自编辑资料（display_name, mobile, address, remark）。
	UpdateMiniProfile(ctx context.Context, userID uint64, updates map[string]string) error
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
	repo      miniRepository
	jwtCfg    config.JWTConfig
	wechatCfg config.WechatConfig
}

// miniLoginRequest is the JSON body for POST /api/v1/mini/login.
type miniLoginRequest struct {
	Code      string `json:"code"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

// mockLoginRequest is the JSON body for POST /api/v1/mini/mock-login.
type mockLoginRequest struct {
	MockOpenID   string `json:"mock_openid"`
	MockNickname string `json:"mock_nickname"`
}

type wechatSessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

var wechatJscode2SessionURL = "https://api.weixin.qq.com/sns/jscode2session"

// login handles POST /api/v1/mini/login using a WeChat wx.login code.
func (h *miniHandler) login(w http.ResponseWriter, r *http.Request) {
	var req miniLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		writeCode(w, http.StatusBadRequest, 400, "code is required")
		return
	}

	session, err := h.exchangeWechatCode(r.Context(), req.Code)
	if err != nil {
		writeCode(w, http.StatusBadGateway, 502, err.Error())
		return
	}

	user, isNewUser, err := h.loginByOpenID(r.Context(), session.OpenID, req.Nickname, req.AvatarURL, session.UnionID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	h.writeLoginResponse(w, r, user, isNewUser)
}

func (h *miniHandler) exchangeWechatCode(ctx context.Context, code string) (*wechatSessionResponse, error) {
	if h.wechatCfg.AppID == "" || h.wechatCfg.Secret == "" {
		return nil, fmt.Errorf("wechat mini program credentials are not configured")
	}

	endpoint, err := url.Parse(wechatJscode2SessionURL)
	if err != nil {
		return nil, fmt.Errorf("invalid wechat login endpoint")
	}
	params := endpoint.Query()
	params.Set("appid", h.wechatCfg.AppID)
	params.Set("secret", h.wechatCfg.Secret)
	params.Set("js_code", code)
	params.Set("grant_type", "authorization_code")
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build wechat request")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat login request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wechat login returned status %d", resp.StatusCode)
	}

	var session wechatSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("invalid wechat login response")
	}
	if session.ErrCode != 0 {
		if session.ErrMsg == "" {
			session.ErrMsg = "wechat login failed"
		}
		return nil, fmt.Errorf("wechat login failed: %s", session.ErrMsg)
	}
	if session.OpenID == "" {
		return nil, fmt.Errorf("wechat login response missing openid")
	}
	return &session, nil
}

// mockLogin handles POST /api/v1/mini/mock-login.
// This endpoint is for development/testing only and is disabled by default.
func (h *miniHandler) mockLogin(w http.ResponseWriter, r *http.Request) {
	if !h.wechatCfg.EnableMockLogin {
		writeCode(w, http.StatusNotFound, 404, "mock login is disabled")
		return
	}

	var req mockLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.MockOpenID == "" {
		writeCode(w, http.StatusBadRequest, 400, "mock_openid is required")
		return
	}

	user, isNewUser, err := h.loginByOpenID(r.Context(), req.MockOpenID, req.MockNickname, "", "")
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	h.writeLoginResponse(w, r, user, isNewUser)
}

func (h *miniHandler) loginByOpenID(ctx context.Context, openID, nickname, avatarURL, unionID string) (*domain.User, bool, error) {
	user, err := h.repo.GetUserByOpenID(ctx, openID)
	if err != nil {
		return nil, false, err
	}

	isNewUser := false
	if user == nil {
		isNewUser = true
		displayName := strings.TrimSpace(nickname)
		if displayName == "" {
			suffix := openID
			if len(suffix) > 6 {
				suffix = suffix[len(suffix)-6:]
			}
			displayName = "微信农户_" + suffix
		}

		newUser := &domain.User{
			UserType:     domain.UserTypeFarmer,
			Role:         domain.RoleFarmer,
			WechatOpenID: &openID,
			DisplayName:  &displayName,
			Status:       domain.UserStatusActive,
		}
		if avatarURL != "" {
			newUser.AvatarURL = &avatarURL
		}
		if unionID != "" {
			newUser.WechatUnionID = &unionID
		}
		if err := h.repo.CreateUser(ctx, newUser); err != nil {
			return nil, false, err
		}
		user = newUser
	}

	return user, isNewUser, nil
}

func (h *miniHandler) writeLoginResponse(w http.ResponseWriter, r *http.Request, user *domain.User, isNewUser bool) {
	if user.UserType != domain.UserTypeFarmer || user.Role != domain.RoleFarmer {
		writeCode(w, http.StatusForbidden, 403, "mini login requires farmer account")
		return
	}
	if user.Status != domain.UserStatusActive {
		writeCode(w, http.StatusForbidden, 403, "user is disabled")
		return
	}

	now := time.Now()
	claims := auth.Claims{
		UserID:   user.UserID,
		UserType: user.UserType,
		Role:     user.Role,
		Aud:      "mini",
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

	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"token":        token,
			"user_id":      idStr(user.UserID),
			"is_new_user":  isNewUser,
			"display_name": displayName,
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

	// 统计绑定点位数量。
	devices, _ := h.repo.ListMiniDevices(r.Context(), claims.UserID)
	bindingCount := len(devices)

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user_id":       idStr(user.UserID),
			"display_name":  displayName,
			"mobile":        mobile,
			"address":       address,
			"remark":        remark,
			"role":          uint8(user.Role),
			"binding_count": bindingCount,
		},
	})
}

// handleUpdateMe 处理 PATCH /api/v1/mini/me。
// 允许农户修改自己的 display_name、mobile、address、remark。
// user_id 从 JWT claims 中获取，不允许请求体传入。
func (h *miniHandler) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusUnauthorized, 401, "missing claims")
		return
	}

	// 解析请求体。只接受白名单字符串字段，同时保留“字段未传”和“字段传了空字符串”的区别。
	var raw map[string]any
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	req := map[string]string{}
	for _, field := range []string{"display_name", "mobile", "address", "remark"} {
		val, ok := raw[field]
		if !ok {
			continue
		}
		strVal, ok := val.(string)
		if !ok {
			writeCode(w, http.StatusBadRequest, 400, field+" must be a string")
			return
		}
		req[field] = strVal
	}

	// 校验 display_name：1-64 字符。
	if displayName, ok := req["display_name"]; ok {
		runes := []rune(displayName)
		if len(runes) < 1 || len(runes) > 64 {
			writeCode(w, http.StatusBadRequest, 400, "display_name must be 1-64 characters")
			return
		}
	}

	// 校验 mobile/address/remark：最长 512 字符。
	for _, field := range []string{"mobile", "address", "remark"} {
		if val, ok := req[field]; ok {
			if len([]rune(val)) > 512 {
				writeCode(w, http.StatusBadRequest, 400, field+" must be at most 512 characters")
				return
			}
		}
	}

	// 构建更新 map，只包含请求体显式传入的白名单字段。
	updates := map[string]string{}
	for _, field := range []string{"display_name", "mobile", "address", "remark"} {
		if val, ok := req[field]; ok {
			updates[field] = val
		}
	}

	if err := h.repo.UpdateMiniProfile(r.Context(), claims.UserID, updates); err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}

	// 回读更新后的用户资料。
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

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user_id":      idStr(user.UserID),
			"display_name": displayName,
			"mobile":       mobile,
			"address":      address,
			"remark":       remark,
			"role":         uint8(user.Role),
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
