package service

import (
	"context"
	"log"
	"time"
)

// ControlTimeoutRepository defines persistence operations needed by ControlTimeoutScanner.
type ControlTimeoutRepository interface {
	MarkControlCommandsTimeout(ctx context.Context, cutoff time.Time) (int64, error)
}

// ControlTimeoutScanner periodically marks stale control commands as timeout.
type ControlTimeoutScanner struct {
	repo     ControlTimeoutRepository
	ttl      time.Duration
	interval time.Duration
}

func NewControlTimeoutScanner(repo ControlTimeoutRepository, ttl time.Duration, interval time.Duration) *ControlTimeoutScanner {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &ControlTimeoutScanner{repo: repo, ttl: ttl, interval: interval}
}

func (s *ControlTimeoutScanner) ScanOnce(ctx context.Context, now time.Time) (int64, error) {
	return s.repo.MarkControlCommandsTimeout(ctx, now.Add(-s.ttl))
}

func (s *ControlTimeoutScanner) Run(ctx context.Context) {
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

func (s *ControlTimeoutScanner) scanAndLog(ctx context.Context) {
	count, err := s.ScanOnce(ctx, time.Now())
	if err != nil {
		log.Printf("control timeout scan failed: %v", err)
		return
	}
	if count > 0 {
		log.Printf("control timeout scan marked commands=%d", count)
	}
}
