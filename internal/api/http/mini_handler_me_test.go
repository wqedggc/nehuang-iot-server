package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
)

// ─── Extended mock repository ────────────────────────────────────────────────

// mockMiniRepoExt extends the mock to support handleMe and handleUpdateMe.
type mockMiniRepoExt struct {
	// GET /me data
	userByID       *domain.User
	userByIDErr    error
	miniDevices    []domain.MiniDevice
	miniDevicesErr error

	// PATCH /me data
	updateProfileCalled  bool
	updateProfileUserID  uint64
	updateProfileUpdates map[string]string
	updateProfileErr     error

	// Track state for verification
	mu sync.Mutex
}

func (m *mockMiniRepoExt) GetUserByOpenID(ctx context.Context, openid string) (*domain.User, error) {
	return nil, nil
}
func (m *mockMiniRepoExt) CreateUser(ctx context.Context, u *domain.User) error { return nil }
func (m *mockMiniRepoExt) UpdateLastLogin(ctx context.Context, userID uint64, t time.Time) error {
	return nil
}
func (m *mockMiniRepoExt) GetUserByID(ctx context.Context, userID uint64) (*domain.User, error) {
	return m.userByID, m.userByIDErr
}
func (m *mockMiniRepoExt) UpdateMiniProfile(ctx context.Context, userID uint64, updates map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateProfileCalled = true
	m.updateProfileUserID = userID
	m.updateProfileUpdates = updates
	return m.updateProfileErr
}
func (m *mockMiniRepoExt) ListMiniDevices(ctx context.Context, userID uint64) ([]domain.MiniDevice, error) {
	return m.miniDevices, m.miniDevicesErr
}
func (m *mockMiniRepoExt) GetMiniDeviceDetail(ctx context.Context, pointID uint64) (*domain.MiniDeviceDetail, error) {
	return nil, nil
}
func (m *mockMiniRepoExt) GetMiniDeviceHistory(ctx context.Context, pointID uint64, rangeType string) ([]domain.MiniHistoryPoint, error) {
	return nil, nil
}
func (m *mockMiniRepoExt) CheckBindRelation(ctx context.Context, userID uint64, pointID uint64) (bool, error) {
	return false, nil
}
func (m *mockMiniRepoExt) CreateControlCommandForPoint(ctx context.Context, input domain.CreateControlCommandInput) (*domain.ControlCommand, error) {
	return &domain.ControlCommand{
		CommandID:     123,
		PointID:       &input.PointID,
		CommandSource: input.CommandSource,
		CommandType:   input.CommandType,
		CommandHex:    input.CommandHex,
		CommandStatus: domain.CommandStatusCreated,
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func makeMiniHandler(repo *mockMiniRepoExt) *miniHandler {
	return &miniHandler{
		repo: repo,
		jwtCfg: config.JWTConfig{
			Secret:         testSecret,
			AdminTokenTTL:  24 * time.Hour,
			FarmerTokenTTL: 72 * time.Hour,
		},
	}
}

// injectClaims creates a request with a valid farmer JWT injected via context.
func injectClaims(req *http.Request, userID uint64, role domain.Role, aud string) *http.Request {
	now := time.Now()
	claims := auth.Claims{
		UserID:   userID,
		UserType: domain.UserTypeFarmer,
		Role:     role,
		Aud:      aud,
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(24 * time.Hour).UnixMilli(),
	}
	ctx := context.WithValue(req.Context(), claimsKey, claims)
	return req.WithContext(ctx)
}

// parseResp unmarshals the response body into a generic map.
func parseResp(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to parse response: %v, body=%s", err, string(body))
	}
	return resp
}

// (strPtr is already defined in adapter_handler.go — no need to redeclare)

// ─── Scenario 1: GET /api/v1/mini/me 返回完整资料 ──────────────────────────────

func TestHandleMe_ReturnsFullProfile(t *testing.T) {
	displayName := "张三农场"
	mobile := "13800001111"
	address := "山东省潍坊市寿光市大棚区A-01"
	remark := "寿光蔬菜大棚种植户"

	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:      3,
			UserType:    domain.UserTypeFarmer,
			Role:        domain.RoleFarmer,
			DisplayName: &displayName,
			Mobile:      &mobile,
			Address:     &address,
			Remark:      &remark,
			Status:      domain.UserStatusActive,
		},
		miniDevices: []domain.MiniDevice{
			{PointID: 1}, {PointID: 11}, {PointID: 21}, {PointID: 101},
		},
	}

	handler := makeMiniHandler(repo)

	req := httptest.NewRequest("GET", "/api/v1/mini/me", nil)
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleMe: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	resp := parseResp(t, rec.Body.Bytes())
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing data field: %v", resp)
	}

	// Verify all fields present
	if data["user_id"] != "3" {
		t.Errorf("user_id: want '3', got %v", data["user_id"])
	}
	if data["display_name"] != displayName {
		t.Errorf("display_name: want %q, got %v", displayName, data["display_name"])
	}
	if data["mobile"] != mobile {
		t.Errorf("mobile: want %q, got %v", mobile, data["mobile"])
	}
	if data["address"] != address {
		t.Errorf("address: want %q, got %v", address, data["address"])
	}
	if data["remark"] != remark {
		t.Errorf("remark: want %q, got %v", remark, data["remark"])
	}
	role, ok := data["role"].(float64)
	if !ok || uint8(role) != uint8(domain.RoleFarmer) {
		t.Errorf("role: want %d, got %v", domain.RoleFarmer, data["role"])
	}
	bindingCount, ok := data["binding_count"].(float64)
	if !ok || int(bindingCount) != 4 {
		t.Errorf("binding_count: want 4, got %v", data["binding_count"])
	}
}

