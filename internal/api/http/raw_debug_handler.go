package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"nehuang-iot-server/internal/domain"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type rawDebugRepository interface {
	CreateRawDebugCommand(context.Context, domain.RawDebugCommandInput) (*domain.ControlCommand, error)
}

var rawDebugRequestID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func (h *adminHandler) handleRawDebugCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCode(w, 405, 405, "method not allowed")
		return
	}
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, 401, 401, "missing claims")
		return
	}
	if claims.Aud != "admin" || (claims.Role != domain.RoleAdmin && claims.Role != domain.RoleSuperAdmin) {
		writeCode(w, 403, 403, "admin required")
		return
	}
	var req struct {
		GatewayID           string `json:"gateway_id"`
		GatewayName         string `json:"gateway_name"`
		CommandHex          string `json:"command_hex"`
		RequestID           string `json:"request_id"`
		Remark              string `json:"remark"`
		Confirmed           bool   `json:"confirmed"`
		ConfirmNoCacheClear bool   `json:"confirm_no_cache_clear"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeCode(w, 400, 400, "invalid request body")
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeCode(w, 400, 400, "only one request object is accepted")
		return
	}
	id, err := strconv.ParseUint(req.GatewayID, 10, 64)
	if err != nil || id == 0 || req.GatewayName == "" || len(req.GatewayName) > 128 || !rawDebugRequestID.MatchString(req.RequestID) || !req.Confirmed || !req.ConfirmNoCacheClear || len(req.Remark) > 160 {
		writeCode(w, 400, 400, "exact gateway, request_id and explicit no-cache-clear confirmation are required")
		return
	}
	text, err := domain.ValidateRawDebugHex(req.CommandHex)
	if err != nil {
		writeCode(w, 400, 400, err.Error())
		return
	}
	repo, ok := h.repo.(rawDebugRepository)
	if !ok {
		writeCode(w, 503, 503, "raw debug repository unavailable")
		return
	}
	cmd, err := repo.CreateRawDebugCommand(r.Context(), domain.RawDebugCommandInput{GatewayID: id, GatewayName: req.GatewayName, UserID: claims.UserID, Role: claims.Role, CommandHex: text, RequestID: req.RequestID, Remark: strings.TrimSpace(req.Remark)})
	if errors.Is(err, domain.ErrRawDebugRateLimited) {
		writeCode(w, 429, 429, "每个网关至少间隔2秒，不自动重试")
		return
	}
	if errors.Is(err, domain.ErrRawDebugConflict) {
		writeCode(w, 409, 409, "gateway or request conflicts; verify target and existing log")
		return
	}
	if err != nil || cmd == nil {
		writeCode(w, 500, 500, "failed to create raw debug command")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"command_id": idStr(cmd.CommandID), "gateway_id": idStr(cmd.GatewayID), "gateway_name": req.GatewayName, "command_hex": cmd.CommandHex, "status": cmd.CommandStatus.Label(), "send_once": true, "request_id": req.RequestID}})
}
func (h *adapterHandler) handleCommandClaim(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdapterToken(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeCode(w, 405, 405, "method not allowed")
		return
	}
	id, err := extractAdapterCommandID(r.URL.Path, "/api/v1/adapter/commands/", "/claim")
	if err != nil || id == 0 {
		writeCode(w, 400, 400, "invalid command id")
		return
	}
	var req struct {
		GatewayExternalID string `json:"gateway_external_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req) != nil || req.GatewayExternalID == "" {
		writeCode(w, 400, 400, "gateway identity required")
		return
	}
	repo, ok := h.repo.(interface {
		ClaimRawDebugCommand(context.Context, uint64, string) error
	})
	if !ok {
		writeCode(w, 503, 503, "claim unavailable")
		return
	}
	if err = repo.ClaimRawDebugCommand(r.Context(), id, req.GatewayExternalID); err != nil {
		writeCode(w, 409, 409, "claim rejected; do not transmit")
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"command_id": idStr(id), "status": "unknown"}})
}
