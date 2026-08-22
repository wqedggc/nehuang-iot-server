package mysql

import (
	"strings"
	"testing"
)

func TestGetControlCommandByIDQueryUsesExistingCommandModel(t *testing.T) {
	for _, field := range []string{
		"FROM control_commands cc",
		"cc.command_id = ?",
		"cc.point_id",
		"cc.command_status",
		"cc.finished_at_ms",
		"cc.error_message",
	} {
		if !strings.Contains(getControlCommandByIDQuery, field) {
			t.Fatalf("command status query must contain %q", field)
		}
	}
	if strings.Contains(getControlCommandByIDQuery, "control_results") {
		t.Fatal("command status must use control_commands as its authoritative source")
	}
}

func TestCheckBindRelationQueryCoversMiniDevicePermissions(t *testing.T) {
	for _, clause := range []string{
		"br.point_id = dp.point_id",
		"br.node_id = dp.node_id AND br.point_id IS NULL",
		"br.gateway_id = dp.gateway_id",
		"cg.access_mode = 'exclusive'",
		"br.bind_status = 1",
	} {
		if !strings.Contains(checkBindRelationQuery, clause) {
			t.Fatalf("mini device permission query must contain %q", clause)
		}
	}
	if got := strings.Count(checkBindRelationQuery, "br.user_id = ?"); got != 3 {
		t.Fatalf("mini device permission query must scope all binding types by user, got %d clauses", got)
	}
}
