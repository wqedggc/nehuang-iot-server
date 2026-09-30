package http

import (
	"context"
	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockRawDebugAdminRepo struct {
	mockAdminRepo
	calls int
	input domain.RawDebugCommandInput
}

func (r *mockRawDebugAdminRepo) CreateRawDebugCommand(_ context.Context, in domain.RawDebugCommandInput) (*domain.ControlCommand, error) {
	r.calls++
	r.input = in
	return &domain.ControlCommand{CommandID: 9, GatewayID: in.GatewayID, CommandHex: in.CommandHex, CommandStatus: domain.CommandStatusCreated}, nil
}
func TestRawDebugRequiresAdminConfirmationAndSafeHex(t *testing.T) {
	for _, tc := range []struct {
		name, hex, aud string
		role           domain.Role
		confirmed      bool
		want           int
	}{{"admin", "01 03", "admin", domain.RoleAdmin, true, 202}, {"no confirmation", "0103", "admin", domain.RoleAdmin, false, 400}, {"cache clear", "0102020000000000", "admin", domain.RoleAdmin, true, 400}, {"farmer", "0103", "mini", domain.RoleFarmer, true, 403}, {"bad hex", "01ZZ", "admin", domain.RoleAdmin, true, 400}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockRawDebugAdminRepo{}
			h := &adminHandler{repo: repo}
			body := `{"gateway_id":"1","gateway_name":"FP_TEST","request_id":"0123456789abcdef0123456789abcdef","command_hex":"` + tc.hex + `","confirm_no_cache_clear":true,"confirmed":` + map[bool]string{true: "true", false: "false"}[tc.confirmed] + `}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/raw-debug/commands", strings.NewReader(body))
			req = req.WithContext(context.WithValue(req.Context(), claimsKey, auth.Claims{UserID: 7, Role: tc.role, Aud: tc.aud}))
			rec := httptest.NewRecorder()
			h.handleRawDebugCommand(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
			if tc.want != 202 && repo.calls != 0 {
				t.Fatal("rejected input queued")
			}
			if tc.want == 202 && repo.input.CommandHex != "0103" {
				t.Fatal("bytes changed")
			}
		})
	}
}
