package gatewayadapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"nehuang-iot-server/internal/controlpolicy"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nehuang-iot-server/internal/gatewayadapter/journal"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

type Config struct {
	TCPAddr             string
	HealthAddr          string
	APIBaseURL          string
	AdapterToken        string
	RawLogDir           string
	Debug               bool
	ReadBufferSize      int
	ReadTimeout         time.Duration
	CommandPollInterval time.Duration
	CommandPollLimit    int
	CommandWriteTimeout time.Duration
	RawReplayInterval   time.Duration
	RawReplayBatch      int
	RawDiskMaxUsedPct   float64
	RawRetention        time.Duration
	RawPruneInterval    time.Duration
}

type Server struct {
	cfg       Config
	journal   *journal.Writer
	logger    *Logger
	forwarder *Forwarder

	listener    net.Listener
	connMu      sync.Mutex
	connections map[net.Conn]context.CancelFunc
	wg          sync.WaitGroup
	closing     atomic.Bool
	pruneNow    chan struct{}

	activeConnections atomic.Int64
	rawFrames         atomic.Uint64
	rawWriteFailures  atomic.Uint64
	forwardFailures   atomic.Uint64
	rawPruneFailures  atomic.Uint64
	rawPrunedSegments atomic.Uint64
	rawPrunedBytes    atomic.Uint64
	rawPrunedAckFiles atomic.Uint64
	rawPrunedAckBytes atomic.Uint64
	rawLastPruneAtMS  atomic.Int64
	rawLastPruneOK    atomic.Bool
}

func NewServer(cfg Config, jw *journal.Writer, logger *Logger) *Server {
	if cfg.TCPAddr == "" {
		cfg.TCPAddr = ":8081"
	}
	if cfg.HealthAddr == "" {
		cfg.HealthAddr = ":18081"
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = "http://127.0.0.1:8080"
	}
	if cfg.ReadBufferSize <= 0 {
		cfg.ReadBufferSize = 4096
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 120 * time.Second
	}
	if cfg.CommandPollInterval <= 0 {
		cfg.CommandPollInterval = 2 * time.Second
	}
	if cfg.CommandPollLimit <= 0 {
		cfg.CommandPollLimit = 20
	}
	if cfg.CommandWriteTimeout <= 0 {
		cfg.CommandWriteTimeout = 5 * time.Second
	}
	if cfg.RawReplayInterval <= 0 {
		cfg.RawReplayInterval = 5 * time.Second
	}
	if cfg.RawReplayBatch <= 0 {
		cfg.RawReplayBatch = 100
	}
	if cfg.RawDiskMaxUsedPct <= 0 {
		cfg.RawDiskMaxUsedPct = 90
	}
	if cfg.RawRetention <= 0 {
		cfg.RawRetention = 14 * 24 * time.Hour
	}
	if cfg.RawPruneInterval <= 0 {
		cfg.RawPruneInterval = time.Hour
	}
	if logger == nil {
		logger = NewLogger(nil)
	}
	return &Server{
		cfg:         cfg,
		journal:     jw,
		logger:      logger,
		forwarder:   NewForwarder(cfg.APIBaseURL, cfg.AdapterToken),
		pruneNow:    make(chan struct{}, 1),
		connections: make(map[net.Conn]context.CancelFunc),
	}
}

