package http

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

// ─── Mock repository for admin handler tests ──────────────────────────────────

type mockAdminRepo struct {
	userByUsername                *domain.User
	userByUsernameErr             error
	devicePointByID               *domain.DevicePoint
	collectorNodeByID             *domain.CollectorNode
	gatewayByID                   *domain.Gateway
	activeBindingByPoint          *domain.BindRelation
	activeBindingByNode           *domain.BindRelation
	activeBindingByGateway        *domain.BindRelation
	activeBindingByUserAndGateway *domain.BindRelation
	updatedGatewayAccessModeID    uint64
	updatedGatewayAccessMode      domain.GatewayAccessMode
	updateDevicePointErr          error
	updatedPointID                uint64
	updatedPointInput             domain.DevicePointUpdateInput
	createdBinding                *domain.BindRelation
	bindingByID                   *domain.BindRelation
	createdOperation              *domain.OperationRecord
	createdControlInput           domain.CreateControlCommandInput
	createdControlCmd             *domain.ControlCommand
	seedGatewayID                 uint64
	seedGatewayInput              domain.GatewayDeviceSeedInput
	seedGatewayResult             *domain.GatewayDeviceSeedResult
	gatewayCandidates             []domain.GatewayBindCandidate
	payloadEventDetail            *domain.PayloadEventDetail
	payloadEvents                 []domain.PayloadEventListItem
	payloadEventTotal             int
	payloadEventQuery             domain.PayloadEventQuery
	operationLogs                 []domain.OperationRecord
	operationTotal                int
	operationLogQuery             domain.OperationLogQuery
	controlLogs                   []domain.ControlLogSummary
	controlTotal                  int
	controlLogQuery               domain.ControlLogQuery
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
func (m *mockAdminRepo) ListMiniDevices(ctx context.Context, userID uint64) ([]domain.MiniDevice, error) {
	return nil, nil
}
func (m *mockAdminRepo) GetMiniDeviceDetail(ctx context.Context, pointID uint64) (*domain.MiniDeviceDetail, error) {
	return nil, nil
}
func (m *mockAdminRepo) CheckBindRelation(ctx context.Context, userID uint64, pointID uint64) (bool, error) {
	return false, nil
}
func (m *mockAdminRepo) GetMiniDeviceHistory(ctx context.Context, pointID uint64, rangeType string) ([]domain.MiniHistoryPoint, error) {
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
func (m *mockAdminRepo) FindBindingByID(ctx context.Context, bindID uint64) (*domain.BindRelation, error) {
	return m.bindingByID, nil
}
func (m *mockAdminRepo) CreateOperationRecord(ctx context.Context, record *domain.OperationRecord) error {
	m.createdOperation = record
	return nil
}
func (m *mockAdminRepo) ListRawEvents(ctx context.Context, limit int) ([]domain.RawDeviceEvent, error) {
	return nil, nil
}
func (m *mockAdminRepo) ListControlLogs(ctx context.Context, params domain.ControlLogQuery) ([]domain.ControlLogSummary, int, error) {
	m.controlLogQuery = params
	return m.controlLogs, m.controlTotal, nil
}
func (m *mockAdminRepo) ListOperationLogs(ctx context.Context, params domain.OperationLogQuery) ([]domain.OperationRecord, int, error) {
	m.operationLogQuery = params
	return m.operationLogs, m.operationTotal, nil
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
func (m *mockAdminRepo) UpdateGatewayAccessMode(ctx context.Context, gatewayID uint64, mode domain.GatewayAccessMode) error {
	m.updatedGatewayAccessModeID = gatewayID
	m.updatedGatewayAccessMode = mode
	return nil
}
func (m *mockAdminRepo) FindCollectorNodeByID(ctx context.Context, nodeID uint64) (*domain.CollectorNode, error) {
	return m.collectorNodeByID, nil
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
func (m *mockAdminRepo) FindActiveBindingByUserAndNode(ctx context.Context, userID uint64, nodeID uint64) (*domain.BindRelation, error) {
	return nil, nil
}
func (m *mockAdminRepo) FindActiveBindingByNode(ctx context.Context, nodeID uint64) (*domain.BindRelation, error) {
	return m.activeBindingByNode, nil
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
func (m *mockAdminRepo) SeedGatewayDevicePoints(ctx context.Context, gatewayID uint64, input domain.GatewayDeviceSeedInput) (*domain.GatewayDeviceSeedResult, error) {
	m.seedGatewayID = gatewayID
	m.seedGatewayInput = input
	if m.seedGatewayResult != nil {
		return m.seedGatewayResult, nil
	}
	return &domain.GatewayDeviceSeedResult{GatewayID: gatewayID, NodeID: 456, PointIDs: []string{"789"}, CreatedPoints: 1}, nil
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
	m.payloadEventQuery = params
	return m.payloadEvents, m.payloadEventTotal, nil
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
			AccessMode:        string(domain.GatewayAccessModeExclusive),
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
	if repo.createdOperation == nil || repo.createdOperation.OpType != domain.OpTypeBind {
		t.Fatalf("gateway binding should create bind operation log, got %#v", repo.createdOperation)
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
			AccessMode:        string(domain.GatewayAccessModeExclusive),
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

func TestHandleCreateBinding_SharedGatewayAllowsMultipleBindings(t *testing.T) {
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
			AccessMode:        string(domain.GatewayAccessModeShared),
		},
		activeBindingByGateway: &domain.BindRelation{
			BindID:       777,
			UserID:       func() *uint64 { v := uint64(22); return &v }(),
			BindUserName: "李四",
			GatewayID:    9001,
			BindingType:  string(domain.BindingTypeGateway),
		},
	}
	handler := &adminHandler{repo: repo}

	body := `{"binding_type":"gateway","user_id":"11","gateway_id":"9001"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/bindings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleCreateBinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("shared gateway binding: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdBinding == nil || repo.createdBinding.BindingType != string(domain.BindingTypeGateway) {
		t.Fatalf("shared gateway should create gateway binding, got %#v", repo.createdBinding)
	}
}

func TestHandleCreateBinding_CollectorSuccess(t *testing.T) {
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
			AccessMode:        string(domain.GatewayAccessModeShared),
		},
		collectorNodeByID: &domain.CollectorNode{
			NodeID:           2001,
			GatewayID:        9001,
			Frequency:        2,
			DeviceExternalID: "1001",
			NodeName:         "A棚采集执行器",
			ShedNo:           "A棚",
		},
	}
	handler := &adminHandler{repo: repo}

	body := `{"binding_type":"collector","user_id":"11","node_id":"2001"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/bindings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleCreateBinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("collector binding: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdBinding == nil || repo.createdBinding.BindingType != string(domain.BindingTypeCollector) {
		t.Fatalf("collector binding should be created as collector: %#v", repo.createdBinding)
	}
	if repo.createdBinding.NodeID == nil || *repo.createdBinding.NodeID != 2001 {
		t.Fatalf("collector binding should keep node_id: %#v", repo.createdBinding)
	}
	if repo.createdOperation == nil || repo.createdOperation.OpType != domain.OpTypeBind {
		t.Fatalf("collector binding should create bind operation log, got %#v", repo.createdOperation)
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
	if repo.createdOperation == nil || repo.createdOperation.OpType != domain.OpTypeBind {
		t.Fatalf("point binding should create bind operation log, got %#v", repo.createdOperation)
	}
}

func TestHandleGatewayUpdate_AccessModeWritesOperationLog(t *testing.T) {
	repo := &mockAdminRepo{
		gatewayByID: &domain.Gateway{
			GatewayID:         9001,
			GatewayExternalID: "GW-9001",
			GatewayName:       "FP_GW_9001",
			AccessMode:        string(domain.GatewayAccessModeShared),
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("PATCH", "/api/v1/admin/gateways/9001", strings.NewReader(`{"access_mode":"exclusive"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.handleGatewayUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("gateway update: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.updatedGatewayAccessModeID != 9001 || repo.updatedGatewayAccessMode != domain.GatewayAccessModeExclusive {
		t.Fatalf("gateway mode update mismatch: id=%d mode=%s", repo.updatedGatewayAccessModeID, repo.updatedGatewayAccessMode)
	}
	if repo.createdOperation == nil || repo.createdOperation.OpType != domain.OpTypeConfig {
		t.Fatalf("gateway update should create config operation log, got %#v", repo.createdOperation)
	}
}

func TestHandleUnbind_CreatesOperationLog(t *testing.T) {
	bindUserName := "张三"
	pointID := uint64(1001)
	repo := &mockAdminRepo{
		bindingByID: &domain.BindRelation{
			BindID:           99,
			BindUserName:     bindUserName,
			BindingType:      string(domain.BindingTypePoint),
			GatewayID:        9001,
			GatewayName:      "FP_GW_9001",
			PointID:          &pointID,
			Frequency:        433,
			DeviceExternalID: "DEV-1",
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("DELETE", "/api/v1/admin/bindings/99", nil)
	rec := httptest.NewRecorder()

	handler.handleUnbind(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unbind: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdOperation == nil || repo.createdOperation.OpType != domain.OpTypeUnbind {
		t.Fatalf("unbind should create unbind operation log, got %#v", repo.createdOperation)
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
					PointID:          &pointID,
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
					PointID:          &pointID,
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

func TestHandleGatewaySeedPoints_Defaults(t *testing.T) {
	const largeGatewayID = uint64(332018003367432192)
	repo := &mockAdminRepo{
		gatewayByID: &domain.Gateway{
			GatewayID:   largeGatewayID,
			GatewayName: "FP_TEST_ADMIN_002",
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("POST", "/api/v1/admin/gateways/332018003367432192/seed-points", strings.NewReader(`{"frequency":2,"device_external_id":"1002"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.handleGatewaySeedPoints(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("gateway seed: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.seedGatewayID != largeGatewayID {
		t.Fatalf("gateway id: want %d, got %d", largeGatewayID, repo.seedGatewayID)
	}
	if repo.seedGatewayInput.Frequency != 2 || repo.seedGatewayInput.DeviceExternalID != "1002" {
		t.Fatalf("unexpected seed input: %#v", repo.seedGatewayInput)
	}
	if len(repo.seedGatewayInput.Points) != 4 {
		t.Fatalf("default points: want 4, got %d", len(repo.seedGatewayInput.Points))
	}
	if repo.seedGatewayInput.Points[2].ControllableStatus != 1 {
		t.Fatalf("pump preset should be controllable")
	}
}

func TestBuildAdminControlCommandInputAcceptsStartAlias(t *testing.T) {
	input, label, err := buildAdminControlCommandInput(1001, 7, "start", "")
	if err != nil {
		t.Fatalf("start alias should be accepted: %v", err)
	}
	if label != "开启" {
		t.Fatalf("label: want 开启, got %q", label)
	}
	if input.CommandType != domain.CommandTypeRelayOpen || input.CommandHex != "F0" {
		t.Fatalf("start should map to relay open F0, got type=%d hex=%q", input.CommandType, input.CommandHex)
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

func TestHandleOperationLogsLegacyFormatWithoutPagination(t *testing.T) {
	repo := &mockAdminRepo{
		operationLogs: []domain.OperationRecord{
			{
				OpID:         9001,
				OperatorName: "admin",
				BindUserName: "农户A",
				GatewayName:  "GW-1",
				OpContent:    "bind gateway",
				OperatedAtMS: time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("GET", "/api/v1/admin/operation-logs", nil)
	rec := httptest.NewRecorder()

	handler.handleOperationLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("operation logs without pagination: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.operationLogQuery.Paginated {
		t.Fatalf("operation logs pagination should be false when page/size are absent")
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	items, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("legacy operation logs response should be array, got %T", resp["data"])
	}
	if len(items) != 1 {
		t.Fatalf("legacy operation logs item count: want 1, got %d", len(items))
	}
}

func TestHandleOperationLogsPaginated(t *testing.T) {
	repo := &mockAdminRepo{
		operationLogs: []domain.OperationRecord{
			{OpID: 9001, OperatorName: "admin", OpContent: "bind"},
			{OpID: 9002, OperatorName: "admin", OpContent: "config"},
		},
		operationTotal: 20,
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("GET", "/api/v1/admin/operation-logs?page=2&size=2&start=2026-07-01T00%3A00%3A00Z&end=2026-07-30T23%3A59%3A59Z&op_type=3&keyword=admin", nil)
	rec := httptest.NewRecorder()

	handler.handleOperationLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("operation logs paginated: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !repo.operationLogQuery.Paginated {
		t.Fatalf("operation logs pagination should be enabled with page+size")
	}
	if repo.operationLogQuery.Page != 2 || repo.operationLogQuery.Size != 2 {
		t.Fatalf("operation log pagination params: want page=2 size=2, got page=%d size=%d", repo.operationLogQuery.Page, repo.operationLogQuery.Size)
	}
	if repo.operationLogQuery.Start == nil || *repo.operationLogQuery.Start != "2026-07-01T00:00:00Z" {
		t.Fatalf("operation log start filter not passed, got %#v", repo.operationLogQuery.Start)
	}
	if repo.operationLogQuery.End == nil || *repo.operationLogQuery.End != "2026-07-30T23:59:59Z" {
		t.Fatalf("operation log end filter not passed, got %#v", repo.operationLogQuery.End)
	}
	if repo.operationLogQuery.OpType == nil || *repo.operationLogQuery.OpType != 3 {
		t.Fatalf("operation log op_type filter not passed: %#v", repo.operationLogQuery.OpType)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	wrapper, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("paginated operation logs should return object, got %T", resp["data"])
	}
	if got := int(wrapper["total"].(float64)); got != 20 {
		t.Fatalf("operation logs total: want 20, got %d", got)
	}
	if got := int(wrapper["page"].(float64)); got != 2 {
		t.Fatalf("operation logs page: want 2, got %d", got)
	}
	if got := int(wrapper["size"].(float64)); got != 2 {
		t.Fatalf("operation logs size: want 2, got %d", got)
	}
	items, ok := wrapper["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("operation logs items: want 2, got %d", len(items))
	}
}

func TestHandleControlLogsLegacyFormatWithoutPagination(t *testing.T) {
	commandHex := hex.EncodeToString(protocol.AppendCRC16Modbus([]byte{
		0x12, 0x34, 0x03, 0x10, 0x20, 0x30, 0xC2,
	}))
	repo := &mockAdminRepo{
		controlLogs: []domain.ControlLogSummary{
			{CommandID: 8001, GatewayID: 5001, BindUserName: "admin", CmdStatusLabel: "succeeded", CommandHex: commandHex},
		},
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("GET", "/api/v1/admin/control-logs", nil)
	rec := httptest.NewRecorder()

	handler.handleControlLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("control logs without pagination: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.controlLogQuery.Paginated {
		t.Fatalf("control logs pagination should be false when page/size are absent")
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	items, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("legacy control logs response should be array, got %T", resp["data"])
	}
	if len(items) != 1 {
		t.Fatalf("legacy control logs item count: want 1, got %d", len(items))
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("control log item should be object, got %T", items[0])
	}
	summary, ok := item["protocol_summary"].(map[string]any)
	if !ok || summary["kind"] != "mcu1_downlink_manual_control" {
		t.Fatalf("protocol summary missing: %#v", item["protocol_summary"])
	}
}

func TestHandleControlLogsPaginated(t *testing.T) {
	repo := &mockAdminRepo{
		controlLogs: []domain.ControlLogSummary{
			{CommandID: 8001, GatewayID: 5001},
			{CommandID: 8002, GatewayID: 5002},
		},
		controlTotal: 15,
	}
	handler := &adminHandler{repo: repo}

	req := httptest.NewRequest("GET", "/api/v1/admin/control-logs?page=1&size=2&cmd_status=4&source=1&keyword=8001", nil)
	rec := httptest.NewRecorder()

	handler.handleControlLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("control logs paginated: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !repo.controlLogQuery.Paginated {
		t.Fatalf("control logs pagination should be enabled with page+size")
	}
	if repo.controlLogQuery.Page != 1 || repo.controlLogQuery.Size != 2 {
		t.Fatalf("control log pagination params: want page=1 size=2, got page=%d size=%d", repo.controlLogQuery.Page, repo.controlLogQuery.Size)
	}
	if repo.controlLogQuery.CmdStatus == nil || *repo.controlLogQuery.CmdStatus != 4 {
		t.Fatalf("control log cmd_status filter not passed: %#v", repo.controlLogQuery.CmdStatus)
	}
	if repo.controlLogQuery.Source == nil || *repo.controlLogQuery.Source != 1 {
		t.Fatalf("control log source filter not passed: %#v", repo.controlLogQuery.Source)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	wrapper, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("paginated control logs should return object, got %T", resp["data"])
	}
	if got := int(wrapper["total"].(float64)); got != 15 {
		t.Fatalf("control logs total: want 15, got %d", got)
	}
	if got := int(wrapper["page"].(float64)); got != 1 {
		t.Fatalf("control logs page: want 1, got %d", got)
	}
	if got := int(wrapper["size"].(float64)); got != 2 {
		t.Fatalf("control logs size: want 2, got %d", got)
	}
	items, ok := wrapper["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("control logs items: want 2, got %d", len(items))
	}
}

func TestParseControlLogQueryEmptyEndMeansOpenEnded(t *testing.T) {
	params := parseControlLogQuery(map[string][]string{
		"end": {""},
	})
	if params.End != nil {
		t.Fatalf("empty end should be open-ended, got %q", *params.End)
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

func TestPayloadEventListIncludesProtocolAndCollectorIdentity(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{0x12, 0x34, 0x03, 0x11, 0x22, 0x33, 0xC0})
	repo := &mockAdminRepo{
		payloadEvents: []domain.PayloadEventListItem{{
			RawEventID: 9002,
			PayloadHex: hex.EncodeToString(packet),
		}},
		payloadEventTotal: 1,
	}
	handler := &adminHandler{repo: repo}
	req := httptest.NewRequest("GET", "/api/v1/admin/payload-events?page=1&size=20", nil)
	rec := httptest.NewRecorder()

	handler.handlePayloadEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("payload list: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), `"protocol_label":"MCU1 故障状态上报"`) {
		t.Fatalf("protocol_label missing: body=%s", rec.Body.String())
	}
	if !contains(rec.Body.String(), `"device_identity":"0x1234"`) {
		t.Fatalf("device_identity missing: body=%s", rec.Body.String())
	}
	if contains(rec.Body.String(), `"payload_hex"`) {
		t.Fatalf("payload_hex must stay out of list response: body=%s", rec.Body.String())
	}
}

func TestPayloadEventListFiltersTCPRawSource(t *testing.T) {
	repo := &mockAdminRepo{}
	handler := &adminHandler{repo: repo}
	req := httptest.NewRequest("GET", "/api/v1/admin/payload-events?source_type=1", nil)
	rec := httptest.NewRecorder()

	handler.handlePayloadEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("payload list: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.payloadEventQuery.SourceType == nil || *repo.payloadEventQuery.SourceType != uint8(domain.SourceTypeGateway) {
		t.Fatalf("source_type query = %+v, want gateway", repo.payloadEventQuery.SourceType)
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