// ─── Scenario 1b: GET /me with nullable fields empty ─────────────────────────

func TestHandleMe_ReturnsEmptyStringsForNullFields(t *testing.T) {
	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:   4,
			UserType: domain.UserTypeFarmer,
			Role:     domain.RoleFarmer,
			// All nullable fields nil
			Status: domain.UserStatusActive,
		},
		miniDevices: nil, // no devices
	}

	handler := makeMiniHandler(repo)

	req := httptest.NewRequest("GET", "/api/v1/mini/me", nil)
	req = injectClaims(req, 4, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleMe: want 200, got %d", rec.Code)
	}

	resp := parseResp(t, rec.Body.Bytes())
	data := resp["data"].(map[string]interface{})

	if data["display_name"] != "" {
		t.Errorf("display_name: want '', got %v", data["display_name"])
	}
	if data["mobile"] != "" {
		t.Errorf("mobile: want '', got %v", data["mobile"])
	}
	if data["address"] != "" {
		t.Errorf("address: want '', got %v", data["address"])
	}
	if data["remark"] != "" {
		t.Errorf("remark: want '', got %v", data["remark"])
	}
	bindingCount, _ := data["binding_count"].(float64)
	if int(bindingCount) != 0 {
		t.Errorf("binding_count: want 0, got %v", bindingCount)
	}
}

// ─── Scenario 1c: GET /me without claims → 401 ───────────────────────────────

func TestHandleMe_NoClaims_Returns401(t *testing.T) {
	repo := &mockMiniRepoExt{}
	handler := makeMiniHandler(repo)

	req := httptest.NewRequest("GET", "/api/v1/mini/me", nil)
	// No claims injected
	rec := httptest.NewRecorder()

	handler.handleMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("handleMe without claims: want 401, got %d", rec.Code)
	}
}

// ─── Scenario 2: PATCH /api/v1/mini/me 更新成功 ────────────────────────────────

