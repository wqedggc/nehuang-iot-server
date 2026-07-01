package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpapi "nehuang-iot-server/internal/api/http"
	"nehuang-iot-server/internal/config"
	mysqlrepo "nehuang-iot-server/internal/repository/mysql"
	"nehuang-iot-server/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := mysqlrepo.Open(cfg.MySQL)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()

	repo := mysqlrepo.New(db)
	handler := httpapi.NewServer(repo, cfg)

	offlineCtx, stopOfflineScanner := context.WithCancel(context.Background())
	defer stopOfflineScanner()
	if cfg.OfflineScan.Enabled {
		scanner := service.NewOfflineScanner(
			repo,
			time.Duration(cfg.OfflineScan.ThresholdSeconds)*time.Second,
			time.Duration(cfg.OfflineScan.IntervalSeconds)*time.Second,
		)
		go scanner.Run(offlineCtx)
		log.Printf("offline scanner enabled threshold=%ds interval=%ds",
			cfg.OfflineScan.ThresholdSeconds, cfg.OfflineScan.IntervalSeconds)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("nehuang server listening on %s", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stopCh:
		log.Printf("received signal: %s", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}

	stopOfflineScanner()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}
