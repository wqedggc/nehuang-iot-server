package gatewayadapter

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func TestHandleConnForwardsEveryReadAsRawCapture(t *testing.T) {
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
		APIBaseURL:        api.URL,
		AdapterToken:      "test-token",
		ReadBufferSize:    64,
		ReadTimeout:       time.Second,
		RawDiskMaxUsedPct: 100,
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
		[]byte("NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=1|channel_no=1|value=26.5|unit=C"),
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
	if result := srv.replayPendingBatch(t.Context()); result.failed != 0 {
		t.Fatal("replay raw captures")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) < len(frames) {
		t.Fatalf("raw captures: want at least %d TCP chunks, got %d", len(frames), len(received))
	}
	if received[1].GatewayName != "FP_TEST_ADMIN_001" {
		t.Fatalf("heartbeat should keep connection gateway name, got %q", received[1].GatewayName)
	}
	var captured []byte
	for i, envelope := range received {
		if envelope.PayloadHex == "" {
			t.Fatalf("frame %d missing payload_hex", i)
		}
		chunk, err := hex.DecodeString(envelope.PayloadHex)
		if err != nil {
			t.Fatalf("decode captured chunk %d: %v", i, err)
		}
		captured = append(captured, chunk...)
		if envelope.MessageID == "" || envelope.ConnectionID == "" {
			t.Fatalf("frame %d missing message identity", i)
		}
		if envelope.CaptureID != envelope.MessageID {
			t.Fatalf("frame %d capture identity mismatch: capture=%q message=%q", i, envelope.CaptureID, envelope.MessageID)
		}
		if envelope.Direction != string(journal.DirectionInbound) {
			t.Fatalf("frame %d direction: want inbound, got %q", i, envelope.Direction)
		}
	}
	var sent []byte
	for _, frame := range frames {
		sent = append(sent, frame...)
	}
	if got, want := strings.ToUpper(hex.EncodeToString(captured)), strings.ToUpper(hex.EncodeToString(sent)); got != want {
		t.Fatalf("captured TCP bytes differ: want %s, got %s", want, got)
	}
	if got := jw.PendingCount(); got != 0 {
		t.Fatalf("pending after successful forwards: want 0, got %d", got)
	}
}

func TestHandleConnForwardsUnknownWhenEnabled(t *testing.T) {
	var received atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/adapter/raw-frames":
			received.Add(1)
			w.WriteHeader(http.StatusOK)
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
		APIBaseURL:        api.URL,
		AdapterToken:      "test-token",
		ReadBufferSize:    64,
		ReadTimeout:       time.Second,
		RawDiskMaxUsedPct: 100,
	}, jw, NewLogger(io.Discard))

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(serverConn)
	}()

	if _, err := clientConn.Write([]byte{0x01, 0x02, 0x03, 0x04}); err != nil {
		t.Fatalf("write unknown: %v", err)
	}
	_ = clientConn.Close()
	<-done
	if result := srv.replayPendingBatch(t.Context()); result.failed != 0 {
		t.Fatal("replay unknown raw capture")
	}

	if received.Load() != 1 {
		t.Fatalf("raw frame posts: want 1, got %d", received.Load())
	}
}

