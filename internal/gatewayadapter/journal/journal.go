package journal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type Frame struct {
	MessageID    string    `json:"message_id"`
	CaptureKind  string    `json:"capture_kind"`
	ConnectionID string    `json:"connection_id"`
	GatewayName  string    `json:"gateway_name,omitempty"`
	RemoteAddr   string    `json:"remote_addr"`
	Direction    Direction `json:"direction"`
	PayloadHex   string    `json:"payload_hex"`
	ReceivedAt   time.Time `json:"received_at"`
	RawSeq       uint64    `json:"raw_seq"`
}

type ForwardAck struct {
	MessageID   string    `json:"message_id"`
	ForwardedAt time.Time `json:"forwarded_at"`
}

type DiskUsage struct {
	TotalBytes     uint64
	AvailableBytes uint64
	UsedPercent    float64
}

type PruneResult struct {
	Segments int
	Bytes    int64
	AckFiles int
	AckBytes int64
}

type Writer struct {
	mu        sync.Mutex
	dir       string
	maxBytes  int64
	file      *os.File
	dateKey   string
	segment   int
	size      int64
	pending   map[string]Frame
	known     map[string]struct{}
	forwarded map[string]int
	order     []string
	head      int
}

func NewWriter(dir string, maxBytes int64) (*Writer, error) {
	if dir == "" {
		return nil, fmt.Errorf("raw journal dir is required")
	}
	if maxBytes <= 0 {
		maxBytes = 64 * 1024 * 1024
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	w := &Writer{
		dir:       dir,
		maxBytes:  maxBytes,
		pending:   make(map[string]Frame),
		known:     make(map[string]struct{}),
		forwarded: make(map[string]int),
	}
	if err := w.loadPendingLocked(); err != nil {
		return nil, err
	}
	if err := w.rotateLocked(time.Now(), false); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) Write(frame Frame) error {
	if frame.ReceivedAt.IsZero() {
		frame.ReceivedAt = time.Now().UTC()
	}
	if frame.MessageID == "" {
		frame.MessageID = legacyMessageID(frame)
	}
	if frame.CaptureKind == "" {
		frame.CaptureKind = captureKind(frame.Direction)
	}
	line, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.known[frame.MessageID]; exists {
		return fmt.Errorf("raw journal duplicate message_id %q", frame.MessageID)
	}
	if w.forwarded[frame.MessageID] > 0 {
		return fmt.Errorf("raw journal message_id %q was already forwarded", frame.MessageID)
	}
	if err := w.ensureOpenLocked(frame.ReceivedAt, int64(len(line))); err != nil {
		return err
	}
	n, err := w.file.Write(line)
	w.size += int64(n)
	if err != nil {
		return err
	}
	if n != len(line) {
		return fmt.Errorf("raw journal short write: wrote %d of %d bytes", n, len(line))
	}
	if err := w.file.Sync(); err != nil {
		return err
	}
	w.known[frame.MessageID] = struct{}{}
	w.order = append(w.order, frame.MessageID)
	w.pending[frame.MessageID] = frame
	return nil
}

func (w *Writer) MarkForwarded(messageID string, forwardedAt time.Time) error {
	if strings.TrimSpace(messageID) == "" {
		return fmt.Errorf("message_id is required")
	}
	if forwardedAt.IsZero() {
		forwardedAt = time.Now().UTC()
	}
	line, err := json.Marshal(ForwardAck{MessageID: messageID, ForwardedAt: forwardedAt})
	if err != nil {
		return err
	}
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.pending[messageID]; !exists {
		return fmt.Errorf("raw journal message_id %q is not pending", messageID)
	}
	path := filepath.Join(w.dir, fmt.Sprintf("raw-forwarded-%s.jsonl", forwardedAt.Format("20060102")))
	created, err := pathDoesNotExist(path)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = f.Close()
		return err
	}
	n, writeErr := f.Write(line)
	if writeErr == nil && n != len(line) {
		writeErr = fmt.Errorf("raw forward ack short write: wrote %d of %d bytes", n, len(line))
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if created {
		if err := syncDirectory(w.dir); err != nil {
			return err
		}
	}
	w.forwarded[messageID]++
	delete(w.pending, messageID)
	w.compactOrderLocked()
	return nil
}

// PruneForwardedSegments removes closed raw-frame segments whose records are
// older than cutoff and all have durable forward acknowledgements. The active
// segment, empty segments, malformed segments, and segments containing any
// unacknowledged record are retained.
func (w *Writer) PruneForwardedSegments(cutoff time.Time) (PruneResult, error) {
	if cutoff.IsZero() {
		return PruneResult{}, fmt.Errorf("prune cutoff is required")
	}

	w.mu.Lock()
	activePath := w.activePathLocked()
	forwarded := make(map[string]struct{}, len(w.forwarded))
	for messageID, count := range w.forwarded {
		if count <= 0 {
			continue
		}
		forwarded[messageID] = struct{}{}
	}
	w.mu.Unlock()

	paths, err := filepath.Glob(filepath.Join(w.dir, "raw-frames-*.jsonl"))
	if err != nil {
		return PruneResult{}, err
	}
	sort.Strings(paths)
	type candidate struct {
		path       string
		info       os.FileInfo
		messageIDs []string
	}
	candidates := make([]candidate, 0, len(paths))
	var firstErr error
	for _, path := range paths {
		if filepath.Clean(path) == activePath {
			continue
		}
		info, messageIDs, eligible, err := inspectPrunableSegment(path, cutoff, forwarded)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("inspect raw journal segment %s: %w", path, err)
			}
			continue
		}
		if eligible {
			candidates = append(candidates, candidate{path: path, info: info, messageIDs: messageIDs})
		}
	}

	var result PruneResult
	var removedRawIDs []string
	for _, candidate := range candidates {
		w.mu.Lock()
		if filepath.Clean(candidate.path) == w.activePathLocked() {
			w.mu.Unlock()
			continue
		}
		info, statErr := os.Lstat(candidate.path)
		if statErr == nil && (!info.Mode().IsRegular() || info.Size() != candidate.info.Size() || !os.SameFile(candidate.info, info)) {
			w.mu.Unlock()
			continue
		}
		if statErr == nil {
			statErr = os.Remove(candidate.path)
		}
		w.mu.Unlock()
		if statErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("remove raw journal segment %s: %w", candidate.path, statErr)
			}
			continue
		}
		result.Segments++
		result.Bytes += candidate.info.Size()
		removedRawIDs = append(removedRawIDs, candidate.messageIDs...)
	}

	w.mu.Lock()
	remainingRaw := make(map[string]struct{}, len(w.known))
	for messageID := range w.known {
		remainingRaw[messageID] = struct{}{}
	}
	for _, messageID := range removedRawIDs {
		delete(remainingRaw, messageID)
	}
	w.mu.Unlock()

	ackPaths, err := filepath.Glob(filepath.Join(w.dir, "raw-forwarded-*.jsonl"))
	if err != nil && firstErr == nil {
		firstErr = err
	}
	sort.Strings(ackPaths)
	ackCandidates := make([]candidate, 0, len(ackPaths))
	for _, path := range ackPaths {
		info, messageIDs, eligible, err := inspectPrunableAckFile(path, cutoff, remainingRaw)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("inspect raw forward acknowledgements %s: %w", path, err)
			}
			continue
		}
		if eligible {
			ackCandidates = append(ackCandidates, candidate{path: path, info: info, messageIDs: messageIDs})
		}
	}

	var removedAckIDs []string
	for _, candidate := range ackCandidates {
		w.mu.Lock()
		info, statErr := os.Lstat(candidate.path)
		if statErr == nil && (!info.Mode().IsRegular() || info.Size() != candidate.info.Size() || !os.SameFile(candidate.info, info)) {
			w.mu.Unlock()
			continue
		}
		if statErr == nil {
			statErr = os.Remove(candidate.path)
		}
		w.mu.Unlock()
		if statErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("remove raw forward acknowledgements %s: %w", candidate.path, statErr)
			}
			continue
		}
		result.AckFiles++
		result.AckBytes += candidate.info.Size()
		removedAckIDs = append(removedAckIDs, candidate.messageIDs...)
	}

	if result.Segments == 0 && result.AckFiles == 0 {
		return result, firstErr
	}
	if err := syncDirectory(w.dir); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		return result, firstErr
	}
	w.mu.Lock()
	for _, messageID := range removedRawIDs {
		delete(w.known, messageID)
	}
	for _, messageID := range removedAckIDs {
		if w.forwarded[messageID] <= 1 {
			delete(w.forwarded, messageID)
		} else {
			w.forwarded[messageID]--
		}
	}
	w.mu.Unlock()
	return result, firstErr
}

