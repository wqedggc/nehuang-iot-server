package service

import (
	"testing"
	"time"
)

func TestRangeDurationSupportsProductRangesAndLegacyAliases(t *testing.T) {
	tests := map[string]time.Duration{
		"1d":  24 * time.Hour,
		"3d":  3 * 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
		"30d": 30 * 24 * time.Hour,
		"24h": 24 * time.Hour,
	}

	for rangeStr, want := range tests {
		got, err := RangeDuration(rangeStr)
		if err != nil {
			t.Fatalf("RangeDuration(%q): %v", rangeStr, err)
		}
		if got != want {
			t.Fatalf("RangeDuration(%q): want %s, got %s", rangeStr, want, got)
		}
	}
}

func TestValidateRangeRejectsInvalidRange(t *testing.T) {
	if err := ValidateRange("2d"); err == nil {
		t.Fatalf("ValidateRange(2d): expected error")
	}
}
