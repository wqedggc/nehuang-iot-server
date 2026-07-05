package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
)

// ─── Mock repository for admin handler tests ──────────────────────────────────

type mockAdminRepo struct {
	userByUsername                *domain.User
	userByUsernameErr             error
	devicePointByID               *domain.DevicePoint
	gatewayByID                   *domain.Gateway
	activeBindingByPoint          *domain.BindRelation
	activeBindingByGateway        *domain.BindRelation
	activeBindingByUserAndGateway *domain.BindRelation
	updateDevicePointErr          error
	updatedPointID                uint64
	updatedPointInput             domain.DevicePointUpdateInput
	createdBinding                *domain.BindRelation
	createdControlInput           domain.CreateControlCommandInput
	createdControlCmd             *domain.ControlCommand
	gatewayCandidates             []domain.GatewayBindCandidate
	payloadEventDetail            *domain.PayloadEventDetail
}

func (m *mockAdminRepo) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	return m.userByUsername, m.userByUsernameErr
}
func (m *mockAdminRepo) GetUserByID(ctx context.Context, userID uint64) (*domain.User, error) {
	if m.userByUsername != nil {
		return m.userByUsername, nil
	}
	return nil, nil
}
func (m *mockAdminRepo) UpdateLastLogin(ctx context.Context, userID uint64, t time.Time) error {
	return nil
}
func (m *mockAdminRepo) ListUsers(ctx context.Context) ([]domain.User, error) { return nil, nil }
func (m *mockAdminRepo) GetFarmerByID(ctx context.Context, userID uint64) (*domain.User, int, error) {
	return nil, 0, nil
}
func (m *mockAdminRepo) UpdateFarmer(ctx context.Context, userID uint64, updates map[string]any) error {
	return nil
}
func (m *mockAdminRepo) ListFarmers(ctx context.Context, keyword string) ([]domain.User, []int, error) {
	return nil, nil, nil
}
func (m *mockAdminRepo) ListCollectors(ctx context.Context) ([]domain.CollectorNode, error) {
	return nil, nil
}
func (m *mockAdminRepo) ListBindings(ctx context.Context) ([]domain.BindRelation, error) {
	return nil, nil
}
func (m *mockAdminRepo) ListBindingsByUserID(ctx context.Context, userID uint64) ([]domain.BindRelation, error) {
	return nil, nil
}
func (m *mockAdminRepo) CreateBinding(ctx context.Context, b *domain.BindRelation) error {
	if b.BindID == 0 {
		b.BindID = 999
	}
	m.createdBinding = b
	return nil
}
func (m *mockAdminRepo) UnbindRelation(ctx context.Context, bindID uint64) error { return nil }
func (m *mockAdminRepo) ListRawEvents(ctx context.Context, limit int) ([]domain.RawDeviceEvent, error) {
	return nil, nil
}
func (m *mockAdminRepo) ListControlLogs(ctx context.Context, limit int) ([]domain.ControlLogSummary, error) {
	return nil, nil
}
func (m *mockAdminRepo) GetDeviceTree(ctx context.Context) ([]domain.DeviceTreeNode, error) {
	return nil, nil
}
func (m *mockAdminRepo) FindDevicePointByID(ctx context.Context, pointID uint64) (*domain.DevicePoint, error) {
	return m.devicePointByID, nil
}
func (m *mockAdminRepo) FindGatewayByID(ctx context.Context, gatewayID uint64) (*domain.Gateway, error) {
	return m.gatewayByID, nil
}
func (m *mockAdminRepo) FindCollectorNodeByID(ctx context.Context, nodeID uint64) (*domain.CollectorNode, error) {
	return nil, nil
}
func (m *mockAdminRepo) FindActiveBindingByUserAndPoint(ctx context.Context, userID uint64, pointID uint64) (*domain.BindRelation, error) {
	return nil, nil
}
func (m *mockAdminRepo) FindActiveBindingByUserAndGateway(ctx context.Context, userID uint64, gatewayID uint64) (*domain.BindRelation, error) {
	return m.activeBindingByUserAndGateway, nil
}
func (m *mockAdminRepo) FindActiveBindingByGateway(ctx context.Context, gatewayID uint64) (*domain.BindRelation, error) {
	return m.activeBindingByGateway, nil
}
func (m *mockAdminRepo) FindActiveBindingByPoint(ctx context.Context, pointID uint64) (*domain.BindRelation, error) {
	return m.activeBindingByPoint, nil
}
func (m *mockAdminRepo) SearchDevicePoints(ctx context.Context, keyword string, bindStatus string) ([]domain.DevicePointSearchResult, error) {
	return nil, nil
}
func (m *mockAdminRepo) ListGatewayBindCandidates(ctx context.Context, keyword string) ([]domain.GatewayBindCandidate, error) {
	return m.gatewayCandidates, nil
}
func (m *mockAdminRepo) UpdateDevicePointConfig(ctx context.Context, pointID uint64, input domain.DevicePointUpdateInput) error {
	m.updatedPointID = pointID
	m.updatedPointInput = input
	return m.updateDevicePointErr
}
func (m *mockAdminRepo) CreateControlCommandForPoint(ctx context.Context, input domain.CreateControlCommandInput) (*domain.ControlCommand, error) {
	m.createdControlInput = input
	if m.createdControlCmd != nil {
		return m.createdControlCmd, nil
	}
	return &domain.ControlCommand{
		CommandID:     123,
		PointID:       &input.PointID,
		CommandSource: input.CommandSource,
		CommandType:   input.CommandType,
		CommandHex:    input.CommandHex,
		CommandStatus: domain.CommandStatusCreated,
	}, nil
}
func (m *mockAdminRepo) ListPayloadEvents(ctx context.Context, params domain.PayloadEventQuery) ([]domain.PayloadEventListItem, int, error) {
	return nil, 0, nil
}
func (m *mockAdminRepo) GetPayloadEventDetail(ctx context.Context, rawEventID uint64) (*domain.PayloadEventDetail, error) {
	return m.payloadEventDetail, nil
}