func (w *Writer) Pending(limit int) []Frame {
	w.mu.Lock()
	defer w.mu.Unlock()
	if limit <= 0 || limit > len(w.pending) {
		limit = len(w.pending)
	}
	frames := make([]Frame, 0, limit)
	for _, messageID := range w.order[w.head:] {
		frame, ok := w.pending[messageID]
		if !ok {
			continue
		}
		frames = append(frames, frame)
		if len(frames) == limit {
			break
		}
	}
	w.compactOrderLocked()
	return frames
}

func (w *Writer) PendingCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.pending)
}

func (w *Writer) DiskUsage() (DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(w.dir, &stat); err != nil {
		return DiskUsage{}, err
	}
	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	available := uint64(stat.Bavail) * uint64(stat.Bsize)
	usage := DiskUsage{TotalBytes: total, AvailableBytes: available}
	if total > 0 {
		usage.UsedPercent = float64(total-available) * 100 / float64(total)
	}
	return usage, nil
}

func (w *Writer) Probe() error {
	if err := os.MkdirAll(w.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(w.dir, 0o700); err != nil {
		return err
	}
	probePath := filepath.Join(w.dir, ".probe")
	f, err := os.OpenFile(probePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.WriteString(time.Now().UTC().Format(time.RFC3339Nano)); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return err
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
	if w.file == nil || w.dateKey != dateKey {
		return w.rotateLocked(t, false)
	}
	if w.size+nextBytes > w.maxBytes {
		return w.rotateLocked(t, true)
	}
	return nil
}

func (w *Writer) rotateLocked(t time.Time, nextSegment bool) error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	dateKey := t.Format("20060102")
	segment := 1
	if w.dateKey == dateKey && nextSegment {
		segment = w.segment + 1
	} else if existing := latestSegment(w.dir, dateKey); existing > 0 {
		segment = existing
	}
	name := filepath.Join(w.dir, fmt.Sprintf("raw-frames-%s-%06d.jsonl", dateKey, segment))
	created, err := pathDoesNotExist(name)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		_ = f.Close()
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.dateKey = dateKey
	w.segment = segment
	w.size = st.Size()
	if created {
		if err := syncDirectory(w.dir); err != nil {
			_ = f.Close()
			w.file = nil
			return err
		}
	}
	return nil
}

func (w *Writer) activePathLocked() string {
	if w.file == nil {
		return ""
	}
	return filepath.Clean(w.file.Name())
}

func (w *Writer) loadPendingLocked() error {
	acked := make(map[string]struct{})
	ackFiles, err := filepath.Glob(filepath.Join(w.dir, "raw-forwarded-*.jsonl"))
	if err != nil {
		return err
	}
	sort.Strings(ackFiles)
	for _, path := range ackFiles {
		if err := repairIncompleteTail(path); err != nil {
			return fmt.Errorf("repair forward acknowledgements %s: %w", path, err)
		}
		if err := scanJSONL(path, func(line []byte) error {
			var ack ForwardAck
			if err := json.Unmarshal(line, &ack); err != nil {
				return err
			}
			if ack.MessageID != "" {
				acked[ack.MessageID] = struct{}{}
				w.forwarded[ack.MessageID]++
			}
			return nil
		}); err != nil {
			return fmt.Errorf("load forward acknowledgements %s: %w", path, err)
		}
	}

	rawFiles, err := filepath.Glob(filepath.Join(w.dir, "raw-frames-*.jsonl"))
	if err != nil {
		return err
	}
	sort.Strings(rawFiles)
	for _, path := range rawFiles {
		if err := repairIncompleteTail(path); err != nil {
			return fmt.Errorf("repair raw journal %s: %w", path, err)
		}
		if err := scanJSONL(path, func(line []byte) error {
			var frame Frame
			if err := json.Unmarshal(line, &frame); err != nil {
				return err
			}
			if frame.MessageID == "" {
				frame.MessageID = legacyMessageID(frame)
			}
			if frame.CaptureKind == "" {
				frame.CaptureKind = captureKind(frame.Direction)
			}
			if _, exists := w.known[frame.MessageID]; exists {
				return fmt.Errorf("duplicate raw journal message_id %q", frame.MessageID)
			}
			w.known[frame.MessageID] = struct{}{}
			if _, ok := acked[frame.MessageID]; ok {
				return nil
			}
			w.order = append(w.order, frame.MessageID)
			w.pending[frame.MessageID] = frame
			return nil
		}); err != nil {
			return fmt.Errorf("load raw journal %s: %w", path, err)
		}
	}
	return nil
}

func inspectPrunableSegment(path string, cutoff time.Time, forwarded map[string]struct{}) (os.FileInfo, []string, bool, error) {
	info, err := inspectJSONLFile(path)
	if err != nil {
		return nil, nil, false, err
	}
	messageIDs := make([]string, 0)
	eligible := true
	if err := scanJSONL(path, func(line []byte) error {
		var frame Frame
		if err := json.Unmarshal(line, &frame); err != nil {
			return err
		}
		if frame.MessageID == "" {
			frame.MessageID = legacyMessageID(frame)
		}
		messageIDs = append(messageIDs, frame.MessageID)
		if frame.ReceivedAt.IsZero() || !frame.ReceivedAt.Before(cutoff) {
			eligible = false
		}
		if _, ok := forwarded[frame.MessageID]; !ok {
			eligible = false
		}
		return nil
	}); err != nil {
		return nil, nil, false, err
	}
	return info, messageIDs, len(messageIDs) > 0 && eligible, nil
}

func inspectPrunableAckFile(path string, cutoff time.Time, remainingRaw map[string]struct{}) (os.FileInfo, []string, bool, error) {
	info, err := inspectJSONLFile(path)
	if err != nil {
		return nil, nil, false, err
	}
	messageIDs := make([]string, 0)
	eligible := true
	if err := scanJSONL(path, func(line []byte) error {
		var ack ForwardAck
		if err := json.Unmarshal(line, &ack); err != nil {
			return err
		}
		if strings.TrimSpace(ack.MessageID) == "" {
			return fmt.Errorf("forward acknowledgement message_id is required")
		}
		messageIDs = append(messageIDs, ack.MessageID)
		if ack.ForwardedAt.IsZero() || !ack.ForwardedAt.Before(cutoff) {
			eligible = false
		}
		if _, exists := remainingRaw[ack.MessageID]; exists {
			eligible = false
		}
		return nil
	}); err != nil {
		return nil, nil, false, err
	}
	return info, messageIDs, len(messageIDs) > 0 && eligible, nil
}

func inspectJSONLFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() == 0 {
		return info, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var last [1]byte
	_, readErr := f.ReadAt(last[:], info.Size()-1)
	closeErr := f.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if last[0] != '\n' {
		return nil, fmt.Errorf("incomplete jsonl tail")
	}
	return info, nil
}

func (w *Writer) compactOrderLocked() {
	for w.head < len(w.order) {
		if _, ok := w.pending[w.order[w.head]]; ok {
			break
		}
		w.head++
	}
	if w.head >= 1024 && w.head*2 >= len(w.order) {
		w.order = append([]string(nil), w.order[w.head:]...)
		w.head = 0
	}
}

func repairIncompleteTail(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() == 0 {
		return err
	}
	var last [1]byte
	if _, err := f.ReadAt(last[:], st.Size()-1); err != nil {
		return err
	}
	if last[0] == '\n' {
		return nil
	}
	const chunkSize int64 = 4096
	for end := st.Size(); end > 0; {
		start := end - chunkSize
		if start < 0 {
			start = 0
		}
		buf := make([]byte, end-start)
		if _, err := f.ReadAt(buf, start); err != nil {
			return err
		}
		if index := bytes.LastIndexByte(buf, '\n'); index >= 0 {
			return f.Truncate(start + int64(index) + 1)
		}
		end = start
	}
	return f.Truncate(0)
}

func scanJSONL(path string, visit func([]byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	endsWithNewline := true
	if st, statErr := f.Stat(); statErr != nil {
		return statErr
	} else if st.Size() > 0 {
		var last [1]byte
		if _, readErr := f.ReadAt(last[:], st.Size()-1); readErr != nil {
			return readErr
		}
		endsWithNewline = last[0] == '\n'
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var previous []byte
	for scanner.Scan() {
		if previous != nil {
			if err := visitJSONLLine(previous, visit); err != nil {
				return err
			}
		}
		previous = append(previous[:0], scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if endsWithNewline && previous != nil {
		if err := visitJSONLLine(previous, visit); err != nil {
			return err
		}
	}
	return nil
}

func visitJSONLLine(line []byte, visit func([]byte) error) error {
	if len(strings.TrimSpace(string(line))) == 0 {
		return nil
	}
	return visit(line)
}

func latestSegment(dir, dateKey string) int {
	paths, _ := filepath.Glob(filepath.Join(dir, fmt.Sprintf("raw-frames-%s-*.jsonl", dateKey)))
	latest := 0
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		parts := strings.Split(name, "-")
		if len(parts) == 4 {
			if segment, err := strconv.Atoi(parts[3]); err == nil && segment > latest {
				latest = segment
			}
		}
	}
	return latest
}

func legacyMessageID(frame Frame) string {
	return frame.ConnectionID + "-" + strconv.FormatUint(frame.RawSeq, 10)
}

func captureKind(direction Direction) string {
	if direction == DirectionOutbound {
		return "tcp_write_attempt"
	}
	return "tcp_read"
}

func pathDoesNotExist(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return false, nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, err
}

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open raw journal directory for sync: %w", err)
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if syncErr != nil {
		return fmt.Errorf("sync raw journal directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close raw journal directory: %w", closeErr)
	}
	return nil
}
