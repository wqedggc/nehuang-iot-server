package mysql

import (
	"nehuang-iot-server/internal/domain"
	"strings"
	"testing"
)

func TestListControlLogsQueryUsesControlCommandsAndQualifiesGatewayID(t *testing.T) {
	if !strings.Contains(listControlLogsQuery, "FROM control_commands cc") {
		t.Fatalf("control logs query must read control_commands")
	}
	if !strings.Contains(listControlLogsQuery, "cc.gateway_id") {
		t.Fatalf("control logs query must qualify gateway_id from control_commands")
	}
	for _, field := range []string{"cc.command_source", "cc.sent_at_ms", "cc.finished_at_ms", "cc.gateway_response", "cc.error_message"} {
		if !strings.Contains(listControlLogsQuery, field) {
			t.Fatalf("control logs query must expose %s", field)
		}
	}
	if strings.Contains(listControlLogsQuery, " op_id, gateway_id,") {
		t.Fatalf("control logs query uses ambiguous bare gateway_id")
	}
}

func TestBuildOperationLogFilterClauses(t *testing.T) {
	start := "2026-07-01 00:00:00"
	end := "2026-07-31 23:59:59"
	kw := "admin"
	opType := uint8(3)
	whereClause, args := buildOperationLogFilterQuery(domain.OperationLogQuery{
		Start:   &start,
		End:     &end,
		Keyword: &kw,
		OpType:  &opType,
	})
	for _, field := range []string{
		"op.operated_at_ms >= ?",
		"op.operated_at_ms <= ?",
		"op.op_type = ?",
		"op.bind_user_name LIKE ?",
		"op.op_content, '') LIKE ?",
	} {
		if !strings.Contains(whereClause, field) {
			t.Fatalf("operation log filter should contain %q", field)
		}
	}
	if len(args) != 7 {
		t.Fatalf("operation log args: want 7, got %d", len(args))
	}
}

func TestBuildControlLogFilterClauses(t *testing.T) {
	start := "2026-07-01 00:00:00"
	kw := "8001"
	status := uint8(4)
	source := uint8(1)
	whereClause, args := buildControlLogFilterQuery(domain.ControlLogQuery{
		Start:     &start,
		Keyword:   &kw,
		CmdStatus: &status,
		Source:    &source,
	})
	for _, field := range []string{"cc.requested_at_ms >= ?", "cc.command_status = ?", "cc.command_source = ?", "CAST(cc.command_id AS CHAR) LIKE ?", "COALESCE(dp.point_name, '') LIKE ?"} {
		if !strings.Contains(whereClause, field) {
			t.Fatalf("control log filter should contain %q", field)
		}
	}
	if len(args) != 8 {
		t.Fatalf("control log args: want 8, got %d", len(args))
	}
}

func TestNormalizePayloadHexKeywordStripsCommonSeparators(t *testing.T) {
	got := normalizePayloadHexKeyword("03 03 00 00 00 01 85 e8")
	if got != "03030000000185E8" {
		t.Fatalf("normalized hex: want 03030000000185E8, got %q", got)
	}
}

func TestNormalizePayloadHexKeywordKeepsPlainTextSearch(t *testing.T) {
	got := normalizePayloadHexKeyword("FP_NB114")
	if got != "FP_NB114" {
		t.Fatalf("plain text keyword should be unchanged, got %q", got)
	}
}
