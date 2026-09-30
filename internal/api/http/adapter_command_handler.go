package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"nehuang-iot-server/internal/domain"
)

func (h *adapterHandler) handlePendingCommands(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdapterToken(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	gatewayExternalID := strings.TrimSpace(r.URL.Query().Get("gateway_external_id"))
	if gatewayExternalID == "" {
		writeCode(w, http.StatusBadRequest, 400, "gateway_external_id is required")
		return
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeCode(w, http.StatusBadRequest, 400, "invalid limit")
			return
		}
		limit = parsed
	}
	commands, err := h.repo.ListPendingControlCommands(r.Context(), gatewayExternalID, limit)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to list pending commands")
		return
	}
	filtered := make([]domain.ControlCommand, 0, len(commands))
	for _, command := range commands {
		command.SendOnce = command.Extra != nil && domain.IsRawDebugExtra(*command.Extra)
		if command.SendOnce && r.URL.Query().Get("send_once") != "1" {
			continue
		}
		filtered = append(filtered, command)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": filtered}})
}

func (h *adapterHandler) handleCommandSent(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdapterToken(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	commandID, err := extractAdapterCommandID(r.URL.Path, "/api/v1/adapter/commands/", "/sent")
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid command id")
		return
	}
	var req struct {
		GatewayResponse string `json:"gateway_response"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var resp *string
	if strings.TrimSpace(req.GatewayResponse) != "" {
		v := strings.TrimSpace(req.GatewayResponse)
		resp = &v
	}
	if err := h.repo.MarkControlCommandSent(r.Context(), commandID, resp); err != nil {
		writeCode(w, http.StatusConflict, 409, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"command_id": idStr(commandID), "status": domain.CommandStatusSent.Label()}})
}

func (h *adapterHandler) handleCommandResult(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdapterToken(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	commandID, err := extractAdapterCommandID(r.URL.Path, "/api/v1/adapter/commands/", "/result")
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid command id")
		return
	}
	var req struct {
		Status          string `json:"status"`
		GatewayResponse string `json:"gateway_response"`
		ErrorMessage    string `json:"error_message"`
		ResultHex       string `json:"result_hex"`
		ResultSnapshot  string `json:"result_snapshot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	status, err := parseCommandResultStatus(req.Status)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	input := domain.AdapterCommandResultInput{
		CommandID:       commandID,
		Status:          status,
		GatewayResponse: optionalString(req.GatewayResponse),
		ErrorMessage:    optionalString(req.ErrorMessage),
		ResultHex:       optionalString(req.ResultHex),
		ResultSnapshot:  optionalString(req.ResultSnapshot),
	}
	if err := h.repo.ApplyControlCommandResult(r.Context(), input); err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to apply command result")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"command_id": idStr(commandID), "status": status.Label()}})
}

func (h *adapterHandler) checkAdapterToken(w http.ResponseWriter, r *http.Request) bool {
	token := r.Header.Get("X-Adapter-Token")
	if token == "" || token != h.adapterToken {
		writeCode(w, http.StatusUnauthorized, 401, "invalid or missing adapter token")
		return false
	}
	return true
}

func extractAdapterCommandID(path, prefix, suffix string) (uint64, error) {
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, strconv.ErrSyntax
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	raw = strings.Trim(raw, "/")
	return strconv.ParseUint(raw, 10, 64)
}

func parseCommandResultStatus(raw string) (domain.CommandStatus, error) {
	switch strings.TrimSpace(raw) {
	case "ack":
		return domain.CommandStatusAck, nil
	case "succeeded", "success":
		return domain.CommandStatusSucceeded, nil
	case "failed", "fail":
		return domain.CommandStatusFailed, nil
	case "timeout":
		return domain.CommandStatusTimeout, nil
	default:
		return 0, fmt.Errorf("status must be ack, succeeded, failed, or timeout")
	}
}

func optionalString(raw string) *string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil
	}
	return &v
}
