package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type Frame struct {
	ConnectionID string    `json:"connection_id"`
	GatewayName  string    `json:"gateway_name,omitempty"`
	RemoteAddr   string    `json:"remote_addr"`
	Direction    Direction `json:"direction"`
	PayloadHex   string    `json:"payload_hex"`
	ReceivedAt   time.Time `json:"received_at"`
	RawSeq       uint64    `json:"raw_seq"`
}

type Writer struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
	file     *os.File
	dateKey  string
	size     int64
}

func NewWriter(dir string, maxBytes int64) (*Writer, error) {
	if dir == "" {
		return nil, fmt.Errorf("raw journal dir is required")
	}
	if maxBytes <= 0 {
		maxBytes = 64 * 1024 * 1024
	}
	w := &Writer{dir: dir, maxBytes: maxBytes}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := w.rotateLocked(time.Now()); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) Write(frame Frame) error {
	if frame.ReceivedAt.IsZero() {
		frame.ReceivedAt = time.Now()
	}
	line, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.ensureOpenLocked(frame.ReceivedAt, int64(len(line))); err != nil {
		return err
	}
	n, err := w.file.Write(line)
	w.size += int64(n)
	return err
}

func (w *Writer) Probe() error {
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}
	probePath := filepath.Join(w.dir, ".probe")
	return os.WriteFile(probePath, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o644)
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *Writer) ensureOpenLocked(t time.Time, nextBytes int64) error {
	dateKey := t.Format("20060102")
	if w.file == nil || w.dateKey != dateKey || w.size+nextBytes > w.maxBytes {
		return w.rotateLocked(t)
	}
	return nil
}

func (w *Writer) rotateLocked(t time.Time) error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	w.dateKey = t.Format("20060102")
	name := filepath.Join(w.dir, fmt.Sprintf("raw-frames-%s.jsonl", w.dateKey))
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.size = st.Size()
	return nil
}
