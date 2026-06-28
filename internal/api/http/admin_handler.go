package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
)

// adminRepository defines the data access methods required by admin handlers.
type adminRepository interface {
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
	GetUserByID(ctx context.Context, userID uint64) (*domain.User, error)
	UpdateLastLogin(ctx context.Context, userID uint64, t time.Time) error
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
	Token       string  `json:"token"`
	UserID      uint64  `json:"user_id"`
	Role        uint8   `json:"role"`
	DisplayName string  `json:"display_name"`
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
			UserID:      user.UserID,
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
			"user_id":         user.UserID,
			"username":        username,
			"role":            uint8(user.Role),
			"display_name":    displayName,
			"last_login_at_ms": lastLoginStr,
		},
	})
}
