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
	userByUsername       *domain.User
	userByUsernameErr    error
	devicePointByID      *domain.DevicePoint
	updateDevicePointErr error
	updatedPointID       uint64
	updatedPointInput    domain.DevicePointUpdateInput
}

func (m *mockAdminRepo) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	return m.userByUsername, m.userByUsernameErr
}
func (m *mockAdminRepo) GetUserByID(ctx context.Context, userID uint64) (*domain.User, error) {
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
func (m *mockAdminRepo) CreateBinding(ctx context.Context, b *domain.BindRelation) error { return nil }
func (m *mockAdminRepo) UnbindRelation(ctx context.Context, bindID uint64) error         { return nil }
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
func (m *mockAdminRepo) FindActiveBindingByUserAndPoint(ctx context.Context, userID uint64, pointID uint64) (*domain.BindRelation, error) {
	return nil, nil
}
func (m *mockAdminRepo) FindActiveBindingByPoint(ctx context.Context, pointID uint64) (*domain.BindRelation, error) {
	return nil, nil
}
func (m *mockAdminRepo) SearchDevicePoints(ctx context.Context, keyword string, bindStatus string) ([]domain.DevicePointSearchResult, error) {
	return nil, nil
}
func (m *mockAdminRepo) UpdateDevicePointConfig(ctx context.Context, pointID uint64, input domain.DevicePointUpdateInput) error {
	m.updatedPointID = pointID
	m.updatedPointInput = input
	return m.updateDevicePointErr
}
func (m *mockAdminRepo) ListPayloadEvents(ctx context.Context, params domain.PayloadEventQuery) ([]domain.PayloadEventListItem, int, error) {
	return nil, 0, nil
}
func (m *mockAdminRepo) GetPayloadEventDetail(ctx context.Context, rawEventID uint64) (*domain.PayloadEventDetail, error) {
	return nil, nil
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
