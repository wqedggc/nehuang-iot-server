package journal

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriterWritesJSONLFrame(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	ts := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	err = w.Write(Frame{
		MessageID:    "conn-1-7",
		ConnectionID: "conn-1",
		GatewayName:  "FP_TEST",
		RemoteAddr:   "127.0.0.1:1234",
		Direction:    DirectionInbound,
		PayloadHex:   "46505F54455354",
		ReceivedAt:   ts,
		RawSeq:       7,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	path := filepath.Join(dir, "raw-frames-20260705-000001.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatalf("expected one jsonl line")
	}
	var got Frame
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("json line: %v", err)
	}
	if got.ConnectionID != "conn-1" || got.PayloadHex != "46505F54455354" || got.RawSeq != 7 {
		t.Fatalf("unexpected frame: %+v", got)
	}
	if got.MessageID != "conn-1-7" {
		t.Fatalf("message id: want conn-1-7, got %q", got.MessageID)
	}
	if pending := w.Pending(10); len(pending) != 1 || pending[0].MessageID != "conn-1-7" {
		t.Fatalf("pending frames: %+v", pending)
	}
	if err := w.MarkForwarded("conn-1-7", ts.Add(time.Second)); err != nil {
		t.Fatalf("MarkForwarded: %v", err)
	}
	if got := w.PendingCount(); got != 0 {
		t.Fatalf("pending count after ack: want 0, got %d", got)
	}
}

func TestWriterRestoresPendingAndForwardedState(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	frames := []Frame{
		{MessageID: "conn-1-1", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "01", RawSeq: 1},
		{MessageID: "conn-1-2", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "02", RawSeq: 2},
	}
	for _, frame := range frames {
		if err := w.Write(frame); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := w.MarkForwarded("conn-1-1", time.Now().UTC()); err != nil {
		t.Fatalf("MarkForwarded: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewWriter(dir, 1024*1024)
	if err != nil {
		t.Fatalf("reopen NewWriter: %v", err)
	}
	defer reopened.Close()
	pending := reopened.Pending(10)
	if len(pending) != 1 || pending[0].MessageID != "conn-1-2" {
		t.Fatalf("restored pending frames: %+v", pending)
	}
}

func TestWriterRotatesToDistinctSegmentsAndRestrictsPermissions(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, 1)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()
	ts := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	for i := uint64(1); i <= 2; i++ {
		if err := w.Write(Frame{MessageID: "conn-1-" + string(rune('0'+i)), ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "01", ReceivedAt: ts, RawSeq: i}); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}
	paths, err := filepath.Glob(filepath.Join(dir, "raw-frames-20260705-*.jsonl"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("segments: want 2, got %v", paths)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("dir mode: want 0700, got %04o", got)
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("file mode %s: want 0600, got %04o", path, got)
		}
	}
}

func TestWriterIgnoresIncompleteTailAfterCrash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw-frames-20260705-000001.jsonl")
	complete := `{"message_id":"conn-1-1","connection_id":"conn-1","direction":"inbound","payload_hex":"01","received_at":"2026-07-05T10:00:00Z","raw_seq":1}` + "\n"
	if err := os.WriteFile(path, []byte(complete+`{"message_id":"partial`), 0o600); err != nil {
		t.Fatalf("write crash fixture: %v", err)
	}
	w, err := NewWriter(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewWriter should recover complete records: %v", err)
	}
	defer w.Close()
	pending := w.Pending(10)
	if len(pending) != 1 || pending[0].MessageID != "conn-1-1" {
		t.Fatalf("pending after incomplete tail: %+v", pending)
	}
}