func TestHandleUpdateMe_Success(t *testing.T) {
	displayName := "张三农场"
	mobile := "13800001111"
	address := "山东省潍坊市"
	remark := "蔬菜种植"

	// After update, GetUserByID returns updated data
	updatedName := "张三农场更新"
	updatedMobile := "13900002222"
	updatedAddress := "山东省潍坊市寿光市新大棚"
	updatedRemark := "新备注"

	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:      3,
			UserType:    domain.UserTypeFarmer,
			Role:        domain.RoleFarmer,
			DisplayName: &updatedName,
			Mobile:      &updatedMobile,
			Address:     &updatedAddress,
			Remark:      &updatedRemark,
			Status:      domain.UserStatusActive,
		},
	}

	handler := makeMiniHandler(repo)

	body := `{"display_name":"张三农场更新","mobile":"13900002222","address":"山东省潍坊市寿光市新大棚","remark":"新备注"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleUpdateMe: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Verify UpdateMiniProfile was called with correct params
	if !repo.updateProfileCalled {
		t.Fatal("UpdateMiniProfile was not called")
	}
	if repo.updateProfileUserID != 3 {
		t.Errorf("UpdateMiniProfile userID: want 3, got %d", repo.updateProfileUserID)
	}
	if repo.updateProfileUpdates["display_name"] != "张三农场更新" {
		t.Errorf("display_name update: want '张三农场更新', got %q", repo.updateProfileUpdates["display_name"])
	}
	if repo.updateProfileUpdates["mobile"] != "13900002222" {
		t.Errorf("mobile update: want '13900002222', got %q", repo.updateProfileUpdates["mobile"])
	}
	if repo.updateProfileUpdates["address"] != "山东省潍坊市寿光市新大棚" {
		t.Errorf("address update: want '山东省潍坊市寿光市新大棚', got %q", repo.updateProfileUpdates["address"])
	}
	if repo.updateProfileUpdates["remark"] != "新备注" {
		t.Errorf("remark update: want '新备注', got %q", repo.updateProfileUpdates["remark"])
	}

	// Verify response contains updated data
	resp := parseResp(t, rec.Body.Bytes())
	data := resp["data"].(map[string]interface{})

	if data["display_name"] != updatedName {
		t.Errorf("response display_name: want %q, got %v", updatedName, data["display_name"])
	}
	if data["mobile"] != updatedMobile {
		t.Errorf("response mobile: want %q, got %v", updatedMobile, data["mobile"])
	}
	if data["address"] != updatedAddress {
		t.Errorf("response address: want %q, got %v", updatedAddress, data["address"])
	}
	if data["remark"] != updatedRemark {
		t.Errorf("response remark: want %q, got %v", updatedRemark, data["remark"])
	}

	_ = displayName
	_ = mobile
	_ = address
	_ = remark
}

// ─── Scenario 2b: PATCH with partial update (only some fields) ────────────────

func TestHandleUpdateMe_PartialUpdate(t *testing.T) {
	updatedName := "仅改名"

	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:      3,
			UserType:    domain.UserTypeFarmer,
			Role:        domain.RoleFarmer,
			DisplayName: &updatedName,
			Status:      domain.UserStatusActive,
		},
	}

	handler := makeMiniHandler(repo)

	body := `{"display_name":"仅改名"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleUpdateMe partial: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Should still pass empty strings for other fields to UpdateMiniProfile
	if repo.updateProfileUpdates["display_name"] != "仅改名" {
		t.Errorf("display_name: want '仅改名', got %q", repo.updateProfileUpdates["display_name"])
	}
	// Empty fields should be passed as "" — UpdateMiniProfile skips them
	if repo.updateProfileUpdates["mobile"] != "" {
		t.Errorf("mobile: want '', got %q", repo.updateProfileUpdates["mobile"])
	}
}

// ─── Scenario 3: admin token 调 PATCH /api/v1/mini/me 由中间件拦截 403 ────────
//
// NOTE: The handleUpdateMe handler itself does NOT check role/aud —
// that enforcement is done by authMiddleware + roleMiddleware in server.go.
// When the handler is called directly (bypassing middleware), it will process
// the request normally (returning 404 if user not found in mock repo).
// The 403 is enforced at the middleware level, tested in middleware_test.go.
//
// This test verifies the handler's behavior when admin claims are present
// but the middleware has already passed the request through (should not happen
// in production because middleware blocks it first).

