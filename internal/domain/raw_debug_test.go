package domain

import (
	"strings"
	"testing"
)

func TestRawDebugHexIsExactAndBounded(t *testing.T) {
	v, err := ValidateRawDebugHex("7E 09\n00 01 03 12 25 01 81 8B 92")
	if err != nil || v != "7E09000103122501818B92" {
		t.Fatal(v, err)
	}
	for _, v := range []string{"", "0x01", "01ZZ", "0", "01,02", strings.Repeat("AA", 513), "0102020000000000"} {
		if _, err := ValidateRawDebugHex(v); err == nil {
			t.Fatalf("unsafe input accepted %q", v)
		}
	}
	if _, err := ValidateRawDebugHex(strings.Repeat("AA", 512)); err == nil {
		t.Fatal(err)
	}
}
func TestRawDebugRequiresSingleAttempt(t *testing.T) {
	if !CommandStatusUnknown.CanAdvanceResult(CommandStatusSucceeded) || CommandStatusUnknown.Label() != "unknown" {
		t.Fatal("unknown response reconciliation unsupported")
	}
	if !IsRawDebugExtra(`{"source":"admin_raw_debug","request_id":"abc"}`) || IsRawDebugExtra(`{"source":"admin_control"}`) {
		t.Fatal("send policy detection")
	}
}
