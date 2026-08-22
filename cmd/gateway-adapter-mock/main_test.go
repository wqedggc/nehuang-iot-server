package main

import (
	"testing"

	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

func TestSelectFramesMCU1BuildsDocumentedUplinkSamples(t *testing.T) {
	frames, err := selectFrames("mcu1", "FP_TEST_ADMIN_002")
	if err != nil {
		t.Fatalf("select MCU1 frames: %v", err)
	}
	if len(frames) != 3 {
		t.Fatalf("MCU1 frame count = %d, want 3", len(frames))
	}

	want := []struct {
		name   string
		bb     byte
		length int
		tag    byte
	}{
		{name: "mcu1-auto", bb: 0x01, length: 10, tag: 0xC0},
		{name: "mcu1-manual", bb: 0x02, length: 10, tag: 0xC0},
		{name: "mcu1-fault", bb: 0x03, length: 9, tag: 0xE0},
	}
	for i, frame := range frames {
		if frame.name != want[i].name {
			t.Fatalf("frame[%d] name = %q, want %q", i, frame.name, want[i].name)
		}
		if len(frame.payload) != want[i].length {
			t.Fatalf("%s length = %d, want %d", frame.name, len(frame.payload), want[i].length)
		}
		if frame.payload[0] != 0xAA || frame.payload[1] != 0xAA || frame.payload[2] != want[i].bb {
			t.Fatalf("%s header = % X", frame.name, frame.payload[:3])
		}
		if !protocol.ValidateCRC16Modbus(frame.payload) {
			t.Fatalf("%s CRC is invalid: % X", frame.name, frame.payload)
		}
		tagOffset := len(frame.payload) - 3
		if frame.payload[tagOffset] != want[i].tag {
			t.Fatalf("%s Tag = 0x%02X, want 0x%02X", frame.name, frame.payload[tagOffset], want[i].tag)
		}
		if _, err := protocol.DecodeMCU1Tag(frame.payload[tagOffset]); err != nil {
			t.Fatalf("%s Tag validation: %v", frame.name, err)
		}
	}
}
