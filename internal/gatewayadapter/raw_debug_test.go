package gatewayadapter

import (
	"context"
	"io"
	"nehuang-iot-server/internal/gatewayadapter/journal"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRawDebugClaimPreventsRetryAcrossConnectionState(t *testing.T) {
	var claims atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/adapter/commands/9009/claim":
			if claims.Add(1) > 1 {
				w.WriteHeader(409)
				return
			}
			w.WriteHeader(200)
		case "/api/v1/adapter/commands/9009/sent":
			w.WriteHeader(503)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer api.Close()
	jw, err := journal.NewWriter(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	s := NewServer(Config{APIBaseURL: api.URL, AdapterToken: "synthetic", CommandWriteTimeout: time.Second}, jw, NewLogger(io.Discard))
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	var writes atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 3)
		for {
			if _, err := io.ReadFull(client, buf); err != nil {
				return
			}
			writes.Add(1)
		}
	}()
	var seq atomic.Uint64
	command := []PendingCommand{{CommandID: 9009, CommandHex: "010203", SendOnce: true}}
	s.dispatchPendingCommands(context.Background(), server, "conn1", "FP_TEST", "TEST", "pipe", "", 0, &seq, command, map[uint64]bool{})
	// A fresh connection-local cache cannot repeat the durable claim/write.
	s.dispatchPendingCommands(context.Background(), server, "conn2", "FP_TEST", "TEST", "pipe", "", 0, &seq, command, map[uint64]bool{})
	client.Close()
	<-done
	if writes.Load() != 1 || claims.Load() != 2 || len(jw.Pending(10)) != 1 {
		t.Fatalf("writes %d claims %d", writes.Load(), claims.Load())
	}
}
func TestRawDebugDoesNotWriteWhenClaimResponseFails(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer api.Close()
	jw, _ := journal.NewWriter(t.TempDir(), 1024*1024)
	defer jw.Close()
	s := NewServer(Config{APIBaseURL: api.URL, AdapterToken: "synthetic"}, jw, NewLogger(io.Discard))
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	var seq atomic.Uint64
	s.dispatchPendingCommands(context.Background(), a, "conn", "FP_TEST", "TEST", "pipe", "", 0, &seq, []PendingCommand{{CommandID: 9, CommandHex: "010203", SendOnce: true}}, map[uint64]bool{})
	if len(jw.Pending(10)) != 0 {
		t.Fatal("write attempted without confirmed claim")
	}
}
