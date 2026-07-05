package protocolsummary

import (
	"testing"

	"nehuang-iot-server/internal/domain"
)

func TestBuildKeepaliveSummary(t *testing.T) {
	summary := Build("6B656570616C697665")
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "heartbeat" {
		t.Fatalf("kind = %q, want heartbeat", summary.Kind)
	}
	if summary.ASCII != "keepalive" {
		t.Fatalf("ascii = %q, want keepalive", summary.ASCII)
	}
	if summary.CRCValid != nil {
		t.Fatalf("ascii heartbeat should not show crc result")
	}
}

func TestBuildModbusReadHoldingRegistersSummary(t *testing.T) {
	summary := Build("03030000000185E8")
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "modbus_read_holding_registers" {
		t.Fatalf("kind = %q, want modbus_read_holding_registers", summary.Kind)
	}
	if summary.CRCValid == nil || !*summary.CRCValid {
		t.Fatalf("crc_valid = %#v, want true", summary.CRCValid)
	}
	if summary.Label != "Modbus 读寄存器指令" {
		t.Fatalf("label = %q", summary.Label)
	}
	assertField(t, summary.Fields, "function_code", "0x03")
	assertField(t, summary.Fields, "start_register", "0")
	assertField(t, summary.Fields, "register_count", "1")
}

func assertField(t *testing.T, fields []domain.ProtocolSummaryField, name, value string) {
	t.Helper()
	for _, f := range fields {
		if f.Name == name {
			if f.Value != value {
				t.Fatalf("field %s = %q, want %q", name, f.Value, value)
			}
			return
		}
	}
	t.Fatalf("field %s not found in %#v", name, fields)
}
