package gatewayadapter

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"nehuang-iot-server/internal/gatewayadapter/journal"
)

func TestPendingCommandRetriesSentWithoutRepeatingTCPWrite(t *testing.T) {
	var sentAttempts atomic.Int32
	var tcpWrites atomic.Int32
	confirmed := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/adapter/commands/pending":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"items": []map[string]any{{"command_id": "9002", "command_hex": "7E09000103122501818B92"}}}})
		case "/api/v1/adapter/commands/9002/sent":
			attempt := sentAttempts.Add(1)
			if attempt < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			if attempt == 3 {
				close(confirmed)
			}
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer api.Close()
	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	srv := NewServer(Config{APIBaseURL: api.URL, AdapterToken: "test-token", CommandPollInterval: time.Millisecond, CommandWriteTimeout: time.Second, RawDiskMaxUsedPct: 100}, jw, NewLogger(io.Discard))
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			var payload [11]byte
			if _, err := io.ReadFull(clientConn, payload[:]); err != nil {
				return
			}
			tcpWrites.Add(1)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		var seq atomic.Uint64
		srv.pollPendingCommands(ctx, serverConn, "conn-test", "FP_TEST", "TEST", "pipe", "", 0, &seq)
	}()
	select {
	case <-confirmed:
	case <-time.After(3 * time.Second):
		t.Fatal("sent state did not recover")
	}
	cancel()
	<-loopDone
	_ = clientConn.Close()
	<-readerDone
	if got := tcpWrites.Load(); got != 1 {
		t.Fatalf("TCP writes after sent response failures: want 1, got %d", got)
	}
	if got := len(jw.Pending(10)); got != 1 {
		t.Fatalf("outbound write-attempt journal records: want 1, got %d", got)
	}
}
