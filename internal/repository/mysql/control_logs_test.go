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
	if strings.Contains(listControlLogsQuery, " op_id, gateway_id,") {
		t.Fatalf("control logs query uses ambiguous bare gateway_id")
	}
}
