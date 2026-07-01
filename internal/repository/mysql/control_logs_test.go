package mysql

import (
	"strings"
	"testing"
)

func TestListControlLogsQueryQualifiesGatewayID(t *testing.T) {
	if !strings.Contains(listControlLogsQuery, "op_records.gateway_id") {
		t.Fatalf("control logs query must qualify gateway_id from op_records")
	}
	if strings.Contains(listControlLogsQuery, " op_id, gateway_id,") {
		t.Fatalf("control logs query uses ambiguous bare gateway_id")
	}
}
