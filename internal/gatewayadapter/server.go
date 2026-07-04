package gatewayadapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	TCPAddr        string
	HealthAddr     string
	APIBaseURL     string
	AdapterToken   string
	RawLogDir      string
	Debug          bool
	ReadBufferSize int
	ReadTimeout    time.Duration
}

type Server struct {
	cfg       Config
	journal   *journal.Writer
	logger    *Logger
	forwarder *Forwarder

	listener net.Listener
	wg       sync.WaitGroup
	closing  atomic.Bool

	activeConnections atomic.Int64
	rawFrames         atomic.Uint64
	rawWriteFailures  atomic.Uint64
	forwardFailures   atomic.Uint64
}

func NewServer(cfg Config, jw *journal.Writer, logger *Logger) *Server {
	if cfg.TCPAddr == "" {
		cfg.TCPAddr = ":8081"
	}
	if cfg.HealthAddr == "" {
		cfg.HealthAddr = ":18081"
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = "http://43.143.208.153:8080"
	}
	if cfg.ReadBufferSize <= 0 {
		cfg.ReadBufferSize = 4096
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 120 * time.Second
	}
	if logger == nil {
		logger = NewLogger(nil)
	}
	return &Server{
		cfg:       cfg,
		journal:   jw,
		logger:    logger,
		forwarder: NewForwarder(cfg.APIBaseURL, cfg.AdapterToken),
	}
}

func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.TCPAddr)
	if err != nil {
		return err
	}
	s.listener = ln
	s.logger.Info("gateway_adapter_started", map[string]any{
		"tcp_addr":    s.cfg.TCPAddr,
		"health_addr": s.cfg.HealthAddr,
		"raw_log_dir": s.cfg.RawLogDir,
	})

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
		_ = ln.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = healthSrv.Shutdown(shutdownCtx)
		s.wg.Wait()
		return nil
	case err := <-errCh:
		s.closing.Store(true)
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
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	s.activeConnections.Add(1)
	defer s.activeConnections.Add(-1)

	connectionID := newConnectionID()
	remoteAddr := conn.RemoteAddr().String()
	remoteIP, remotePort := splitRemote(remoteAddr)
	var gatewayName string
	var rawSeq uint64

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
		_ = conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout))
		n, err := conn.Read(buf)
		if err != nil {
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
		rawSeq++
		payload := append([]byte(nil), buf[:n]...)
		payloadHex := strings.ToUpper(hex.EncodeToString(payload))
		frame := journal.Frame{
			ConnectionID: connectionID,
			GatewayName:  gatewayName,
			RemoteAddr:   remoteAddr,
			Direction:    journal.DirectionInbound,
			PayloadHex:   payloadHex,
			ReceivedAt:   time.Now().UTC(),
			RawSeq:       rawSeq,
		}
		if err := s.journal.Write(frame); err != nil {
			s.rawWriteFailures.Add(1)
			s.logger.Error("gateway_raw_journal_write_failed", map[string]any{
				"connection_id": connectionID,
				"error":         err.Error(),
			})
			continue
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
			"raw_seq":       rawSeq,
		})

		if msg.Kind == protocol.MessageRegister || msg.Kind == protocol.MessageHeartbeat {
			if gatewayName == "" {
				gatewayName = msg.GatewayName
			}
			if err := s.forwardGatewayEvent(context.Background(), msg.Kind, connectionID, gatewayName, remoteIP, remotePort, rawSeq); err != nil {
				s.forwardFailures.Add(1)
				s.logger.Warn("gateway_event_forward_failed", map[string]any{
					"connection_id": connectionID,
					"gateway_name":  gatewayName,
					"kind":          msg.Kind,
					"error":         err.Error(),
				})
			}
		}
	}
}

func (s *Server) forwardGatewayEvent(ctx context.Context, kind protocol.MessageKind, connectionID, gatewayName, remoteIP string, remotePort int, rawSeq uint64) error {
	eventType := "heartbeat"
	if kind == protocol.MessageRegister {
		eventType = "register"
	}
	envelope := DeviceEventEnvelope{
		ProtocolVersion:   "nehuang-gateway-adapter-v1",
		SourceType:        "gateway_adapter",
		MessageID:         connectionID + "-" + strconv.FormatUint(rawSeq, 10),
		GatewayName:       gatewayName,
		GatewayExternalID: protocol.GatewayExternalID(gatewayName),
		RemoteIP:          remoteIP,
		RemotePort:        remotePort,
		ReportedAtMS:      time.Now().UTC().Format(time.RFC3339),
		Events:            []DeviceEvent{{EventType: eventType}},
	}
	return s.forwarder.PostDeviceEvents(ctx, envelope)
}

func (s *Server) healthRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeProbe(w, http.StatusOK, map[string]any{
			"status":             "ok",
			"active_connections": s.activeConnections.Load(),
			"raw_frames":         s.rawFrames.Load(),
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
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.forwarder.Ready(ctx); err != nil {
			writeProbe(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "error": err.Error()})
			return
		}
		writeProbe(w, http.StatusOK, map[string]any{"status": "ready"})
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
