package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nehuang-iot-server/internal/config"
	"nehuang-iot-server/internal/domain"
)

func TestMiniControlCommandStatusReturnsAuthorizedCommand(t *testing.T) {
	pointID := uint64(9223372036854775807)
	finishedAt := time.Date(2026, 8, 22, 12, 34, 56, 789000000, time.UTC)
	errorMessage := "设备执行失败"
	repo := &mockMiniRepo{
		bound: true,
		controlCommand: &domain.ControlCommand{
			CommandID:       18446744073709551610,
			PointID:         &pointID,
			CommandHex:      "must-not-leak",
			CommandStatus:   domain.CommandStatusFailed,
			FinishedAtMS:    &finishedAt,
			GatewayResponse: stringPtr("must-not-leak"),
			ErrorMessage:    &errorMessage,
		},
	}
	handler := &miniHandler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini/control-commands/18446744073709551610", nil)
	req = injectClaims(req, 88, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleControlCommandStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status query: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := response.Data["command_id"]; got != "18446744073709551610" {
		t.Fatalf("command_id must remain a string, got %#v", got)
	}
	if got := response.Data["status"]; got != "failed" {
		t.Fatalf("status: want failed, got %#v", got)
	}
	if got := response.Data["finished_at_ms"]; got != "2026-08-22T12:34:56.789Z" {
		t.Fatalf("finished_at_ms: got %#v", got)
	}
	if got := response.Data["error_message"]; got != errorMessage {
		t.Fatalf("error_message: got %#v", got)
	}
	if strings.Contains(rec.Body.String(), "must-not-leak") {
		t.Fatalf("response leaked internal command data: %s", rec.Body.String())
	}
	if repo.queriedCommandID != 18446744073709551610 || repo.checkedUserID != 88 || repo.checkedPointID != pointID {
		t.Fatalf("authorization inputs: command=%d user=%d point=%d", repo.queriedCommandID, repo.checkedUserID, repo.checkedPointID)
	}
}

func TestMiniControlCommandStatusRejectsUnboundDevice(t *testing.T) {
	pointID := uint64(99)
	repo := &mockMiniRepo{
		bound: false,
		controlCommand: &domain.ControlCommand{
			CommandID:     123,
			PointID:       &pointID,
			CommandStatus: domain.CommandStatusSucceeded,
		},
	}
	handler := &miniHandler{repo: repo}
	req := injectClaims(httptest.NewRequest(http.MethodGet, "/api/v1/mini/control-commands/123", nil), 7, domain.RoleFarmer, "mini")
	rec := httptest.NewRecorder()

	handler.handleControlCommandStatus(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("unbound command: want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "succeeded") {
		t.Fatalf("forbidden response leaked command status: %s", rec.Body.String())
	}
}

func TestMiniControlCommandStatusHandlesInvalidAndMissingCommands(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		command    *domain.ControlCommand
		wantStatus int
	}{
		{name: "invalid id", path: "/api/v1/mini/control-commands/not-a-number", wantStatus: http.StatusBadRequest},
		{name: "missing", path: "/api/v1/mini/control-commands/123", wantStatus: http.StatusNotFound},
		{name: "missing target point", path: "/api/v1/mini/control-commands/123", command: &domain.ControlCommand{CommandID: 123}, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockMiniRepo{controlCommand: tt.command}
			handler := &miniHandler{repo: repo}
			req := injectClaims(httptest.NewRequest(http.MethodGet, tt.path, nil), 7, domain.RoleFarmer, "mini")
			rec := httptest.NewRecorder()

			handler.handleControlCommandStatus(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("want %d, got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

type miniControlStatusRouteRepo struct {
	repository
	command *domain.ControlCommand
}

func (r *miniControlStatusRouteRepo) GetControlCommandByID(ctx context.Context, commandID uint64) (*domain.ControlCommand, error) {
	return r.command, nil
}

func (r *miniControlStatusRouteRepo) CheckBindRelation(ctx context.Context, userID uint64, pointID uint64) (bool, error) {
	return true, nil
}

func TestMiniControlCommandStatusRoute(t *testing.T) {
	pointID := uint64(99)
	repo := &miniControlStatusRouteRepo{command: &domain.ControlCommand{
		CommandID:     123,
		PointID:       &pointID,
		CommandStatus: domain.CommandStatusSent,
	}}
	server := NewServer(repo, config.Config{JWT: config.JWTConfig{Secret: testSecret}})
	token := helperSign(7, domain.UserTypeFarmer, domain.RoleFarmer, "mini")

	for _, tt := range []struct {
		method     string
		wantStatus int
	}{
		{method: http.MethodGet, wantStatus: http.StatusOK},
		{method: http.MethodPost, wantStatus: http.StatusMethodNotAllowed},
	} {
		req := httptest.NewRequest(tt.method, "/api/v1/mini/control-commands/123", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		server.Routes().ServeHTTP(rec, req)

		if rec.Code != tt.wantStatus {
			t.Fatalf("%s route: want %d, got %d body=%s", tt.method, tt.wantStatus, rec.Code, rec.Body.String())
		}
	}
}

func TestMiniControlCommandStatusRouteRequiresMiniAuth(t *testing.T) {
	server := NewServer(&miniControlStatusRouteRepo{}, config.Config{JWT: config.JWTConfig{Secret: testSecret}})

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized},
		{name: "admin token", token: helperSign(1, domain.UserTypeAdmin, domain.RoleAdmin, "admin"), wantStatus: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/mini/control-commands/123", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()

			server.Routes().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("want %d, got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func stringPtr(value string) *string {
	return &value
}
