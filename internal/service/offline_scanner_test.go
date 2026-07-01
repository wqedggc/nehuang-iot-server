package service

import (
	"context"
	"testing"
	"time"
)

type fakeOfflineRepo struct {
	cutoff time.Time
	result OfflineScanResult
}

func (f *fakeOfflineRepo) MarkOffline(ctx context.Context, cutoff time.Time) (OfflineScanResult, error) {
	f.cutoff = cutoff
	return f.result, nil
}

func TestOfflineScannerScanOnceUsesThresholdCutoff(t *testing.T) {
	repo := &fakeOfflineRepo{result: OfflineScanResult{Gateways: 1}}
	scanner := NewOfflineScanner(repo, 35*time.Second, 10*time.Second)
	now := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)

	result, err := scanner.ScanOnce(context.Background(), now)
	if err != nil {
		t.Fatalf("scan once: %v", err)
	}
	if result.Gateways != 1 {
		t.Fatalf("result gateways: want 1, got %d", result.Gateways)
	}
	wantCutoff := now.Add(-35 * time.Second)
	if !repo.cutoff.Equal(wantCutoff) {
		t.Fatalf("cutoff: want %s, got %s", wantCutoff, repo.cutoff)
	}
}