func (s *Server) Run(ctx context.Context) error {
	s.pruneForwardedSegmentsOnce(time.Now().UTC())
	ln, err := net.Listen("tcp", s.cfg.TCPAddr)
	if err != nil {
		return err
	}
	s.listener = ln
	s.logger.Info("gateway_adapter_started", map[string]any{
		"tcp_addr":               s.cfg.TCPAddr,
		"health_addr":            s.cfg.HealthAddr,
		"raw_log_dir":            s.cfg.RawLogDir,
		"raw_forward_policy":     "all",
		"raw_replay_interval_ms": s.cfg.RawReplayInterval.Milliseconds(),
		"raw_replay_batch":       s.cfg.RawReplayBatch,
		"raw_retention_hours":    s.cfg.RawRetention.Hours(),
		"raw_prune_interval_ms":  s.cfg.RawPruneInterval.Milliseconds(),
	})

	replayCtx, replayCancel := context.WithCancel(ctx)
	defer replayCancel()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.replayPendingFrames(replayCtx)
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.pruneForwardedSegments(replayCtx)
	}()

	healthSrv := &http.Server{
		Addr:              s.cfg.HealthAddr,
		Handler:           s.healthRoutes(),
		ReadHeaderTimeout: 3 * time.Second,
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := healthSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("gateway_adapter_health_server_failed", map[string]any{"error": err.Error()})
		}
	}()

	errCh := make(chan error, 1)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		errCh <- s.acceptLoop()
	}()

	select {
	case <-ctx.Done():
		s.closing.Store(true)
		replayCancel()
		_ = ln.Close()
		s.closeConnections()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = healthSrv.Shutdown(shutdownCtx)
		s.wg.Wait()
		return nil
	case err := <-errCh:
		s.closing.Store(true)
		replayCancel()
		_ = ln.Close()
		s.closeConnections()
		_ = healthSrv.Close()
		s.wg.Wait()
		return err
	}
}

func (s *Server) acceptLoop() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closing.Load() {
				return nil
			}
			return err
		}
		connCtx, cancel := context.WithCancel(context.Background())
		s.connMu.Lock()
		if s.closing.Load() {
			s.connMu.Unlock()
			cancel()
			conn.Close()
			return nil
		}
		s.connections[conn] = cancel
		s.wg.Add(1)
		s.connMu.Unlock()
		go func(c net.Conn, ctx context.Context, cancel context.CancelFunc) {
			defer s.wg.Done()
			defer func() { s.connMu.Lock(); delete(s.connections, c); s.connMu.Unlock() }()
			s.handleConnContext(c, ctx, cancel)
		}(conn, connCtx, cancel)
	}
}

