package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"nehuang-iot-server/internal/gatewayadapter"
	"nehuang-iot-server/internal/gatewayadapter/journal"
)

func main() {
	cfg := gatewayadapter.Config{
		TCPAddr:              envDefault("NEHUANG_GATEWAY_TCP_ADDR", ":8081"),
		HealthAddr:           envDefault("NEHUANG_GATEWAY_HEALTH_ADDR", ":18081"),
		APIBaseURL:           envDefault("NEHUANG_GATEWAY_API_BASE_URL", "http://43.143.208.153:8080"),
		AdapterToken:         envDefault("NEHUANG_ADAPTER_TOKEN", "nehuang-adapter-dev-token"),
		RawLogDir:            envDefault("NEHUANG_GATEWAY_RAW_LOG_DIR", "data/gateway-adapter/raw-frames"),
		Debug:                envBool("NEHUANG_GATEWAY_DEBUG", false),
		ReadBufferSize:       envInt("NEHUANG_GATEWAY_READ_BUFFER_SIZE", 4096),
		ReadTimeout:          time.Duration(envInt("NEHUANG_GATEWAY_READ_TIMEOUT_SECONDS", 120)) * time.Second,
		CommandPollInterval:  time.Duration(envInt("NEHUANG_GATEWAY_COMMAND_POLL_INTERVAL_SECONDS", 2)) * time.Second,
		CommandPollLimit:     envInt("NEHUANG_GATEWAY_COMMAND_POLL_LIMIT", 20),
		CommandWriteTimeout:  time.Duration(envInt("NEHUANG_GATEWAY_COMMAND_WRITE_TIMEOUT_SECONDS", 5)) * time.Second,
		ForwardUnknownFrames: envBool("NEHUANG_GATEWAY_FORWARD_UNKNOWN_FRAMES", false),
	}

	logger := gatewayadapter.NewLogger(os.Stdout)
	jw, err := journal.NewWriter(cfg.RawLogDir, int64(envInt("NEHUANG_GATEWAY_RAW_LOG_MAX_BYTES", 64*1024*1024)))
	if err != nil {
		log.Fatalf("open raw journal: %v", err)
	}
	defer jw.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-stopCh
		logger.Info("gateway_adapter_signal", map[string]any{"signal": sig.String()})
		cancel()
	}()

	srv := gatewayadapter.NewServer(cfg, jw, logger)
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("gateway adapter failed: %v", err)
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		switch v {
		case "1", "true", "TRUE", "yes", "YES", "on", "ON":
			return true
		case "0", "false", "FALSE", "no", "NO", "off", "OFF":
			return false
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
