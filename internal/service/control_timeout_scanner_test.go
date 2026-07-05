package service

import (
	"context"
	"testing"
	"time"
)

type fakeControlTimeoutRepo struct {
	cutoff time.Time
	count  int64
}

func (f *fakeControlTimeoutRepo) MarkControlCommandsTimeout(ctx context.Context, cutoff time.Time) (int64, error) {
	f.cutoff = cutoff
	return f.count, nil
}

func TestControlTimeoutScannerScanOnceUsesTTLCutoff(t *testing.T) {
	repo := &fakeControlTimeoutRepo{count: 2}
	scanner := NewControlTimeoutScanner(repo, 5*time.Minute, 30*time.Second)
	now := time.Date(2026, 7, 5, 14, 0, 0, 0, time.UTC)

	count, err := scanner.ScanOnce(context.Background(), now)
	if err != nil {
		t.Fatalf("scan once: %v", err)
	}
	if count != 2 {
		t.Fatalf("timeout count: want 2, got %d", count)
	}
	wantCutoff := now.Add(-5 * time.Minute)
	if !repo.cutoff.Equal(wantCutoff) {
		t.Fatalf("cutoff: want %s, got %s", wantCutoff, repo.cutoff)
	}
}