func (s *Server) closeConnections() {
	s.connMu.Lock()
	peers := make(map[net.Conn]context.CancelFunc, len(s.connections))
	for c, cancel := range s.connections {
		peers[c] = cancel
	}
	s.connMu.Unlock()
	for c, cancel := range peers {
		cancel()
		_ = c.Close()
	}
}
func (s *Server) handleConn(conn net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	s.handleConnContext(conn, ctx, cancel)
}
func (s *Server) handleConnContext(conn net.Conn, connCtx context.Context, cancel context.CancelFunc) {
	defer conn.Close()
	s.activeConnections.Add(1)
	defer s.activeConnections.Add(-1)

	connectionID := newConnectionID()
	remoteAddr := conn.RemoteAddr().String()
	remoteIP, remotePort := splitRemote(remoteAddr)
	var gatewayName string
	var gatewayExternalID string
	var rawSeq atomic.Uint64
	defer cancel()
	var startCommandPolling sync.Once

	s.logger.Info("gateway_connection_opened", map[string]any{
		"connection_id": connectionID,
		"remote_addr":   remoteAddr,
	})
	defer func() {
		s.logger.Info("gateway_connection_closed", map[string]any{
			"connection_id": connectionID,
			"gateway_name":  gatewayName,
			"remote_addr":   remoteAddr,
		})
	}()

	buf := make([]byte, s.cfg.ReadBufferSize)
	for {
		if disk, err := s.journal.DiskUsage(); err != nil || disk.UsedPercent >= s.cfg.RawDiskMaxUsedPct {
			s.requestRawPrune()
			fields := map[string]any{
				"connection_id": connectionID,
				"gateway_name":  gatewayName,
			}
			if err != nil {
				fields["error"] = err.Error()
			} else {
				fields["disk_used_percent"] = disk.UsedPercent
			}
			s.logger.Error("gateway_raw_journal_capacity_blocked", fields)
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout))
		n, err := conn.Read(buf)
		if err != nil && n == 0 {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				s.logger.Warn("gateway_connection_read_timeout", map[string]any{
					"connection_id": connectionID,
					"gateway_name":  gatewayName,
				})
			}
			return
		}
		if n == 0 {
			return
		}
		seq := rawSeq.Add(1)
		payload := append([]byte(nil), buf[:n]...)
		payloadHex := strings.ToUpper(hex.EncodeToString(payload))
		frame := journal.Frame{
			MessageID:    connectionID + "-" + strconv.FormatUint(seq, 10),
			CaptureKind:  "tcp_read",
			ConnectionID: connectionID,
			GatewayName:  gatewayName,
			RemoteAddr:   remoteAddr,
			Direction:    journal.DirectionInbound,
			PayloadHex:   payloadHex,
			ReceivedAt:   time.Now().UTC(),
			RawSeq:       seq,
		}
		if err := s.journal.Write(frame); err != nil {
			s.rawWriteFailures.Add(1)
			s.logger.Error("gateway_raw_journal_write_failed", map[string]any{
				"connection_id": connectionID,
				"error":         err.Error(),
			})
			return
		}
		s.rawFrames.Add(1)

		msg := protocol.ClassifyPayload(payload)
		if msg.Kind == protocol.MessageRegister {
			gatewayName = msg.GatewayName
		}
		s.logger.Debug(s.cfg.Debug, "gateway_protocol_classified", map[string]any{
			"connection_id": connectionID,
			"gateway_name":  gatewayName,
			"kind":          msg.Kind,
			"crc_valid":     msg.CRCValid,
			"raw_seq":       seq,
		})

		if msg.Kind == protocol.MessageHeartbeat && gatewayName == "" {
			gatewayName = msg.GatewayName
		}
		if gatewayName != "" && gatewayExternalID == "" {
			gatewayExternalID = protocol.GatewayExternalID(gatewayName)
		}
		if gatewayExternalID != "" && err == nil {
			startCommandPolling.Do(func() {
				s.startPendingCommandLoop(connCtx, conn, connectionID, gatewayName, gatewayExternalID, remoteAddr, remoteIP, remotePort, &rawSeq)
			})
		}
		// net.Conn may return final bytes and EOF together; preserve bytes first.
		if err != nil {
			return
		}
	}
}

func (s *Server) startPendingCommandLoop(ctx context.Context, conn net.Conn, connectionID, gatewayName, gatewayExternalID, remoteAddr, remoteIP string, remotePort int, rawSeq *atomic.Uint64) {
	s.logger.Info("gateway_command_polling_started", map[string]any{
		"connection_id":       connectionID,
		"gateway_name":        gatewayName,
		"gateway_external_id": gatewayExternalID,
		"poll_interval_ms":    s.cfg.CommandPollInterval.Milliseconds(),
		"poll_limit":          s.cfg.CommandPollLimit,
	})
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.pollPendingCommands(ctx, conn, connectionID, gatewayName, gatewayExternalID, remoteAddr, remoteIP, remotePort, rawSeq)
	}()
}

func (s *Server) pollPendingCommands(ctx context.Context, conn net.Conn, connectionID, gatewayName, gatewayExternalID, remoteAddr, remoteIP string, remotePort int, rawSeq *atomic.Uint64) {
	// Remember completed TCP writes while their sent status is awaiting confirmation.
	writtenCommands := make(map[uint64]bool)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		commands, err := s.forwarder.ListPendingCommands(ctx, gatewayExternalID, s.cfg.CommandPollLimit)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.logger.Warn("gateway_pending_commands_poll_failed", map[string]any{
				"connection_id":       connectionID,
				"gateway_name":        gatewayName,
				"gateway_external_id": gatewayExternalID,
				"error":               err.Error(),
			})
		} else {
			s.dispatchPendingCommands(ctx, conn, connectionID, gatewayName, gatewayExternalID, remoteAddr, remoteIP, remotePort, rawSeq, commands, writtenCommands)
		}

		timer.Reset(s.cfg.CommandPollInterval)
	}
}

