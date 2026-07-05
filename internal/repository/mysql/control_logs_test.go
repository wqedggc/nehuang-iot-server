package mysql

import (
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