// ─── Scenario 7: Admin login JWT contains aud="admin" ────────────────────────

func TestAdminLogin_JWTContainsAudAdmin(t *testing.T) {
	displayName := "超级管理员"
	username := "admin"
	passwordHash, _ := auth.HashPassword("admin123")
	repo := &mockAdminRepo{
		userByUsername: &domain.User{
			UserID:       1,
			UserType:     domain.UserTypeAdmin,
			Role:         domain.RoleSuperAdmin,
			Username:     &username,
			PasswordHash: &passwordHash,
			DisplayName:  &displayName,
			Status:       domain.UserStatusActive,
		},
	}

	jwtCfg := config.JWTConfig{
		Secret:         testSecret,
		AdminTokenTTL:  24 * time.Hour,
		FarmerTokenTTL: 72 * time.Hour,
	}

	handler := &adminHandler{repo: repo, jwtCfg: jwtCfg}

	body := `{"username":"admin","password":"admin123"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Parse response JSON
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing data field: %v", resp)
	}
	tokenStr, ok := data["token"].(string)
	if !ok {
		t.Fatalf("response data missing token: %v", data)
	}

	// Parse the JWT and verify aud
	claims, err := auth.Parse(tokenStr, []byte(testSecret))
	if err != nil {
		t.Fatalf("failed to parse admin token: %v", err)
	}
	if claims.Aud != "admin" {
		t.Errorf("admin token aud: want 'admin', got '%s'", claims.Aud)
	}
}

// ─── Test that admin login still works correctly with role check ──────────────

func TestAdminLogin_InvalidPassword_Returns4013(t *testing.T) {
	displayName := "超级管理员"
	username := "admin"
	passwordHash, _ := auth.HashPassword("admin123")
	repo := &mockAdminRepo{
		userByUsername: &domain.User{
			UserID:       1,
			UserType:     domain.UserTypeAdmin,
			Role:         domain.RoleSuperAdmin,
			Username:     &username,
			PasswordHash: &passwordHash,
			DisplayName:  &displayName,
			Status:       domain.UserStatusActive,
		},
	}

	jwtCfg := config.JWTConfig{
		Secret:         testSecret,
		AdminTokenTTL:  24 * time.Hour,
		FarmerTokenTTL: 72 * time.Hour,
	}

	handler := &adminHandler{repo: repo, jwtCfg: jwtCfg}

	body := `{"username":"admin","password":"wrongpassword"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: want 401, got %d", rec.Code)
	}
	// Should return code 4013
	if !contains(rec.Body.String(), "4013") {
		t.Errorf("wrong password should return code 4013, got: %s", rec.Body.String())
	}
}