func (s *Server) dispatchPendingCommands(ctx context.Context, conn net.Conn, connectionID, gatewayName, gatewayExternalID, remoteAddr, remoteIP string, remotePort int, rawSeq *atomic.Uint64, commands []PendingCommand, writtenCommands map[uint64]bool) {
	// Bound connection-local memory to commands still returned as pending.
	pendingIDs := make(map[uint64]bool, len(commands))
	for _, command := range commands {
		pendingIDs[command.CommandID] = true
	}
	for commandID := range writtenCommands {
		if !pendingIDs[commandID] {
			delete(writtenCommands, commandID)
		}
	}
	for _, command := range commands {
		if ctx.Err() != nil {
			return
		}
		if !writtenCommands[command.CommandID] {
			payload, err := decodeCommandHex(command.CommandHex)
			if err != nil {
				s.logger.Error("gateway_pending_command_decode_failed", map[string]any{
					"connection_id":       connectionID,
					"gateway_name":        gatewayName,
					"gateway_external_id": gatewayExternalID,
					"command_id":          command.CommandID,
					"error":               err.Error(),
				})
				continue
			}
			if policyErr := controlpolicy.ValidateDownlink(payload, !command.SendOnce); policyErr != nil {
				s.logger.Warn("gateway_command_policy_rejected_no_send", map[string]any{"command_id": command.CommandID, "error": policyErr.Error()})
				if reportErr := s.forwarder.RejectUnsafeCommand(ctx, command.CommandID); reportErr != nil {
					s.logger.Warn("gateway_command_policy_reject_report_failed", map[string]any{"command_id": command.CommandID})
				}
				continue
			}
			if command.SendOnce {
				if err := s.forwarder.ClaimRawDebugCommand(ctx, command.CommandID, gatewayExternalID); err != nil {
					s.logger.Warn("gateway_raw_debug_claim_failed_no_send", map[string]any{"command_id": command.CommandID, "error": err.Error()})
					continue
				}
			}
			if s.cfg.CommandWriteTimeout > 0 {
				_ = conn.SetWriteDeadline(time.Now().Add(s.cfg.CommandWriteTimeout))
			}
			seq := rawSeq.Add(1)
			frame := journal.Frame{
				MessageID:    connectionID + "-" + strconv.FormatUint(seq, 10),
				CaptureKind:  "tcp_write_attempt",
				ConnectionID: connectionID,
				GatewayName:  gatewayName,
				RemoteAddr:   remoteAddr,
				Direction:    journal.DirectionOutbound,
				PayloadHex:   strings.ToUpper(hex.EncodeToString(payload)),
				ReceivedAt:   time.Now().UTC(),
				RawSeq:       seq,
			}
			if err := s.journal.Write(frame); err != nil {
				s.rawWriteFailures.Add(1)
				s.logger.Error("gateway_outbound_journal_write_failed", map[string]any{
					"connection_id": connectionID,
					"command_id":    command.CommandID,
					"error":         err.Error(),
				})
				return
			}
			s.rawFrames.Add(1)
			if _, err := conn.Write(payload); err != nil {
				s.logger.Warn("gateway_pending_command_write_failed", map[string]any{
					"connection_id":       connectionID,
					"gateway_name":        gatewayName,
					"gateway_external_id": gatewayExternalID,
					"command_id":          command.CommandID,
					"error":               err.Error(),
				})
				return
			}
			if s.cfg.CommandWriteTimeout > 0 {
				_ = conn.SetWriteDeadline(time.Time{})
			}
			s.logger.Info("gateway_pending_command_written", map[string]any{
				"connection_id":       connectionID,
				"gateway_name":        gatewayName,
				"gateway_external_id": gatewayExternalID,
				"command_id":          command.CommandID,
				"bytes":               len(payload),
			})
			writtenCommands[command.CommandID] = true
		}
		if err := s.forwarder.MarkCommandSent(ctx, command.CommandID); err != nil {
			if ctx.Err() != nil {
				return
			}
			s.logger.Warn("gateway_pending_command_mark_sent_failed", map[string]any{
				"connection_id":       connectionID,
				"gateway_name":        gatewayName,
				"gateway_external_id": gatewayExternalID,
				"command_id":          command.CommandID,
				"error":               err.Error(),
			})
			continue
		}
	}
}