func TestHandleUpdateMe_AdminToken_Returns404_UserNotFound(t *testing.T) {
	repo := &mockMiniRepoExt{
		// userByID returns nil (user not found) because admin user_id=1
		// is not in the mock repo for mini handler
	}
	handler := makeMiniHandler(repo)

	body := `{"display_name":"hack"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Inject admin claims (role=super_admin, aud="admin")
	req = injectClaims(req, 1, domain.RoleSuperAdmin, "admin")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	// Handler processes request normally (middleware would have blocked this).
	// The mock repo returns nil user, so the handler returns 404.
	if rec.Code != http.StatusNotFound {
		t.Errorf("admin token on mini PATCH (direct call): want 404 (user not found), got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// ─── Scenario 3b: admin token 调 GET /api/v1/mini/me 由中间件拦截 403 ─────────
//
// Same as above: the handler doesn't check role, middleware does.
// Direct handler call with admin claims processes normally.

func TestHandleMe_AdminToken_Returns404_UserNotFound(t *testing.T) {
	repo := &mockMiniRepoExt{
		// userByID returns nil — admin user_id=1 not found
	}
	handler := makeMiniHandler(repo)

	req := httptest.NewRequest("GET", "/api/v1/mini/me", nil)
	req = injectClaims(req, 1, domain.RoleSuperAdmin, "admin")
	rec := httptest.NewRecorder()

	handler.handleMe(rec, req)

	// Handler processes request normally — middleware blocks this in production.
	if rec.Code != http.StatusNotFound {
		t.Errorf("admin token on mini GET /me (direct call): want 404 (user not found), got %d", rec.Code)
	}
}

// ─── Scenario 3c: Integration test — admin token through middleware returns 403 ─
//
// This test verifies the FULL middleware chain (authMiddleware + roleMiddleware)
// correctly blocks admin tokens from accessing mini endpoints.

func TestMiddleware_AdminToken_AccessMiniMe_Returns403(t *testing.T) {
	// Build the middleware chain as server.go does for /api/v1/mini/me
	secret := []byte(testSecret)
	farmerRoles := roleMiddleware(domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleFarmer)

	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:      3,
			UserType:    domain.UserTypeFarmer,
			Role:        domain.RoleFarmer,
			DisplayName: strPtr("test"),
			Status:      domain.UserStatusActive,
		},
	}
	handler := makeMiniHandler(repo)

	// Build the same chain as server.go line 153-162
	miniMeHandler := authMiddleware(secret, farmerRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler.handleMe(w, r)
		} else if r.Method == http.MethodPatch {
			handler.handleUpdateMe(w, r)
		} else {
			writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	})))

	// Test GET with admin token
	req := httptest.NewRequest("GET", "/api/v1/mini/me", nil)
	token := helperSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "admin")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	miniMeHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("admin token GET /api/v1/mini/me through middleware: want 403, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Test PATCH with admin token
	req2 := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(`{"display_name":"hack"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()

	miniMeHandler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Errorf("admin token PATCH /api/v1/mini/me through middleware: want 403, got %d, body=%s", rec2.Code, rec2.Body.String())
	}
}

// ─── Scenario 4: 无 token 返回 401 ─────────────────────────────────────────────

func TestHandleUpdateMe_NoClaims_Returns401(t *testing.T) {
	repo := &mockMiniRepoExt{}
	handler := makeMiniHandler(repo)

	body := `{"display_name":"test"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No claims injected
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token on PATCH: want 401, got %d", rec.Code)
	}
}

// ─── Scenario 5: 请求体传 user_id/status/role 不生效（安全校验）───────────────

func TestHandleUpdateMe_ImmutableFieldsNotApplied(t *testing.T) {
	// After "update", GetUserByID returns the ORIGINAL role (not the one in the request body)
	originalName := "张三农场"
	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:      3,
			UserType:    domain.UserTypeFarmer,
			Role:        domain.RoleFarmer, // Still farmer, not admin
			DisplayName: &originalName,
			Status:      domain.UserStatusActive,
		},
	}

	handler := makeMiniHandler(repo)

	// Attempt to inject user_id, status, role in request body
	body := `{"display_name":"hack","user_id":1,"status":2,"role":1}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleUpdateMe: want 200, got %d", rec.Code)
	}

	// Verify that user_id, status, role were NOT passed to UpdateMiniProfile
	if repo.updateProfileUpdates["user_id"] != "" {
		t.Errorf("user_id should not be in updates, got %q", repo.updateProfileUpdates["user_id"])
	}
	if repo.updateProfileUpdates["status"] != "" {
		t.Errorf("status should not be in updates, got %q", repo.updateProfileUpdates["status"])
	}
	if repo.updateProfileUpdates["role"] != "" {
		t.Errorf("role should not be in updates, got %q", repo.updateProfileUpdates["role"])
	}

	// The response should reflect the ORIGINAL role (farmer), not the injected one
	resp := parseResp(t, rec.Body.Bytes())
	data := resp["data"].(map[string]interface{})
	role, _ := data["role"].(float64)
	if uint8(role) != uint8(domain.RoleFarmer) {
		t.Errorf("role in response: want %d, got %v — immutable field was modified!", domain.RoleFarmer, role)
	}
}

