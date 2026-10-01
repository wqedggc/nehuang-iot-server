package http

import (
	"context"
	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/domain"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRawDebugRejectsCacheClearAndMultipleFramesBeforeQueue(t *testing.T) {
	for _, hex := range []string{"0100020C0000C071", "7E080100020C0000C071", "0100020C0000C0710100030C0000004C90", "7E080100020C0000C0717E090100030C0000004C90", "7E09000103122501818B9200", "7E09000103122501818B927E09000103122501818B92"} {
		t.Run(hex, func(t *testing.T) {
			repo := &mockRawDebugAdminRepo{}
			h := &adminHandler{repo: repo}
			body := `{"gateway_id":"1","gateway_name":"FP_TEST","request_id":"0123456789abcdef0123456789abcdef","command_hex":"` + hex + `","confirmed":true,"confirm_no_cache_clear":true}`
			req := httptest.NewRequest("POST", "/api/v1/admin/raw-debug/commands", strings.NewReader(body))
			req = req.WithContext(context.WithValue(req.Context(), claimsKey, auth.Claims{UserID: 7, Role: domain.RoleAdmin, Aud: "admin"}))
			w := httptest.NewRecorder()
			h.handleRawDebugCommand(w, req)
			if w.Code != 400 || repo.calls != 0 {
				t.Fatalf("unsafe queued: HTTP=%d calls=%d", w.Code, repo.calls)
			}
		})
	}
}
func TestLegacyPointCustomHexRejectsCacheClearAndMultipleFrames(t *testing.T) {
	for _, h := range []string{"0100020C0000C071", "7E080100020C0000C071", "0100020C0000C0710100030C0000004C90", "7E09000103122501818B927E09000103122501818B92"} {
		if _, _, e := buildAdminControlCommandInput(1, 2, "", h); e == nil {
			t.Fatalf("legacy entry accepted %s", h)
		}
	}
}

func TestLegacyPointSingleByteCustomControlsRemainCompatible(t *testing.T) {
	for _, h := range []string{"F0", "0F", "00"} {
		in, _, e := buildAdminControlCommandInput(1, 2, "", h)
		if e != nil || in.CommandHex != h {
			t.Fatalf("legacy %s: %v", h, e)
		}
	}
}