func TestHandleConnDispatchesPendingCommandsAndMarksSent(t *testing.T) {
	var rawFrames atomic.Int32
	var outboundFrames atomic.Int32
	var pendingRequests atomic.Int32
	var sentRequests atomic.Int32
	sentCh := make(chan struct{}, 1)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/adapter/raw-frames":
			rawFrames.Add(1)
			var envelope RawFrameEnvelope
			if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
				t.Errorf("decode raw frame: %v", err)
			}
			if envelope.Direction == string(journal.DirectionOutbound) {
				outboundFrames.Add(1)
			}
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
							{"command_id": "9001", "command_hex": "7E09000103122501818B92"},
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
		RawDiskMaxUsedPct:   100,
	}, jw, NewLogger(io.Discard))

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(serverConn)
	}()

	if _, err := clientConn.Write([]byte("FP_TEST_ADMIN_001\nkeepalive")); err != nil {
		t.Fatalf("write coalesced register: %v", err)
	}

	received := make([]byte, 11)
	if err := clientConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if _, err := io.ReadFull(clientConn, received); err != nil {
		t.Fatalf("read pending command payload: %v", err)
	}
	if got := strings.ToUpper(hex.EncodeToString(received)); got != "7E09000103122501818B92" {
		t.Fatalf("pending command bytes: want complete verified frame, got %s", got)
	}

	select {
	case <-sentCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sent callback")
	}

	_ = clientConn.Close()
	<-done
	if result := srv.replayPendingBatch(t.Context()); result.failed != 0 {
		t.Fatal("replay inbound and outbound raw captures")
	}

	if sentRequests.Load() != 1 {
		t.Fatalf("sent requests: want 1, got %d", sentRequests.Load())
	}
	if rawFrames.Load() != 2 {
		t.Fatalf("raw frame posts including outbound command: want 2, got %d", rawFrames.Load())
	}
	if outboundFrames.Load() != 1 {
		t.Fatalf("outbound raw captures: want 1, got %d", outboundFrames.Load())
	}
}

func TestReplayPendingBatchForwardsAfterUpstreamRecovery(t *testing.T) {
	var available atomic.Bool
	var received atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/adapter/raw-frames" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if !available.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()

	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()
	frame := journal.Frame{
		MessageID:    "conn-replay-1",
		ConnectionID: "conn-replay",
		RemoteAddr:   "127.0.0.1:10001",
		Direction:    journal.DirectionInbound,
		PayloadHex:   "01020304",
		ReceivedAt:   time.Now().UTC(),
		RawSeq:       1,
	}
	if err := jw.Write(frame); err != nil {
		t.Fatalf("write pending frame: %v", err)
	}
	srv := NewServer(Config{APIBaseURL: api.URL, RawReplayBatch: 10}, jw, NewLogger(io.Discard))

	srv.replayPendingBatch(t.Context())
	if got := jw.PendingCount(); got != 1 {
		t.Fatalf("pending while upstream unavailable: want 1, got %d", got)
	}
	available.Store(true)
	srv.replayPendingBatch(t.Context())
	if got := jw.PendingCount(); got != 0 {
		t.Fatalf("pending after recovery: want 0, got %d", got)
	}
	if got := received.Load(); got != 1 {
		t.Fatalf("successful replay posts: want 1, got %d", got)
	}
}

func TestReplayPendingBatchDoesNotBlockFollowingCaptureAfterOneFailure(t *testing.T) {
	var accepted atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var envelope RawFrameEnvelope
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			t.Errorf("decode raw frame: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if envelope.CaptureID == "poison-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		accepted.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()

	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()
	for _, frame := range []journal.Frame{
		{MessageID: "poison-1", ConnectionID: "conn-replay", Direction: journal.DirectionInbound, PayloadHex: "01", RawSeq: 1},
		{MessageID: "valid-2", ConnectionID: "conn-replay", Direction: journal.DirectionInbound, PayloadHex: "02", RawSeq: 2},
	} {
		if err := jw.Write(frame); err != nil {
			t.Fatalf("write pending frame %s: %v", frame.MessageID, err)
		}
	}

	srv := NewServer(Config{APIBaseURL: api.URL, RawReplayBatch: 10}, jw, NewLogger(io.Discard))
	result := srv.replayPendingBatch(t.Context())
	if result.failed != 1 || result.forwarded != 1 {
		t.Fatalf("batch result: want one failure and one success, got %+v", result)
	}
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted captures after poison frame: want 1, got %d", got)
	}
	if got := jw.PendingCount(); got != 1 {
		t.Fatalf("pending captures after replay: want only poison frame, got %d", got)
	}
}

