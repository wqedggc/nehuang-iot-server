package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/domain"
)

const testSecret = "nehuang-test-secret-key-123456"

// helperSign creates a signed JWT for testing.
func helperSign(userID uint64, userType domain.UserType, role domain.Role, aud string) string {
	now := time.Now()
	claims := auth.Claims{
		UserID:   userID,
		UserType: userType,
		Role:     role,
		Aud:      aud,
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(24 * time.Hour).UnixMilli(),
	}
	token, _ := auth.Sign(claims, []byte(testSecret))
	return token
}

// helperExpiredSign creates an expired JWT for testing.
func helperExpiredSign(userID uint64, userType domain.UserType, role domain.Role, aud string) string {
	claims := auth.Claims{
		UserID:   userID,
		UserType: userType,
		Role:     role,
		Aud:      aud,
		IssuedAt: time.Now().Add(-48 * time.Hour).UnixMilli(),
		ExpireAt: time.Now().Add(-24 * time.Hour).UnixMilli(),
	}
	token, _ := auth.Sign(claims, []byte(testSecret))
	return token
}

// helperNext is a simple http.Handler that writes 200 OK.
func helperNext() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// ─── Scenario 5: Middleware aud check ─────────────────────────────────────────

func TestAuthMiddleware_AdminToken_AccessAdminEndpoint(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	token := helperSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "admin")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("admin token accessing /api/v1/admin/*: want 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AdminToken_AccessMiniEndpoint_Returns403(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/mini/devices", nil)
	token := helperSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "admin")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("admin token accessing /api/v1/mini/*: want 403, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MiniToken_AccessMiniEndpoint(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/mini/devices", nil)
	token := helperSign(3, domain.UserTypeFarmer, domain.RoleFarmer, "mini")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("mini token accessing /api/v1/mini/*: want 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MiniToken_AccessAdminEndpoint_Returns403(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	token := helperSign(3, domain.UserTypeFarmer, domain.RoleFarmer, "mini")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("mini token accessing /api/v1/admin/*: want 403, got %d", rec.Code)
	}
}

// ─── Exact path matching (no trailing slash) ──────────────────────────────────

func TestAuthMiddleware_AdminToken_AccessAdminRootPath(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin", nil)
	token := helperSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "admin")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("admin token accessing /api/v1/admin (no trailing slash): want 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MiniToken_AccessMiniRootPath(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/mini", nil)
	token := helperSign(3, domain.UserTypeFarmer, domain.RoleFarmer, "mini")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("mini token accessing /api/v1/mini (no trailing slash): want 200, got %d", rec.Code)
	}
}

// ─── Non-admin/mini paths (no aud enforcement) ────────────────────────────────

func TestAuthMiddleware_AdminToken_AccessGatewayEndpoint(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/gateways", nil)
	token := helperSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "admin")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// /api/v1/gateways is not admin/ or mini/ — no aud enforcement
	if rec.Code != http.StatusOK {
		t.Errorf("admin token accessing /api/v1/gateways: want 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MiniToken_AccessGatewayEndpoint(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/gateways", nil)
	token := helperSign(3, domain.UserTypeFarmer, domain.RoleFarmer, "mini")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("mini token accessing /api/v1/gateways: want 200, got %d", rec.Code)
	}
}

// ─── Missing / malformed Authorization header ─────────────────────────────────

func TestAuthMiddleware_NoAuthHeader_Returns401(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no auth header: want 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MalformedAuthHeader_Returns401(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "NotBearer token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("malformed auth header: want 401, got %d", rec.Code)
	}
}

// ─── Expired token ────────────────────────────────────────────────────────────

func TestAuthMiddleware_ExpiredToken_Returns401(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	token := helperExpiredSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "admin")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expired token: want 401, got %d", rec.Code)
	}
}

// ─── Invalid token (wrong secret) ─────────────────────────────────────────────

func TestAuthMiddleware_InvalidToken_WrongSecret_Returns401(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	// Sign with a different secret
	claims := auth.Claims{
		UserID:   1,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleSuperAdmin,
		Aud:      "admin",
		IssuedAt: time.Now().UnixMilli(),
		ExpireAt: time.Now().Add(24 * time.Hour).UnixMilli(),
	}
	token, _ := auth.Sign(claims, []byte("wrong-secret-key"))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token with wrong secret: want 401, got %d", rec.Code)
	}
}

// ─── Token with empty aud ─────────────────────────────────────────────────────

func TestAuthMiddleware_EmptyAud_AccessAdminEndpoint_Returns403(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	token := helperSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("token with empty aud accessing admin endpoint: want 403, got %d", rec.Code)
	}
}

// ─── Claims injected into context ─────────────────────────────────────────────

func TestAuthMiddleware_ClaimsInContext(t *testing.T) {
	var capturedClaims auth.Claims
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFromContext(r.Context())
		if !ok {
			t.Error("ClaimsFromContext returned false")
			return
		}
		capturedClaims = c
		w.WriteHeader(http.StatusOK)
	})

	handler := authMiddleware([]byte(testSecret), next)
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	token := helperSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "admin")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedClaims.UserID != 1 {
		t.Errorf("UserID: want 1, got %d", capturedClaims.UserID)
	}
	if capturedClaims.Aud != "admin" {
		t.Errorf("Aud: want 'admin', got '%s'", capturedClaims.Aud)
	}
}

// ─── Specific error codes ─────────────────────────────────────────────────────

func TestAuthMiddleware_MiniTokenAccessAdmin_ReturnsCode4034(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	token := helperSign(3, domain.UserTypeFarmer, domain.RoleFarmer, "mini")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
	// The body should contain code 4034
	body := rec.Body.String()
	if !contains(body, "4034") {
		t.Errorf("response body should contain code 4034, got: %s", body)
	}
}

func TestAuthMiddleware_AdminTokenAccessMini_ReturnsCode4035(t *testing.T) {
	handler := authMiddleware([]byte(testSecret), helperNext())
	req := httptest.NewRequest("GET", "/api/v1/mini/devices", nil)
	token := helperSign(1, domain.UserTypeAdmin, domain.RoleSuperAdmin, "admin")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !contains(body, "4035") {
		t.Errorf("response body should contain code 4035, got: %s", body)
	}
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
