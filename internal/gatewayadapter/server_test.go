package gatewayadapter

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nehuang-iot-server/internal/gatewayadapter/journal"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

func TestHandleConnForwardsEveryReadAsRawFrame(t *testing.T) {
	var mu sync.Mutex
	var received []RawFrameEnvelope
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/adapter/raw-frames":
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
		case r.URL.Path == "/api/v1/adapter/commands/pending":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"items": []any{}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
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

func TestHandleConnDispatchesPendingCommandsAndMarksSent(t *testing.T) {
	var rawFrames atomic.Int32
	var pendingRequests atomic.Int32
	var sentRequests atomic.Int32
	sentCh := make(chan struct{}, 1)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/adapter/raw-frames":
			rawFrames.Add(1)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/adapter/commands/pending":
			pendingRequests.Add(1)
			if got := r.URL.Query().Get("gateway_external_id"); got != "TEST_ADMIN_001" {
				t.Errorf("gateway_external_id: want TEST_ADMIN_001, got %q", got)
			}
			if got := r.URL.Query().Get("limit"); got != "5" {
				t.Errorf("limit: want 5, got %q", got)
			}
			if pendingRequests.Load() == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"items": []map[string]any{
							{"command_id": "9001", "command_hex": "AABBCC"},
						},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"items": []any{}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/adapter/commands/9001/sent":
			sentRequests.Add(1)
			select {
			case sentCh <- struct{}{}:
			default:
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()

	srv := NewServer(Config{
		APIBaseURL:          api.URL,
		AdapterToken:        "test-token",
		ReadBufferSize:      64,
		ReadTimeout:         time.Second,
		CommandPollInterval: 10 * time.Millisecond,
		CommandPollLimit:    5,
		CommandWriteTimeout: 200 * time.Millisecond,
	}, jw, NewLogger(io.Discard))

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(serverConn)
	}()

	if _, err := clientConn.Write([]byte("FP_TEST_ADMIN_001\n")); err != nil {
		t.Fatalf("write register: %v", err)
	}

	received := make([]byte, 3)
	if err := clientConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if _, err := io.ReadFull(clientConn, received); err != nil {
		t.Fatalf("read pending command payload: %v", err)
	}
	if got := strings.ToUpper(hex.EncodeToString(received)); got != "AABBCC" {
		t.Fatalf("pending command bytes: want AABBCC, got %s", got)
	}

	select {
	case <-sentCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sent callback")
	}

	_ = clientConn.Close()
	<-done

	if sentRequests.Load() != 1 {
		t.Fatalf("sent requests: want 1, got %d", sentRequests.Load())
	}
	if rawFrames.Load() != 1 {
		t.Fatalf("raw frame posts: want 1, got %d", rawFrames.Load())
	}
}

func TestHandleConnDoesNotMarkSentAfterDisconnect(t *testing.T) {
	pendingStarted := make(chan struct{})
	releasePending := make(chan struct{})
	var sentRequests atomic.Int32

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/adapter/raw-frames":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/adapter/commands/pending":
			select {
			case <-pendingStarted:
			default:
				close(pendingStarted)
			}
			<-releasePending
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"items": []map[string]any{
						{"command_id": "9002", "command_hex": "AABBCC"},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/adapter/commands/9002/sent":
			sentRequests.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()

	srv := NewServer(Config{
		APIBaseURL:          api.URL,
		AdapterToken:        "test-token",
		ReadBufferSize:      64,
		ReadTimeout:         time.Second,
		CommandPollInterval: 10 * time.Millisecond,
		CommandPollLimit:    5,
		CommandWriteTimeout: 200 * time.Millisecond,
	}, jw, NewLogger(io.Discard))

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(serverConn)
	}()

	if _, err := clientConn.Write([]byte("FP_TEST_ADMIN_001\n")); err != nil {
		t.Fatalf("write register: %v", err)
	}

	select {
	case <-pendingStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending poll")
	}

	_ = clientConn.Close()
	close(releasePending)
	<-done

	time.Sleep(50 * time.Millisecond)
	if sentRequests.Load() != 0 {
		t.Fatalf("sent requests after disconnect: want 0, got %d", sentRequests.Load())
	}
}
