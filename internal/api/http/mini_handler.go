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

// miniRepository defines the data access methods required by mini-program handlers.
type miniRepository interface {
	GetUserByOpenID(ctx context.Context, openid string) (*domain.User, error)
	CreateUser(ctx context.Context, u *domain.User) error
	UpdateLastLogin(ctx context.Context, userID uint64, t time.Time) error
}

// miniHandler handles all /api/v1/mini/* endpoints.
type miniHandler struct {
	repo   miniRepository
	jwtCfg config.JWTConfig
}

// mockLoginRequest is the JSON body for POST /api/v1/mini/mock-login.
type mockLoginRequest struct {
	MockOpenID   string `json:"mock_openid"`
	MockNickname string `json:"mock_nickname"`
}

// mockLogin handles POST /api/v1/mini/mock-login.
// This endpoint is for development/testing only and should be disabled in production.
func (h *miniHandler) mockLogin(w http.ResponseWriter, r *http.Request) {
	var req mockLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.MockOpenID == "" {
		writeCode(w, http.StatusBadRequest, 400, "mock_openid is required")
		return
	}

	user, err := h.repo.GetUserByOpenID(r.Context(), req.MockOpenID)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "internal error")
		return
	}

	isNewUser := false
	if user == nil {
		// User does not exist yet — create a new farmer account.
		isNewUser = true
		displayName := req.MockNickname
		openID := req.MockOpenID
		newUser := &domain.User{
			UserType:     domain.UserTypeFarmer,
			Role:         domain.RoleFarmer,
			WechatOpenID: &openID,
			DisplayName:  &displayName,
			Status:       domain.UserStatusActive,
		}
		if err := h.repo.CreateUser(r.Context(), newUser); err != nil {
			writeCode(w, http.StatusInternalServerError, 500, "failed to create user")
			return
		}
		user = newUser
	}

	now := time.Now()
	claims := auth.Claims{
		UserID:   user.UserID,
		UserType: user.UserType,
		Role:     user.Role,
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

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"token":       token,
			"user_id":     user.UserID,
			"is_new_user": isNewUser,
		},
	})
}
