package protocol

import (
	"encoding/hex"
	"testing"
)

func TestCRC16Modbus_StandardVector(t *testing.T) {
	data := []byte("123456789")
	if got, want := CRC16Modbus(data), uint16(0x4B37); got != want {
		t.Fatalf("CRC16Modbus = 0x%04X, want 0x%04X", got, want)
	}
}

func TestAppendAndValidateCRC16Modbus(t *testing.T) {
	payload, _ := hex.DecodeString("AAAA0B010203")
	packet := AppendCRC16Modbus(payload)
	if !ValidateCRC16Modbus(packet) {
		t.Fatalf("packet with appended CRC should validate: %X", packet)
	}
	packet[3] ^= 0xFF
	if ValidateCRC16Modbus(packet) {
		t.Fatalf("mutated packet should fail CRC validation")
	}
}

func TestClassifyPayload_Register(t *testing.T) {
	msg := ClassifyPayload([]byte("FP_TEST_ADMIN_001\n"))
	if msg.Kind != MessageRegister {
		t.Fatalf("Kind = %s, want register", msg.Kind)
	}
	if msg.GatewayName != "FP_TEST_ADMIN_001" {
		t.Fatalf("GatewayName = %q", msg.GatewayName)
	}
	if got, want := GatewayExternalID(msg.GatewayName), "TEST_ADMIN_001"; got != want {
		t.Fatalf("GatewayExternalID = %q, want %q", got, want)
	}
}

func TestClassifyPayload_RegisterWithCoalescedHeartbeat(t *testing.T) {
	msg := ClassifyPayload([]byte("FP_TEST_ADMIN_001\nkeepalive"))
	if msg.Kind != MessageRegister {
		t.Fatalf("Kind = %s, want register", msg.Kind)
	}
	if msg.GatewayName != "FP_TEST_ADMIN_001" {
		t.Fatalf("GatewayName = %q", msg.GatewayName)
	}
	if got, want := GatewayExternalID(msg.GatewayName), "TEST_ADMIN_001"; got != want {
		t.Fatalf("GatewayExternalID = %q, want %q", got, want)
	}
}

func TestGatewayExternalIDNormalizesDirtyGatewayName(t *testing.T) {
	got := GatewayExternalID(" FP_TEST_ADMIN_002\nkeepalive ")
	if got != "TEST_ADMIN_002" {
		t.Fatalf("GatewayExternalID = %q, want TEST_ADMIN_002", got)
	}
}

func TestClassifyPayload_RejectsNonUnderscoreFPRegister(t *testing.T) {
	msg := ClassifyPayload([]byte("FPTEST_ADMIN_001\n"))
	if msg.Kind == MessageRegister {
		t.Fatalf("Kind = register, want non-FP_ payload not to register")
	}
}

func TestClassifyPayload_KeepaliveHeartbeat(t *testing.T) {
	for _, payload := range []string{"keepalive", "keepalive message"} {
		msg := ClassifyPayload([]byte(payload))
		if msg.Kind != MessageHeartbeat {
			t.Fatalf("payload %q kind = %s, want heartbeat", payload, msg.Kind)
		}
	}
}

func TestClassifyPayload_DoesNotMisparseRandomBytes(t *testing.T) {
	msg := ClassifyPayload([]byte{0x01, 0x02, 0x03, 0x04})
	if msg.Kind != MessageUnknown {
		t.Fatalf("Kind = %s, want unknown", msg.Kind)
	}
}

func TestClassifyPayload_MCU1UplinkUsesVariableCollectorID(t *testing.T) {
	packet := AppendCRC16Modbus([]byte{
		0x03, 0xE9, // collector ID: not a fixed AA AA header
		0x01, // BB: automatic status/history
		0x01, 0x02, 0x03,
		0x00, // no history records
		0xC0, // valid Tag
	})
	msg := ClassifyPayload(packet)
	if msg.Kind != MessageWorkData {
		t.Fatalf("Kind = %s, want work_data for MCU1 collector 0x03E9", msg.Kind)
	}
}

func TestClassifyPayload_MCU1ShapeStillRequiresValidCRC(t *testing.T) {
	packet := []byte{0x03, 0xE9, 0x03, 0x01, 0x02, 0x03, 0xC0, 0x00, 0x00}
	msg := ClassifyPayload(packet)
	if msg.Kind != MessageUnknown {
		t.Fatalf("Kind = %s, want unknown for invalid CRC", msg.Kind)
	}
}
