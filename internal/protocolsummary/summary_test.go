package protocolsummary

import (
	"encoding/hex"
	"testing"

	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
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

func TestBuildGatewayRegisterSummaryDoesNotClaimMCU1Identity(t *testing.T) {
	summary := Build(hex.EncodeToString([]byte("FP_TEST_ADMIN_002\n")))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "register" || summary.Label != "云边设备网关注册（非 MCU1）" {
		t.Fatalf("kind/label = %q/%q", summary.Kind, summary.Label)
	}
	name := findField(t, summary.Fields, "gateway_name")
	if name.RawHex == "" || name.Status != "decoded" || name.ByteStart != 0 {
		t.Fatalf("gateway_name = %#v", name)
	}
	externalID := findField(t, summary.Fields, "gateway_external_id")
	if externalID.Status != "decoded" || externalID.Note == "" {
		t.Fatalf("gateway_external_id = %#v", externalID)
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

func TestBuildMCU1AutomaticStatusWithoutHistory(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{
		0x12, 0x34, 0x01, 0x10, 0x20, 0x30, 0x00, 0xC2,
	})
	summary := Build(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_uplink_automatic" {
		t.Fatalf("kind = %q, want mcu1_uplink_automatic", summary.Kind)
	}
	if summary.Status != "partial" {
		t.Fatalf("status = %q, want partial", summary.Status)
	}
	if summary.CRCValid == nil || !*summary.CRCValid {
		t.Fatalf("crc_valid = %#v, want true", summary.CRCValid)
	}
	assertFieldDetail(t, summary.Fields, "device_id", 0, 1, "12 34", "0x1234")
	assertFieldDetail(t, summary.Fields, "function_code", 2, 2, "01", "自动状态与历史温度上报")
	assertFieldDetail(t, summary.Fields, "history_marker", 6, 6, "00", "无历史温度记录")
	tag := findField(t, summary.Fields, "tag")
	if len(tag.Bits) != 8 {
		t.Fatalf("tag bits = %d, want 8", len(tag.Bits))
	}
	assertBit(t, tag.Bits, "N", 1, "NTC 在线")
	assertBit(t, tag.Bits, "T", 1, "开启继电器已启动")
	assertBit(t, tag.Bits, "U", 0, "关闭继电器已停止")
}

func TestBuildDownlinkMCU1Initialization(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{
		0x12, 0x34, 0x00, 0x10, 0x20, 0x30,
	})
	summary := BuildDownlink(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_downlink_initialization" || summary.Status != "partial" {
		t.Fatalf("kind/status = %q/%q", summary.Kind, summary.Status)
	}
	if summary.CRCValid == nil || !*summary.CRCValid {
		t.Fatalf("crc_valid = %#v, want true", summary.CRCValid)
	}
	assertFieldDetail(t, summary.Fields, "device_id", 0, 1, "12 34", "0x1234")
	assertFieldDetail(t, summary.Fields, "function_code", 2, 2, "00", "整点校时/初始化")
	assertFieldDetail(t, summary.Fields, "report_time", 3, 5, "10 20 30", "10 20 30（原始字节）")
}

func TestBuildDownlinkMCU1AutomaticConfigAndRead(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{
		0x12, 0x34, 0x01, 0x10, 0x20, 0x30,
		0x00, 0x19, 0x00, 0x1E,
	})
	summary := BuildDownlink(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_downlink_automatic_config_read" || summary.Status != "partial" {
		t.Fatalf("kind/status = %q/%q", summary.Kind, summary.Status)
	}
	assertFieldDetail(t, summary.Fields, "lower_temperature_threshold", 6, 7, "00 19", "换算规则待确认")
	assertFieldDetail(t, summary.Fields, "upper_temperature_threshold", 8, 9, "00 1E", "换算规则待确认")
}

func TestBuildDownlinkMCU1HistoryReceivedHasNoDocumentedCRC(t *testing.T) {
	summary := BuildDownlink("123402102030")
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_downlink_history_received" || summary.Status != "partial" {
		t.Fatalf("kind/status = %q/%q", summary.Kind, summary.Status)
	}
	if summary.CRCValid != nil {
		t.Fatalf("crc_valid = %#v, want nil for documented six-byte reply", summary.CRCValid)
	}
	if !hasDiagnostic(summary.Diagnostics, "ack_semantics_unknown") {
		t.Fatalf("diagnostics = %#v", summary.Diagnostics)
	}
}

func TestBuildDownlinkMCU1ManualControlDecodesTag(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{
		0x12, 0x34, 0x03, 0x10, 0x20, 0x30, 0xC2,
	})
	summary := BuildDownlink(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_downlink_manual_control" || summary.Status != "partial" {
		t.Fatalf("kind/status = %q/%q", summary.Kind, summary.Status)
	}
	tag := findField(t, summary.Fields, "tag")
	assertBit(t, tag.Bits, "T", 1, "开启继电器已启动")
	assertBit(t, tag.Bits, "U", 0, "关闭继电器已停止")
}

