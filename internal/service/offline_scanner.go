package service

import (
	"context"
	"log"
	"time"
)

// OfflineRepository defines the persistence operations needed by OfflineScanner.
type OfflineRepository interface {
	MarkOffline(ctx context.Context, cutoff time.Time) (OfflineScanResult, error)
}

// OfflineScanResult reports how many rows were corrected by one scan.
type OfflineScanResult struct {
	Gateways     int64
	Collectors   int64
	DevicePoints int64
	RealtimeMaps int64
}

// OfflineScanner periodically marks stale devices offline.
type OfflineScanner struct {
	repo      OfflineRepository
	threshold time.Duration
	interval  time.Duration
}

func NewOfflineScanner(repo OfflineRepository, threshold time.Duration, interval time.Duration) *OfflineScanner {
	if threshold <= 0 {
		threshold = 35 * time.Second
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &OfflineScanner{repo: repo, threshold: threshold, interval: interval}
}

func (s *OfflineScanner) ScanOnce(ctx context.Context, now time.Time) (OfflineScanResult, error) {
	return s.repo.MarkOffline(ctx, now.Add(-s.threshold))
}

func (s *OfflineScanner) Run(ctx context.Context) {
	s.scanAndLog(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scanAndLog(ctx)
		}
	}
}

func (s *OfflineScanner) scanAndLog(ctx context.Context) {
	result, err := s.ScanOnce(ctx, time.Now())
	if err != nil {
		log.Printf("offline scan failed: %v", err)
		return
	}
	if result.Gateways+result.Collectors+result.DevicePoints+result.RealtimeMaps > 0 {
		log.Printf("offline scan marked gateways=%d collectors=%d points=%d realtime_maps=%d",
			result.Gateways, result.Collectors, result.DevicePoints, result.RealtimeMaps)
	}
}
