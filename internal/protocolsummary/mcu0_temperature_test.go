package protocolsummary

import (
	"encoding/hex"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
	"testing"
)

func TestCurrentMCU0SummaryKeepsUnresolvedFields(t *testing.T) {
	raw := protocol.AppendCRC16Modbus([]byte{1, 2, 2, 12, 0, 0, 0xFF, 0xCE, 0xC0})
	s := BuildCurrentUplink(hex.EncodeToString(raw))
	if s.Kind != "mcu0_temperature_candidate" || s.Status != "partial" {
		t.Fatalf("%+v", s)
	}
	found := false
	for _, f := range s.Fields {
		if f.Name == "current_temperature" {
			found = f.Value == "-5.0 ℃"
		}
		if f.Name == "report_time" && f.Status != "unknown" {
			t.Fatal("guessed clock")
		}
	}
	if !found {
		t.Fatal("temperature missing")
	}
	ack := BuildCurrentDownlink(hex.EncodeToString(protocol.BuildMCU0HistoryACK([2]byte{1, 2}, [3]byte{12, 0, 0})))
	if ack.CRCValid == nil || !*ack.CRCValid || ack.Kind != "mcu0_history_ack" {
		t.Fatal("ACK summary incorrect")
	}
}
