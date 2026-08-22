package main

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

func TestSelectFramesMCU1BuildsDocumentedUplinkSamples(t *testing.T) {
	frames, err := selectFrames("mcu1", "FP_TEST_ADMIN_002", []byte{0x03, 0xE9})
	if err != nil {
		t.Fatalf("select MCU1 frames: %v", err)
	}
	if len(frames) != 4 {
		t.Fatalf("MCU1 frame count = %d, want 4", len(frames))
	}

	want := []struct {
		name   string
		bb     byte
		length int
		tag    byte
	}{
		{name: "mcu1-auto", bb: 0x01, length: 10, tag: 0xC0},
		{name: "mcu1-auto-history", bb: 0x01, length: 45, tag: 0xC0},
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
		if frame.payload[0] != 0x03 || frame.payload[1] != 0xE9 || frame.payload[2] != want[i].bb {
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

func TestParseCollectorIDHexRequiresExactlyTwoBytes(t *testing.T) {
	got, err := parseCollectorIDHex("03E9")
	if err != nil || !bytes.Equal(got, []byte{0x03, 0xE9}) {
		t.Fatalf("parse collector ID = % X, %v", got, err)
	}
	for _, raw := range []string{"", "03", "03E900", "ZZZZ"} {
		if _, err := parseCollectorIDHex(raw); err == nil {
			t.Fatalf("parseCollectorIDHex(%q) error = nil", raw)
		}
	}
}

func TestRunLoopSendsSelectedMCU1FramesInsteadOfLegacyTelemetry(t *testing.T) {
	frames, err := selectFrames("register,mcu1-manual", "FP_TEST_ADMIN_002", []byte{0x03, 0xE9})
	if err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	done := make(chan error, 1)
	go func() {
		done <- runLoop(clientConn, "FP_TEST_ADMIN_002", frames, 40*time.Millisecond, time.Hour, time.Hour, 0, 0, 0, false, 0, 0)
		_ = clientConn.Close()
	}()

	want := append(append([]byte(nil), frames[0].payload...), frames[1].payload...)
	got := make([]byte, len(want))
	if _, err := io.ReadFull(serverConn, got); err != nil {
		t.Fatalf("read loop bootstrap: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("loop bootstrap = % X, want % X", got, want)
	}
	if bytes.Contains(got, []byte("NH_TELEMETRY|")) {
		t.Fatalf("loop unexpectedly sent legacy telemetry: %q", got)
	}
	if err := <-done; err != nil {
		t.Fatalf("runLoop: %v", err)
	}
}