func TestHandleDevicePointUpdate_Success(t *testing.T) {
	bestMax := 30.0
	repo := &mockAdminRepo{
		devicePointByID: &domain.DevicePoint{
			PointID: 1001,
			NodeID:  2001,
			BestMax: &bestMax,
		},
	}
	handler := &adminHandler{repo: repo}

	body := `{"point_name":" A棚温度 ","shed_no":" A棚 ","unit":"C","best_min":18,"run_mode":2,"controllable_status":1,"poll_interval":300,"poll_cmd":"0102","point_status":1,"remark":" 现场 "}`
	req := httptest.NewRequest("PATCH", "/api/v1/admin/device-points/1001", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleDevicePointUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("patch device point: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.updatedPointID != 1001 {
		t.Fatalf("updated point id: want 1001, got %d", repo.updatedPointID)
	}
	if repo.updatedPointInput.PointName == nil || *repo.updatedPointInput.PointName != "A棚温度" {
		t.Fatalf("point_name was not trimmed/applied: %#v", repo.updatedPointInput.PointName)
	}
	if repo.updatedPointInput.ShedNo == nil || *repo.updatedPointInput.ShedNo != "A棚" {
		t.Fatalf("shed_no was not trimmed/applied: %#v", repo.updatedPointInput.ShedNo)
	}
	if repo.updatedPointInput.PollCmd == nil || *repo.updatedPointInput.PollCmd != "0102" {
		t.Fatalf("poll_cmd not applied: %#v", repo.updatedPointInput.PollCmd)
	}
}

func TestHandleDevicePointUpdate_InvalidPollCmdReturns400(t *testing.T) {
	repo := &mockAdminRepo{
		devicePointByID: &domain.DevicePoint{PointID: 1001, NodeID: 2001},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("PATCH", "/api/v1/admin/device-points/1001", strings.NewReader(`{"poll_cmd":"XYZ"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleDevicePointUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid poll_cmd: want 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleDevicePointUpdate_NotFoundReturns404(t *testing.T) {
	handler := &adminHandler{repo: &mockAdminRepo{}}

	req := httptest.NewRequest("PATCH", "/api/v1/admin/device-points/404", strings.NewReader(`{"unit":"C"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleDevicePointUpdate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing point: want 404, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateBinding_GatewaySuccess(t *testing.T) {
	displayName := "张三"
	repo := &mockAdminRepo{
		userByUsername: &domain.User{
			UserID:      11,
			DisplayName: &displayName,
			Status:      domain.UserStatusActive,
			Role:        domain.RoleAdmin,
		},
		gatewayByID: &domain.Gateway{
			GatewayID:         9001,
			GatewayExternalID: "GW-9001",
			GatewayName:       "FP_GW_9001",
		},
	}
	handler := &adminHandler{repo: repo}

	body := `{"binding_type":"gateway","user_id":"11","gateway_id":"9001"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/bindings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleCreateBinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("gateway binding: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdBinding == nil {
		t.Fatal("gateway binding should call CreateBinding")
	}
	if repo.createdBinding.BindingType != "gateway" {
		t.Fatalf("binding type: want gateway, got %q", repo.createdBinding.BindingType)
	}
	if repo.createdBinding.GatewayID != 9001 || repo.createdBinding.PointID != nil || repo.createdBinding.NodeID != nil || repo.createdBinding.RealtimeID != nil {
		t.Fatalf("gateway binding should only keep gateway_id, got %#v", repo.createdBinding)
	}
	if repo.createdBinding.DeviceExternalID != "" || repo.createdBinding.Frequency != 0 || repo.createdBinding.FunctionType != 0 {
		t.Fatalf("gateway binding should clear point identity fields, got %#v", repo.createdBinding)
	}
}

func TestHandleCreateBinding_GatewayConflictReturns409(t *testing.T) {
	displayName := "张三"
	otherName := "李四"
	repo := &mockAdminRepo{
		userByUsername: &domain.User{
			UserID:      11,
			DisplayName: &displayName,
			Status:      domain.UserStatusActive,
			Role:        domain.RoleAdmin,
		},
		gatewayByID: &domain.Gateway{
			GatewayID:         9001,
			GatewayExternalID: "GW-9001",
			GatewayName:       "FP_GW_9001",
		},
		activeBindingByGateway: &domain.BindRelation{
			BindID:       777,
			UserID:       func() *uint64 { v := uint64(22); return &v }(),
			BindUserName: otherName,
			GatewayID:    9001,
			BindingType:  "gateway",
		},
	}
	handler := &adminHandler{repo: repo}

	body := `{"binding_type":"gateway","user_id":"11","gateway_id":"9001"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/bindings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleCreateBinding(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("gateway conflict: want 409, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdBinding != nil {
		t.Fatal("conflict should not create binding")
	}
}

func TestHandleCreateBinding_PointBindingStillWorks(t *testing.T) {
	displayName := "张三"
	repo := &mockAdminRepo{
		userByUsername: &domain.User{
			UserID:      11,
			DisplayName: &displayName,
			Status:      domain.UserStatusActive,
			Role:        domain.RoleAdmin,
		},
		gatewayByID: &domain.Gateway{
			GatewayID:         9001,
			GatewayExternalID: "GW-9001",
			GatewayName:       "FP_GW_9001",
		},
		devicePointByID: &domain.DevicePoint{
			PointID:          1001,
			NodeID:           2001,
			GatewayID:        9001,
			Frequency:        433,
			DeviceExternalID: "DEV-1",
			FunctionType:     domain.FuncTypeTemperature,
		},
	}
	handler := &adminHandler{repo: repo}

	body := `{"user_id":"11","point_id":"1001"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/bindings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleCreateBinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("point binding: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdBinding == nil || repo.createdBinding.BindingType != "point" {
		t.Fatalf("point binding should be created as point: %#v", repo.createdBinding)
	}
	if repo.createdBinding.PointID == nil || *repo.createdBinding.PointID != 1001 {
		t.Fatalf("point binding should keep point_id: %#v", repo.createdBinding)
	}
}

type bindingsListRepo struct {
	mockAdminRepo
	bindings []domain.BindRelation
}

func (r *bindingsListRepo) ListBindings(ctx context.Context) ([]domain.BindRelation, error) {
	return r.bindings, nil
}

type farmerBindingsListRepo struct {
	mockAdminRepo
	bindings []domain.BindRelation
}

func (r *farmerBindingsListRepo) ListBindingsByUserID(ctx context.Context, userID uint64) ([]domain.BindRelation, error) {
	return r.bindings, nil
}

func TestHandleBindings_ReturnsBindingTypeAndCollectorSummary(t *testing.T) {
	pointID := uint64(1001)
	repo := &bindingsListRepo{
		bindings: []domain.BindRelation{
			{
				BindID:            1,
				BindingType:       "gateway",
				GatewayID:         9001,
				GatewayName:       "FP_GW_9001",
				GatewayExternalID: "GW-9001",
				BindUserName:      "张三",
			},
			{
				BindID:            2,
				BindingType:       "point",
				GatewayID:         9002,
				GatewayName:       "FP_GW_9002",
				GatewayExternalID: "GW-9002",
				BindUserName:      "李四",
				PointID:           &pointID,
				Collector: &domain.BindingCollectorSummary{
					PointID:          pointID,
					FunctionName:     "温度",
					Frequency:        433,
					DeviceExternalID: "DEV-1",
					ShedNo:           "A棚",
				},
			},
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("GET", "/api/v1/admin/bindings", nil)
	rec := httptest.NewRecorder()
	handler.handleBindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("bindings list: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), `"binding_type":"gateway"`) {
		t.Fatalf("bindings list should expose gateway binding_type, body=%s", rec.Body.String())
	}
	if !contains(rec.Body.String(), `"collector":`) || !contains(rec.Body.String(), `"func_name":"温度"`) {
		t.Fatalf("bindings list should expose collector summary, body=%s", rec.Body.String())
	}
}

func TestHandleFarmerBindings_ReturnsBindingTypeAndCollectorSummary(t *testing.T) {
	pointID := uint64(2001)
	repo := &farmerBindingsListRepo{
		bindings: []domain.BindRelation{
			{
				BindID:            3,
				BindingType:       "point",
				GatewayID:         9003,
				GatewayName:       "FP_GW_9003",
				GatewayExternalID: "GW-9003",
				BindUserName:      "王五",
				PointID:           &pointID,
				Collector: &domain.BindingCollectorSummary{
					PointID:          pointID,
					FunctionName:     "墒情",
					Frequency:        433,
					DeviceExternalID: "DEV-2",
					ShedNo:           "B棚",
				},
			},
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("GET", "/api/v1/admin/farmers/11/bindings", nil)
	rec := httptest.NewRecorder()
	handler.handleFarmerBindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("farmer bindings: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), `"binding_type":"point"`) {
		t.Fatalf("farmer bindings should expose point binding_type, body=%s", rec.Body.String())
	}
}

func TestHandleGatewayBindCandidates_ReturnsUnboundGateways(t *testing.T) {
	repo := &mockAdminRepo{
		gatewayCandidates: []domain.GatewayBindCandidate{
			{
				GatewayID:         9001,
				GatewayName:       "FP_GW_9001",
				GatewayExternalID: "GW-9001",
			},
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("GET", "/api/v1/admin/gateway-bind-candidates?keyword=GW", nil)
	rec := httptest.NewRecorder()
	handler.handleGatewayBindCandidates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("gateway candidates: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), `"gateway_external_id":"GW-9001"`) {
		t.Fatalf("gateway candidates should include external id, body=%s", rec.Body.String())
	}
}

func TestHandleAdminDevicePointControl_CreatesPendingCommand(t *testing.T) {
	repo := &mockAdminRepo{
		devicePointByID: &domain.DevicePoint{
			PointID:          1001,
			NodeID:           2001,
			GatewayID:        9001,
			Frequency:        433,
			DeviceExternalID: "DEV-1",
		},
		createdControlCmd: &domain.ControlCommand{
			CommandID:     7788,
			CommandStatus: domain.CommandStatusCreated,
		},
	}
	handler := &adminHandler{repo: repo}
	adminRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin)
	protected := authMiddleware([]byte(testSecret), adminRoles(http.HandlerFunc(handler.handleDevicePointControl)))

	now := time.Now()
	token, err := auth.Sign(auth.Claims{
		UserID:   7,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleAdmin,
		Aud:      "admin",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(time.Hour).UnixMilli(),
	}, []byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/admin/device-points/1001/control", strings.NewReader(`{"action":"open"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("device point control: want 202, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdControlInput.PointID != 1001 {
		t.Fatalf("control command should target point 1001, got %d", repo.createdControlInput.PointID)
	}
	if repo.createdControlInput.CommandSource != domain.CommandSourceAdmin {
		t.Fatalf("control source should be admin, got %d", repo.createdControlInput.CommandSource)
	}
	if repo.createdControlInput.CommandHex == "" {
		t.Fatal("control command should carry hex payload")
	}
	if !contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("control response should be pending, body=%s", rec.Body.String())
	}
}

func TestHandleAdminDevicePointControl_CustomHex(t *testing.T) {
	repo := &mockAdminRepo{
		devicePointByID: &domain.DevicePoint{
			PointID:          1001,
			NodeID:           2001,
			GatewayID:        9001,
			Frequency:        433,
			DeviceExternalID: "DEV-1",
		},
		createdControlCmd: &domain.ControlCommand{
			CommandID:     7789,
			CommandStatus: domain.CommandStatusCreated,
			CommandHex:    "03030000000185E8",
		},
	}
	handler := &adminHandler{repo: repo}
	adminRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin)
	protected := authMiddleware([]byte(testSecret), adminRoles(http.HandlerFunc(handler.handleDevicePointControl)))
	token := mustAdminToken(t)

	req := httptest.NewRequest("POST", "/api/v1/admin/device-points/1001/control", strings.NewReader(`{"command_hex":"03 03 00 00 00 01 85 E8"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("device point control: want 202, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdControlInput.CommandType != domain.CommandTypeCustomHex {
		t.Fatalf("command type: want custom hex, got %d", repo.createdControlInput.CommandType)
	}
	if repo.createdControlInput.CommandHex != "03030000000185E8" {
		t.Fatalf("command hex = %q", repo.createdControlInput.CommandHex)
	}
}

func TestHandleAdminDevicePointControl_InvalidCustomHexReturns400(t *testing.T) {
	repo := &mockAdminRepo{
		devicePointByID: &domain.DevicePoint{PointID: 1001, NodeID: 2001},
	}
	handler := &adminHandler{repo: repo}
	adminRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin)
	protected := authMiddleware([]byte(testSecret), adminRoles(http.HandlerFunc(handler.handleDevicePointControl)))
	token := mustAdminToken(t)

	req := httptest.NewRequest("POST", "/api/v1/admin/device-points/1001/control", strings.NewReader(`{"command_hex":"not-hex"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid custom hex: want 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestDevicePointControlMiniTokenReturns403(t *testing.T) {
	repo := &mockAdminRepo{
		devicePointByID: &domain.DevicePoint{PointID: 1001, NodeID: 2001},
	}
	handler := &adminHandler{repo: repo}
	adminRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin)
	protected := authMiddleware([]byte(testSecret), adminRoles(http.HandlerFunc(handler.handleDevicePointControl)))

	now := time.Now()
	token, err := auth.Sign(auth.Claims{
		UserID:   7,
		UserType: domain.UserTypeFarmer,
		Role:     domain.RoleFarmer,
		Aud:      "mini",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(time.Hour).UnixMilli(),
	}, []byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/admin/device-points/1001/control", strings.NewReader(`{"action":"open"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("mini token on admin control: want 403, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestPayloadEventDetailIncludesProtocolSummary(t *testing.T) {
	repo := &mockAdminRepo{
		payloadEventDetail: &domain.PayloadEventDetail{
			RawEventID:      9001,
			EventType:       uint8(domain.RawEventUnknown),
			ParseStatus:     uint8(domain.ParseStatusPending),
			SourceType:      uint8(domain.SourceTypeGateway),
			PayloadHex:      "03030000000185E8",
			PayloadByteLen:  8,
			PayloadSnapshot: `{"payload_hex":"03030000000185E8"}`,
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("GET", "/api/v1/admin/payload-events/9001", nil)
	rec := httptest.NewRecorder()

	handler.handlePayloadEventDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("payload detail: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), `"payload_hex":"03030000000185E8"`) {
		t.Fatalf("payload_hex missing: body=%s", rec.Body.String())
	}
	if !contains(rec.Body.String(), `"kind":"modbus_read_holding_registers"`) {
		t.Fatalf("protocol summary missing: body=%s", rec.Body.String())
	}
}

func TestDevicePointUpdateMiniTokenReturns403(t *testing.T) {
	repo := &mockAdminRepo{
		devicePointByID: &domain.DevicePoint{PointID: 1001, NodeID: 2001},
	}
	handler := &adminHandler{repo: repo}
	adminRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin)
	protected := authMiddleware([]byte(testSecret), adminRoles(http.HandlerFunc(handler.handleDevicePointUpdate)))

	now := time.Now()
	token, err := auth.Sign(auth.Claims{
		UserID:   7,
		UserType: domain.UserTypeFarmer,
		Role:     domain.RoleFarmer,
		Aud:      "mini",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(time.Hour).UnixMilli(),
	}, []byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/api/v1/admin/device-points/1001", strings.NewReader(`{"unit":"C"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("mini token on admin patch: want 403, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func mustAdminToken(t *testing.T) string {
	t.Helper()
	now := time.Now()
	token, err := auth.Sign(auth.Claims{
		UserID:   7,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleAdmin,
		Aud:      "admin",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(time.Hour).UnixMilli(),
	}, []byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}
