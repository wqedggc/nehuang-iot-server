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

func TestWriterRejectsMessageIDAlreadyForwarded(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()
	first := Frame{MessageID: "capture-1", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "01", RawSeq: 1}
	if err := w.Write(first); err != nil {
		t.Fatalf("Write first capture: %v", err)
	}
	if err := w.MarkForwarded(first.MessageID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkForwarded: %v", err)
	}
	duplicate := first
	duplicate.PayloadHex = "02"
	if err := w.Write(duplicate); err == nil {
		t.Fatal("reusing a forwarded message_id must be rejected")
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

func TestPruneForwardedSegmentsDeletesOnlyClosedFullyAcknowledgedOldSegments(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, 1)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	old := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	frames := []Frame{
		{MessageID: "acked-closed", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "01", ReceivedAt: old, RawSeq: 1},
		{MessageID: "pending-closed", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "02", ReceivedAt: old, RawSeq: 2},
		{MessageID: "acked-current", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "03", ReceivedAt: old, RawSeq: 3},
	}
	for _, frame := range frames {
		if err := w.Write(frame); err != nil {
			t.Fatalf("Write %s: %v", frame.MessageID, err)
		}
	}
	for _, messageID := range []string{"acked-closed", "acked-current"} {
		if err := w.MarkForwarded(messageID, old.Add(time.Minute)); err != nil {
			t.Fatalf("MarkForwarded %s: %v", messageID, err)
		}
	}

	result, err := w.PruneForwardedSegments(old.Add(24 * time.Hour))
	if err != nil {
		t.Fatalf("PruneForwardedSegments: %v", err)
	}
	if result.Segments != 1 {
		t.Fatalf("pruned segments: want 1, got %+v", result)
	}
	if result.Bytes == 0 {
		t.Fatalf("pruned bytes: want > 0, got %+v", result)
	}

	assertPathMissing(t, filepath.Join(dir, "raw-frames-20260705-000001.jsonl"))
	assertPathExists(t, filepath.Join(dir, "raw-frames-20260705-000002.jsonl"))
	assertPathExists(t, filepath.Join(dir, "raw-frames-20260705-000003.jsonl"))
	if pending := w.Pending(10); len(pending) != 1 || pending[0].MessageID != "pending-closed" {
		t.Fatalf("pending frames after prune: %+v", pending)
	}
}

func TestPruneForwardedSegmentsUsesDurableAcknowledgementsAfterRestart(t *testing.T) {
	dir := t.TempDir()
	old := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	w, err := NewWriter(dir, 1)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := w.Write(Frame{MessageID: "acked-before-restart", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "01", ReceivedAt: old, RawSeq: 1}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.MarkForwarded("acked-before-restart", old.Add(time.Minute)); err != nil {
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
	result, err := reopened.PruneForwardedSegments(old.Add(24 * time.Hour))
	if err != nil {
		t.Fatalf("PruneForwardedSegments: %v", err)
	}
	if result.Segments != 1 {
		t.Fatalf("pruned segments after restart: want 1, got %+v", result)
	}
	if result.AckFiles != 1 {
		t.Fatalf("pruned acknowledgement files after restart: want 1, got %+v", result)
	}
	assertPathMissing(t, filepath.Join(dir, "raw-frames-20260705-000001.jsonl"))
	assertPathMissing(t, filepath.Join(dir, "raw-forwarded-20260705.jsonl"))
}

func TestPruneForwardedSegmentsRetainsMalformedSegmentWithoutBlockingSafeCleanup(t *testing.T) {
	dir := t.TempDir()
	old := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	w, err := NewWriter(dir, 1)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()
	if err := w.Write(Frame{MessageID: "safe-acked", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "01", ReceivedAt: old, RawSeq: 1}); err != nil {
		t.Fatalf("Write safe frame: %v", err)
	}
	if err := w.Write(Frame{MessageID: "keep-current", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "02", ReceivedAt: old, RawSeq: 2}); err != nil {
		t.Fatalf("Write current frame: %v", err)
	}
	if err := w.MarkForwarded("safe-acked", old.Add(time.Minute)); err != nil {
		t.Fatalf("MarkForwarded: %v", err)
	}
	malformedPath := filepath.Join(dir, "raw-frames-20260704-000001.jsonl")
	if err := os.WriteFile(malformedPath, []byte("not-json\n"), 0o600); err != nil {
		t.Fatalf("write malformed segment: %v", err)
	}

	result, err := w.PruneForwardedSegments(old.Add(24 * time.Hour))
	if err == nil {
		t.Fatal("malformed segment must report a prune failure")
	}
	if result.Segments != 1 {
		t.Fatalf("safe cleanup result: want 1 segment, got %+v", result)
	}
	assertPathMissing(t, filepath.Join(dir, "raw-frames-20260705-000001.jsonl"))
	assertPathExists(t, malformedPath)
}

func TestPruneForwardedSegmentsRetainsIncompleteTail(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()
	path := filepath.Join(dir, "raw-frames-20260704-000001.jsonl")
	frame := Frame{MessageID: "truncated-1", ConnectionID: "conn-1", Direction: DirectionInbound, PayloadHex: "01", ReceivedAt: time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC), RawSeq: 1}
	line, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	if err := os.WriteFile(path, line, 0o600); err != nil {
		t.Fatalf("write incomplete segment: %v", err)
	}

	if _, err := w.PruneForwardedSegments(time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("incomplete segment must report a prune failure")
	}
	assertPathExists(t, path)
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err=%v", path, err)
	}
}