func TestReplayPendingFramesKeepsDrainingWhenEachBatchContainsPoisonCapture(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var envelope RawFrameEnvelope
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if envelope.CaptureID == "poison-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()

	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()
	if err := jw.Write(journal.Frame{MessageID: "poison-1", ConnectionID: "conn-replay", Direction: journal.DirectionInbound, PayloadHex: "01", RawSeq: 1}); err != nil {
		t.Fatalf("write poison frame: %v", err)
	}
	for i := 2; i <= 11; i++ {
		messageID := fmt.Sprintf("valid-%d", i)
		if err := jw.Write(journal.Frame{MessageID: messageID, ConnectionID: "conn-replay", Direction: journal.DirectionInbound, PayloadHex: "02", RawSeq: uint64(i)}); err != nil {
			t.Fatalf("write %s: %v", messageID, err)
		}
	}

	srv := NewServer(Config{APIBaseURL: api.URL, RawReplayBatch: 2, RawReplayInterval: 50 * time.Millisecond}, jw, NewLogger(io.Discard))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.replayPendingFrames(ctx)
	}()
	deadline := time.Now().Add(500 * time.Millisecond)
	for jw.PendingCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if got := jw.PendingCount(); got != 1 {
		t.Fatalf("pending captures after progressive replay: want only poison frame, got %d", got)
	}
}

func TestReadyzAcceptsTrafficWhenUpstreamIsTemporarilyUnavailable(t *testing.T) {
	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	srv := NewServer(Config{APIBaseURL: "http://127.0.0.1:1", RawDiskMaxUsedPct: 100}, jw, NewLogger(io.Discard))
	srv.listener = listener

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.healthRoutes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz while upstream unavailable: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"upstream_status":"unavailable"`) {
		t.Fatalf("readyz missing degraded upstream status: %s", rec.Body.String())
	}
}

func TestHealthzReportsCompletedPruneCheckEvenWhenNothingWasDeleted(t *testing.T) {
	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()
	srv := NewServer(Config{}, jw, NewLogger(io.Discard))
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	srv.pruneForwardedSegmentsOnce(now)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	srv.healthRoutes().ServeHTTP(recorder, req)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if got := int64(body["raw_last_prune_at_ms"].(float64)); got != now.UnixMilli() {
		t.Fatalf("last prune time: want %d, got %d", now.UnixMilli(), got)
	}
	if ok, _ := body["raw_last_prune_ok"].(bool); !ok {
		t.Fatalf("last prune status: want true, got %#v", body["raw_last_prune_ok"])
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
						{"command_id": "9002", "command_hex": "7E09000103122501818B92"},
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
		RawDiskMaxUsedPct:   100,
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

func TestHandleConnKeepsJournalingWhileUpstreamIsBlocked(t *testing.T) {
	releaseUpstream := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/adapter/raw-frames" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		<-releaseUpstream
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()
	defer close(releaseUpstream)

	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("new journal writer: %v", err)
	}
	defer jw.Close()

	srv := NewServer(Config{
		APIBaseURL:        api.URL,
		ReadBufferSize:    64,
		ReadTimeout:       time.Second,
		RawDiskMaxUsedPct: 100,
	}, jw, NewLogger(io.Discard))

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(serverConn)
	}()

	if err := clientConn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set first write deadline: %v", err)
	}
	if _, err := clientConn.Write([]byte{0x01}); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	if err := clientConn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set second write deadline: %v", err)
	}
	_, secondWriteErr := clientConn.Write([]byte{0x02})

	_ = clientConn.Close()
	<-done

	if secondWriteErr != nil {
		t.Fatalf("second TCP chunk must not wait for upstream delivery: %v", secondWriteErr)
	}
	if got := jw.PendingCount(); got != 2 {
		t.Fatalf("pending raw captures: want 2, got %d", got)
	}
}
