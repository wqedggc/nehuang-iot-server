package gatewayadapter

import (
	"context"
	"io"
	"nehuang-iot-server/internal/gatewayadapter/journal"
	"net"
	"testing"
	"time"
)

func TestAdapterCancellationClosesAlreadyConnectedTCP(t *testing.T) {
	probe, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := probe.Addr().String()
	probe.Close()
	jw, e := journal.NewWriter(t.TempDir(), 1024*1024)
	if e != nil {
		t.Fatal(e)
	}
	defer jw.Close()
	s := NewServer(Config{TCPAddr: addr, HealthAddr: "127.0.0.1:0", ReadTimeout: 2 * time.Minute, RawDiskMaxUsedPct: 100}, jw, NewLogger(io.Discard))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	var conn net.Conn
	for end := time.Now().Add(time.Second); time.Now().Before(end); {
		conn, e = net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if e == nil {
			break
		}
		time.Sleep(time.Millisecond * 5)
	}
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	for end := time.Now().Add(time.Second); s.activeConnections.Load() == 0 && time.Now().Before(end); {
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		conn.Close()
		<-done
		t.Fatal("cancel left an accepted TCP connection alive")
	}
	if s.activeConnections.Load() != 0 {
		t.Fatal("connection leak")
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 1)
	if _, e = conn.Read(b); e == nil {
		t.Fatal("client connection not closed")
	}
}