func TestBuildDownlinkMarksLegacyOneByteControlWord(t *testing.T) {
	tests := []struct {
		hex   string
		label string
	}{
		{hex: "F0", label: "旧联调控制字：开启"},
		{hex: "0F", label: "旧联调控制字：关闭"},
		{hex: "00", label: "旧联调控制字：停止"},
	}
	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			summary := BuildDownlink(tt.hex)
			if summary == nil || summary.Kind != "legacy_control_word" || summary.Label != tt.label {
				t.Fatalf("summary = %#v", summary)
			}
			if summary.Status != "partial" || summary.CRCValid != nil {
				t.Fatalf("status/crc = %q/%#v", summary.Status, summary.CRCValid)
			}
		})
	}
}

func TestBuildMCU1ManualStatusKeepsUnknownTemperatureRaw(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{
		0x00, 0x2A, 0x02, 0x11, 0x22, 0x33, 0x01, 0x90, 0xC0,
	})
	summary := Build(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_uplink_manual" {
		t.Fatalf("kind = %q, want mcu1_uplink_manual", summary.Kind)
	}
	if summary.Status != "partial" {
		t.Fatalf("status = %q, want partial", summary.Status)
	}
	temperature := findField(t, summary.Fields, "current_temperature")
	if temperature.Status != "unknown" {
		t.Fatalf("temperature status = %q, want unknown", temperature.Status)
	}
	if temperature.Value != "换算规则待确认" {
		t.Fatalf("temperature value = %q", temperature.Value)
	}
}

func TestBuildMCU1ManualStatusAcceptsOneByteTemperatureVariant(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{
		0x00, 0x2A, 0x02, 0x11, 0x22, 0x33, 0x19, 0xC0,
	})
	summary := Build(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_uplink_manual" || summary.Status != "partial" {
		t.Fatalf("kind/status = %q/%q", summary.Kind, summary.Status)
	}
	assertFieldDetail(t, summary.Fields, "current_temperature", 6, 6, "19", "换算规则待确认")
	assertFieldDetail(t, summary.Fields, "tag", 7, 7, "C0", "11000000")
}

func TestBuildMCU1FaultStatusRejectsInvalidTagCombination(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{
		0x00, 0x2A, 0x03, 0x11, 0x22, 0x33, 0xC3,
	})
	summary := Build(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Status != "invalid" {
		t.Fatalf("status = %q, want invalid", summary.Status)
	}
	if len(summary.Diagnostics) == 0 {
		t.Fatal("diagnostics is empty")
	}
	tag := findField(t, summary.Fields, "tag")
	if tag.Status != "invalid" {
		t.Fatalf("tag status = %q, want invalid", tag.Status)
	}
}

func TestBuildMCU1AutomaticStatusWithHistory(t *testing.T) {
	body := []byte{0x00, 0x2A, 0x01, 0x11, 0x22, 0x33}
	for i := 0; i < 12; i++ {
		body = append(body, byte(i+1), 0x01, byte(0x20+i))
	}
	body = append(body, 0xC0)
	packet := protocol.AppendCRC16Modbus(body)
	summary := Build(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_uplink_automatic" || summary.Status != "partial" {
		t.Fatalf("kind/status = %q/%q", summary.Kind, summary.Status)
	}
	if summary.RecordCount != 12 {
		t.Fatalf("record_count = %d, want 12", summary.RecordCount)
	}
	record := findField(t, summary.Fields, "history_record_01")
	if record.ByteStart != 6 || record.ByteEnd != 8 || record.RawHex != "01 01 20" {
		t.Fatalf("first history record = %#v", record)
	}
}

func TestBuildMCU1KnownBBReportsLengthMismatch(t *testing.T) {
	packet := protocol.AppendCRC16Modbus([]byte{
		0x00, 0x2A, 0x01, 0x11, 0x22, 0x33, 0x01, 0xC0,
	})
	summary := Build(hex.EncodeToString(packet))
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "mcu1_uplink_automatic" || summary.Status != "invalid" {
		t.Fatalf("kind/status = %q/%q", summary.Kind, summary.Status)
	}
	if !hasDiagnostic(summary.Diagnostics, "invalid_length") {
		t.Fatalf("diagnostics = %#v", summary.Diagnostics)
	}
}

func TestBuildShortBinaryWithKnownBBDoesNotAssumeCompleteMCU1Frame(t *testing.T) {
	summary := Build("000101")
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Kind != "unknown" {
		t.Fatalf("kind = %q, want unknown", summary.Kind)
	}
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

func assertFieldDetail(t *testing.T, fields []domain.ProtocolSummaryField, name string, byteStart, byteEnd int, rawHex, value string) {
	t.Helper()
	field := findField(t, fields, name)
	if field.ByteStart != byteStart || field.ByteEnd != byteEnd || field.RawHex != rawHex || field.Value != value {
		t.Fatalf("field %s = %#v", name, field)
	}
}

func findField(t *testing.T, fields []domain.ProtocolSummaryField, name string) domain.ProtocolSummaryField {
	t.Helper()
	for _, field := range fields {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("field %s not found in %#v", name, fields)
	return domain.ProtocolSummaryField{}
}

func assertBit(t *testing.T, bits []domain.ProtocolSummaryBit, symbol string, value uint8, meaning string) {
	t.Helper()
	for _, bit := range bits {
		if bit.Symbol == symbol {
			if bit.Value != value || bit.Meaning != meaning {
				t.Fatalf("bit %s = %#v", symbol, bit)
			}
			return
		}
	}
	t.Fatalf("bit %s not found in %#v", symbol, bits)
}

func hasDiagnostic(diagnostics []domain.ProtocolDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