func decodeCommandHex(raw string) ([]byte, error) {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(raw)), "")
	normalized = strings.TrimPrefix(strings.TrimPrefix(normalized, "0x"), "0X")
	if normalized == "" {
		return nil, errors.New("command_hex is empty")
	}
	payload, err := hex.DecodeString(normalized)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *Server) forwardRawFrame(ctx context.Context, frame journal.Frame, gatewayName, remoteIP string, remotePort int) error {
	envelope := RawFrameEnvelope{
		ProtocolVersion: "nehuang-gateway-adapter-v1",
		SourceType:      "gateway_adapter",
		MessageID:       frame.MessageID,
		CaptureID:       frame.MessageID,
		CaptureKind:     frame.CaptureKind,
		ConnectionID:    frame.ConnectionID,
		GatewayName:     gatewayName,
		RemoteIP:        remoteIP,
		RemotePort:      remotePort,
		Direction:       string(frame.Direction),
		PayloadHex:      frame.PayloadHex,
		ReceivedAt:      frame.ReceivedAt.Format(time.RFC3339Nano),
		RawSeq:          frame.RawSeq,
	}
	return s.forwarder.PostRawFrame(ctx, envelope)
}

func (s *Server) replayPendingFrames(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	backoff := s.cfg.RawReplayInterval
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		result := s.replayPendingBatch(ctx)
		delay := s.cfg.RawReplayInterval
		if result.forwarded > 0 || result.failed == 0 {
			backoff = s.cfg.RawReplayInterval
			if s.journal.PendingCount() > 0 {
				delay = 10 * time.Millisecond
			}
		} else {
			backoff *= 2
			if backoff > time.Minute {
				backoff = time.Minute
			}
			delay = backoff
		}
		timer.Reset(delay)
	}
}

type replayBatchResult struct {
	forwarded int
	failed    int
}

func (s *Server) replayPendingBatch(ctx context.Context) replayBatchResult {
	var result replayBatchResult
	for _, frame := range s.journal.Pending(s.cfg.RawReplayBatch) {
		remoteIP, remotePort := splitRemote(frame.RemoteAddr)
		if err := s.forwardRawFrame(ctx, frame, frame.GatewayName, remoteIP, remotePort); err != nil {
			result.failed++
			s.forwardFailures.Add(1)
			s.logger.Warn("gateway_raw_replay_failed", map[string]any{
				"message_id": frame.MessageID,
				"raw_seq":    frame.RawSeq,
				"error":      err.Error(),
			})
			continue
		}
		if err := s.journal.MarkForwarded(frame.MessageID, time.Now().UTC()); err != nil {
			result.failed++
			s.rawWriteFailures.Add(1)
			s.logger.Error("gateway_raw_replay_ack_write_failed", map[string]any{
				"message_id": frame.MessageID,
				"error":      err.Error(),
			})
			continue
		}
		result.forwarded++
	}
	return result
}

func (s *Server) pruneForwardedSegments(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.RawPruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.pruneForwardedSegmentsOnce(now.UTC())
		case <-s.pruneNow:
			s.pruneForwardedSegmentsOnce(time.Now().UTC())
		}
	}
}

func (s *Server) requestRawPrune() {
	select {
	case s.pruneNow <- struct{}{}:
	default:
	}
}

