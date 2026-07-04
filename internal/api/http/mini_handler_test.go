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
	createdUser     *domain.User
}

func (m *mockMiniRepo) GetUserByOpenID(ctx context.Context, openid string) (*domain.User, error) {
	return m.userByOpenID, m.userByOpenIDErr
}
func (m *mockMiniRepo) CreateUser(ctx context.Context, u *domain.User) error {
	if u.UserID == 0 {
		u.UserID = 99
	}
	m.createdUser = u
	m.userByOpenID = u
	return nil
}
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
func (m *mockMiniRepo) CreateControlCommandForPoint(ctx context.Context, input domain.CreateControlCommandInput) (*domain.ControlCommand, error) {
	return &domain.ControlCommand{
		CommandID:     123,
		PointID:       &input.PointID,
		CommandSource: input.CommandSource,
		CommandType:   input.CommandType,
		CommandHex:    input.CommandHex,
		CommandStatus: domain.CommandStatusCreated,
	}, nil
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

	handler := &miniHandler{
		repo:      repo,
		jwtCfg:    jwtCfg,
		wechatCfg: config.WechatConfig{EnableMockLogin: true},
	}

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

func TestBuildMiniControlCommandInput(t *testing.T) {
	tests := []struct {
		action  string
		wantHex string
		wantTyp domain.CommandType
	}{
		{"start", "F0", domain.CommandTypeRelayOpen},
		{"close", "0F", domain.CommandTypeRelayClose},
		{"stop", "00", domain.CommandTypeRelayStop},
	}
	for _, tt := range tests {
		input, _, err := buildMiniControlCommandInput(99, 3, tt.action)
		if err != nil {
			t.Fatalf("buildMiniControlCommandInput(%q): %v", tt.action, err)
		}
		if input.CommandHex != tt.wantHex || input.CommandType != tt.wantTyp {
			t.Fatalf("action %q mapped to hex=%s type=%d", tt.action, input.CommandHex, input.CommandType)
		}
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

	handler := &miniHandler{
		repo:      repo,
		jwtCfg:    jwtCfg,
		wechatCfg: config.WechatConfig{EnableMockLogin: true},
	}

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
	if repo.createdUser == nil {
		t.Fatal("new user login should create a farmer user")
	}
	if repo.createdUser.UserType != domain.UserTypeFarmer || repo.createdUser.Role != domain.RoleFarmer {
		t.Fatalf("created user should be farmer, got user_type=%d role=%d", repo.createdUser.UserType, repo.createdUser.Role)
	}
}

func TestMiniWechatLogin_JWTContainsAudMini(t *testing.T) {
	displayName := "微信农户"
	openID := "wx_openid_001"
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

	wechatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("appid"); got != "test-appid" {
			t.Errorf("appid: want test-appid, got %q", got)
		}
		if got := r.URL.Query().Get("secret"); got != "test-secret" {
			t.Errorf("secret: want test-secret, got %q", got)
		}
		if got := r.URL.Query().Get("js_code"); got != "wx-code-123" {
			t.Errorf("js_code: want wx-code-123, got %q", got)
		}
		if got := r.URL.Query().Get("grant_type"); got != "authorization_code" {
			t.Errorf("grant_type: want authorization_code, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openid":"wx_openid_001","session_key":"session","unionid":"union_001"}`))
	}))
	defer wechatServer.Close()

	oldEndpoint := wechatJscode2SessionURL
	wechatJscode2SessionURL = wechatServer.URL
	defer func() { wechatJscode2SessionURL = oldEndpoint }()

	handler := &miniHandler{
		repo:   repo,
		jwtCfg: config.JWTConfig{Secret: testSecret, AdminTokenTTL: 24 * time.Hour, FarmerTokenTTL: 72 * time.Hour},
		wechatCfg: config.WechatConfig{
			AppID:  "test-appid",
			Secret: "test-secret",
		},
	}

	body := `{"code":"wx-code-123","nickname":"微信农户","avatar_url":"https://example.com/avatar.png"}`
	req := httptest.NewRequest("POST", "/api/v1/mini/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("mini login: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	data := resp["data"].(map[string]any)
	tokenStr := data["token"].(string)
	userID := data["user_id"].(string)
	if userID != "3" {
		t.Fatalf("user_id should stay string '3', got %#v", data["user_id"])
	}

	claims, err := auth.Parse(tokenStr, []byte(testSecret))
	if err != nil {
		t.Fatalf("failed to parse mini token: %v", err)
	}
	if claims.Aud != "mini" {
		t.Errorf("mini token aud: want 'mini', got '%s'", claims.Aud)
	}
}

func TestMiniMockLogin_DisabledByDefault(t *testing.T) {
	handler := &miniHandler{
		repo:   &mockMiniRepo{},
		jwtCfg: config.JWTConfig{Secret: testSecret, AdminTokenTTL: 24 * time.Hour, FarmerTokenTTL: 72 * time.Hour},
	}

	body := `{"mock_openid":"mock_user_001","mock_nickname":"张三农场"}`
	req := httptest.NewRequest("POST", "/api/v1/mini/mock-login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.mockLogin(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("disabled mock-login: want 404, got %d, body=%s", rec.Code, rec.Body.String())
	}
}
