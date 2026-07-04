package gatewayadapter

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"nehuang-iot-server/internal/gatewayadapter/journal"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

func TestHandleConnForwardsEveryReadAsRawFrame(t *testing.T) {
	var mu sync.Mutex
	var received []RawFrameEnvelope
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/adapter/raw-frames" {
			t.Errorf("path: want /api/v1/adapter/raw-frames, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Adapter-Token") != "test-token" {
			t.Errorf("adapter token mismatch")
		}
		defer r.Body.Close()
		var envelope RawFrameEnvelope
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			t.Errorf("decode raw frame: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, envelope)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()

	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()

	srv := NewServer(Config{
		APIBaseURL:     api.URL,
		AdapterToken:   "test-token",
		ReadBufferSize: 64,
		ReadTimeout:    time.Second,
	}, jw, NewLogger(io.Discard))

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(serverConn)
	}()

	frames := [][]byte{
		[]byte("FP_TEST_ADMIN_001\n"),
		[]byte("HEARTBEAT\n"),
		protocol.AppendCRC16Modbus([]byte{0xAA, 0xAA, 0x0B, 0x01, 0x02, 0x03}),
		protocol.AppendCRC16Modbus([]byte{0xAA, 0xAA, 0x0B, 0xFF, 0xFF, 0xFF, 0x01}),
		{0x01, 0x02, 0x03, 0x04},
	}
	for _, frame := range frames {
		if _, err := clientConn.Write(frame); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	}
	_ = clientConn.Close()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(received) != len(frames) {
		t.Fatalf("raw frame posts: want %d, got %d", len(frames), len(received))
	}
	if received[0].GatewayName != "FP_TEST_ADMIN_001" {
		t.Fatalf("register gateway name: want FP_TEST_ADMIN_001, got %q", received[0].GatewayName)
	}
	if received[1].GatewayName != "FP_TEST_ADMIN_001" {
		t.Fatalf("heartbeat should keep connection gateway name, got %q", received[1].GatewayName)
	}
	for i, envelope := range received {
		if envelope.PayloadHex == "" {
			t.Fatalf("frame %d missing payload_hex", i)
		}
		if envelope.MessageID == "" || envelope.ConnectionID == "" {
			t.Fatalf("frame %d missing message identity", i)
		}
		if envelope.Direction != string(journal.DirectionInbound) {
			t.Fatalf("frame %d direction: want inbound, got %q", i, envelope.Direction)
		}
	}
}
