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
	userByUsername    *domain.User
	userByUsernameErr error
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
	return nil, nil
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
