package gatewayadapter

import (
	"context"
	"io"
	"nehuang-iot-server/internal/gatewayadapter/journal"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type policyWriteConn struct {
	net.Conn
	writes int
}

func (c *policyWriteConn) Write(b []byte) (int, error)      { c.writes++; return len(b), nil }
func (c *policyWriteConn) SetWriteDeadline(time.Time) error { return nil }
func TestUnsafeStoredCommandsNeverClaimJournalOrWrite(t *testing.T) {
	var result, claim, sent atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/result"):
			result.Add(1)
		case strings.HasSuffix(r.URL.Path, "/claim"):
			claim.Add(1)
		case strings.HasSuffix(r.URL.Path, "/sent"):
			sent.Add(1)
		}
		w.WriteHeader(200)
	}))
	defer api.Close()
	jw, e := journal.NewWriter(t.TempDir(), 1024*1024)
	if e != nil {
		t.Fatal(e)
	}
	defer jw.Close()
	s := NewServer(Config{APIBaseURL: api.URL}, jw, NewLogger(io.Discard))
	conn := &policyWriteConn{}
	var seq atomic.Uint64
	bad := []string{"0100020C0000C071", "7E080100020C0000C071", "0100020C0000C0710100030C0000004C90", "7E080100020C0000C0717E090100030C0000004C90", "7E09000103122501818B9200"}
	for _, once := range []bool{false, true} {
		for i, h := range bad {
			s.dispatchPendingCommands(context.Background(), conn, "synthetic", "FP_TEST", "TEST", "local", "", 0, &seq, []PendingCommand{{CommandID: uint64(i + 1), CommandHex: h, SendOnce: once}}, map[uint64]bool{})
		}
	}
	if conn.writes != 0 || claim.Load() != 0 || sent.Load() != 0 || len(jw.Pending(10)) != 0 || result.Load() != 10 {
		t.Fatalf("writes %d claim %d sent %d rejection %d", conn.writes, claim.Load(), sent.Load(), result.Load())
	}
}
func TestExistingSingleByteActionsStillWrite(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sent") {
			t.Errorf("unexpected %s", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer api.Close()
	jw, e := journal.NewWriter(t.TempDir(), 1024*1024)
	if e != nil {
		t.Fatal(e)
	}
	defer jw.Close()
	s := NewServer(Config{APIBaseURL: api.URL}, jw, NewLogger(io.Discard))
	conn := &policyWriteConn{}
	var seq atomic.Uint64
	for i, h := range []string{"F0", "0F", "00"} {
		s.dispatchPendingCommands(context.Background(), conn, "synthetic", "FP_TEST", "TEST", "local", "", 0, &seq, []PendingCommand{{CommandID: uint64(i + 1), CommandHex: h}}, map[uint64]bool{})
	}
	if conn.writes != 3 {
		t.Fatal("existing actions rejected")
	}
}
