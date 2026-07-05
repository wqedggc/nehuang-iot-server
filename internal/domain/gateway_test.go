package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGatewayIDSerializesAsString(t *testing.T) {
	gateway := Gateway{GatewayID: 332018003367432192}

	data, err := json.Marshal(gateway)
	if err != nil {
		t.Fatalf("marshal gateway: %v", err)
	}

	body := string(data)
	if !strings.Contains(body, `"gateway_id":"332018003367432192"`) {
		t.Fatalf("gateway_id must be serialized as a string to avoid JS precision loss, got %s", body)
	}
}
