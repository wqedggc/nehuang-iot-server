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

// ─── Mock repository for mini handler tests ───────────────────────────────────

type mockMiniRepo struct {
	userByOpenID    *domain.User
	userByOpenIDErr error
}

func (m *mockMiniRepo) GetUserByOpenID(ctx context.Context, openid string) (*domain.User, error) {
	return m.userByOpenID, m.userByOpenIDErr
}
func (m *mockMiniRepo) CreateUser(ctx context.Context, u *domain.User) error { return nil }
func (m *mockMiniRepo) UpdateLastLogin(ctx context.Context, userID uint64, t time.Time) error {
	return nil
}
func (m *mockMiniRepo) GetUserByID(ctx context.Context, userID uint64) (*domain.User, error) {
	return nil, nil
}
func (m *mockMiniRepo) ListMiniDevices(ctx context.Context, userID uint64) ([]domain.MiniDevice, error) {
	return nil, nil
}
func (m *mockMiniRepo) GetMiniDeviceDetail(ctx context.Context, pointID uint64) (*domain.MiniDeviceDetail, error) {
	return nil, nil
}
func (m *mockMiniRepo) GetMiniDeviceHistory(ctx context.Context, pointID uint64, rangeType string) ([]domain.MiniHistoryPoint, error) {
	return nil, nil
}
func (m *mockMiniRepo) CheckBindRelation(ctx context.Context, userID uint64, pointID uint64) (bool, error) {
	return false, nil
}
func (m *mockMiniRepo) UpdateMiniProfile(ctx context.Context, userID uint64, updates map[string]string) error {
	return nil
}

// ─── Scenario 7: Mini login JWT contains aud="mini" ──────────────────────────

func TestMiniLogin_JWTContainsAudMini(t *testing.T) {
	displayName := "张三农场"
	openID := "mock_user_001"
	repo := &mockMiniRepo{
		userByOpenID: &domain.User{
			UserID:       3,
			UserType:     domain.UserTypeFarmer,
			Role:         domain.RoleFarmer,
			WechatOpenID: &openID,
			DisplayName:  &displayName,
			Status:       domain.UserStatusActive,
		},
	}

	jwtCfg := config.JWTConfig{
		Secret:         testSecret,
		AdminTokenTTL:  24 * time.Hour,
		FarmerTokenTTL: 72 * time.Hour,
	}

	handler := &miniHandler{repo: repo, jwtCfg: jwtCfg}

	body := `{"mock_openid":"mock_user_001","mock_nickname":"张三农场"}`
	req := httptest.NewRequest("POST", "/api/v1/mini/mock-login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.mockLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("mock-login: want 200, got %d, body=%s", rec.Code, rec.Body.String())
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
		t.Fatalf("failed to parse mini token: %v", err)
	}
	if claims.Aud != "mini" {
		t.Errorf("mini token aud: want 'mini', got '%s'", claims.Aud)
	}
}

// ─── Test that mini login with new user also has aud="mini" ──────────────────

func TestMiniLogin_NewUser_JWTContainsAudMini(t *testing.T) {
	// User does not exist — the handler should create one
	repo := &mockMiniRepo{
		userByOpenID: nil, // user not found → auto-create
	}

	jwtCfg := config.JWTConfig{
		Secret:         testSecret,
		AdminTokenTTL:  24 * time.Hour,
		FarmerTokenTTL: 72 * time.Hour,
	}

	handler := &miniHandler{repo: repo, jwtCfg: jwtCfg}

	body := `{"mock_openid":"new_user_999","mock_nickname":"新农户"}`
	req := httptest.NewRequest("POST", "/api/v1/mini/mock-login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.mockLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("mock-login new user: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	tokenStr := data["token"].(string)

	claims, err := auth.Parse(tokenStr, []byte(testSecret))
	if err != nil {
		t.Fatalf("failed to parse new user token: %v", err)
	}
	if claims.Aud != "mini" {
		t.Errorf("new user mini token aud: want 'mini', got '%s'", claims.Aud)
	}
}
