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

	path := filepath.Join(dir, "raw-frames-20260705.jsonl")
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
}