func (s *Server) pruneForwardedSegmentsOnce(now time.Time) {
	cutoff := now.Add(-s.cfg.RawRetention)
	result, err := s.journal.PruneForwardedSegments(cutoff)
	s.rawLastPruneAtMS.Store(now.UnixMilli())
	s.rawLastPruneOK.Store(err == nil)
	if result.Segments > 0 || result.AckFiles > 0 {
		s.rawPrunedSegments.Add(uint64(result.Segments))
		s.rawPrunedBytes.Add(uint64(result.Bytes))
		s.rawPrunedAckFiles.Add(uint64(result.AckFiles))
		s.rawPrunedAckBytes.Add(uint64(result.AckBytes))
		s.logger.Info("gateway_raw_journal_pruned", map[string]any{
			"cutoff":    cutoff.Format(time.RFC3339),
			"segments":  result.Segments,
			"bytes":     result.Bytes,
			"ack_files": result.AckFiles,
			"ack_bytes": result.AckBytes,
		})
	}
	if err != nil {
		s.rawPruneFailures.Add(1)
		s.logger.Error("gateway_raw_journal_prune_failed", map[string]any{
			"cutoff": cutoff.Format(time.RFC3339),
			"error":  err.Error(),
		})
		return
	}
}

func (s *Server) healthRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeProbe(w, http.StatusOK, map[string]any{
			"status":                        "ok",
			"connection_shutdown_supported": true,
			"active_connections":            s.activeConnections.Load(),
			"raw_frames":                    s.rawFrames.Load(),
			"raw_pending":                   s.journal.PendingCount(),
			"raw_write_failures":            s.rawWriteFailures.Load(),
			"forward_failures":              s.forwardFailures.Load(),
			"raw_prune_failures":            s.rawPruneFailures.Load(),
			"raw_pruned_segments":           s.rawPrunedSegments.Load(),
			"raw_pruned_bytes":              s.rawPrunedBytes.Load(),
			"raw_pruned_ack_files":          s.rawPrunedAckFiles.Load(),
			"raw_pruned_ack_bytes":          s.rawPrunedAckBytes.Load(),
			"raw_last_prune_at_ms":          s.rawLastPruneAtMS.Load(),
			"raw_last_prune_ok":             s.rawLastPruneOK.Load(),
		})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.listener == nil {
			writeProbe(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "error": "tcp listener not ready"})
			return
		}
		if err := s.journal.Probe(); err != nil {
			writeProbe(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "error": "raw journal not writable: " + err.Error()})
			return
		}
		disk, err := s.journal.DiskUsage()
		if err != nil {
			writeProbe(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "error": "raw journal disk usage unavailable: " + err.Error()})
			return
		}
		if disk.UsedPercent >= s.cfg.RawDiskMaxUsedPct {
			s.requestRawPrune()
			writeProbe(w, http.StatusServiceUnavailable, map[string]any{
				"status":            "unavailable",
				"error":             "raw journal disk usage above limit",
				"disk_used_percent": disk.UsedPercent,
				"raw_pending":       s.journal.PendingCount(),
			})
			return
		}
		upstreamStatus := "ready"
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		if err := s.forwarder.Ready(ctx); err != nil {
			upstreamStatus = "unavailable"
		}
		cancel()
		writeProbe(w, http.StatusOK, map[string]any{
			"status":               "ready",
			"upstream_status":      upstreamStatus,
			"raw_pending":          s.journal.PendingCount(),
			"disk_used_percent":    disk.UsedPercent,
			"raw_last_prune_at_ms": s.rawLastPruneAtMS.Load(),
			"raw_last_prune_ok":    s.rawLastPruneOK.Load(),
		})
	})
	return mux
}

func writeProbe(w http.ResponseWriter, status int, value map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newConnectionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

func splitRemote(addr string) (string, int) {
	host, portRaw, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 0
	}
	port, _ := strconv.Atoi(portRaw)
	return host, port
}