// ─── Scenario 6: display_name 为空时返回验证错误 ────────────────────────────────

func TestHandleUpdateMe_EmptyDisplayName_ReturnsValidationError(t *testing.T) {
	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:      3,
			UserType:    domain.UserTypeFarmer,
			Role:        domain.RoleFarmer,
			DisplayName: strPtr("张三农场"),
			Status:      domain.UserStatusActive,
		},
	}

	handler := makeMiniHandler(repo)

	body := `{"display_name":""}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty display_name: want 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "display_name") {
		t.Fatalf("empty display_name: response should mention display_name, body=%s", rec.Body.String())
	}
	if repo.updateProfileCalled {
		t.Fatal("UpdateMiniProfile should not have been called")
	}
}

func TestHandleUpdateMe_DisplayNameTooLong_ReturnsValidationError(t *testing.T) {
	repo := &mockMiniRepoExt{}
	handler := makeMiniHandler(repo)

	// 65 characters (over the 64 limit)
	longName := strings.Repeat("a", 65)
	body := `{"display_name":"` + longName + `"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("display_name > 64 chars: want 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateMe_MobileTooLong_ReturnsValidationError(t *testing.T) {
	repo := &mockMiniRepoExt{}
	handler := makeMiniHandler(repo)

	longMobile := strings.Repeat("1", 513)
	body := `{"mobile":"` + longMobile + `"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("mobile > 512 chars: want 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// ─── Scenario 6c: display_name 1 char is valid ────────────────────────────────

func TestHandleUpdateMe_DisplayNameOneChar_Success(t *testing.T) {
	updatedName := "张"
	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:      3,
			UserType:    domain.UserTypeFarmer,
			Role:        domain.RoleFarmer,
			DisplayName: &updatedName,
			Status:      domain.UserStatusActive,
		},
	}

	handler := makeMiniHandler(repo)

	body := `{"display_name":"张"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("display_name 1 char: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// ─── Scenario 6d: display_name 64 chars (boundary) is valid ───────────────────

func TestHandleUpdateMe_DisplayName64Chars_Success(t *testing.T) {
	repo := &mockMiniRepoExt{
		userByID: &domain.User{
			UserID:      3,
			UserType:    domain.UserTypeFarmer,
			Role:        domain.RoleFarmer,
			DisplayName: strPtr("test"),
			Status:      domain.UserStatusActive,
		},
	}

	handler := makeMiniHandler(repo)

	name64 := strings.Repeat("a", 64)
	body := `{"display_name":"` + name64 + `"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("display_name 64 chars: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// ─── Scenario: PATCH with invalid JSON body returns 400 ────────────────────────

func TestHandleUpdateMe_InvalidJSON_Returns400(t *testing.T) {
	repo := &mockMiniRepoExt{}
	handler := makeMiniHandler(repo)

	body := `{invalid json`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON: want 400, got %d", rec.Code)
	}
}

// ─── Scenario: handleMe with repo error returns 500 ───────────────────────────

func TestHandleMe_RepoError_Returns500(t *testing.T) {
	repo := &mockMiniRepoExt{
		userByIDErr: context.DeadlineExceeded,
	}
	handler := makeMiniHandler(repo)

	req := httptest.NewRequest("GET", "/api/v1/mini/me", nil)
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleMe(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("repo error: want 500, got %d", rec.Code)
	}
}

// ─── Scenario: handleUpdateMe with repo error returns 500 ──────────────────────

func TestHandleUpdateMe_RepoError_Returns500(t *testing.T) {
	repo := &mockMiniRepoExt{
		updateProfileErr: context.DeadlineExceeded,
	}
	handler := makeMiniHandler(repo)

	body := `{"display_name":"test"}`
	req := httptest.NewRequest("PATCH", "/api/v1/mini/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectClaims(req, 3, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleUpdateMe(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("update repo error: want 500, got %d", rec.Code)
	}
}
