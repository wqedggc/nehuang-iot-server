package protocolsummary

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCU0RealWireAdminDisplayAll50(t *testing.T) {
	b, e := os.ReadFile(filepath.Join("..", "..", "docs", "protocols", "mcu0-20261001", "frames.txt"))
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		label, text, _ := strings.Cut(line, " ")
		h := strings.ReplaceAll(text, " ", "")
		raw, _ := hex.DecodeString(h)
		s := BuildCurrentUplink(h)
		if strings.HasPrefix(label, "SEND") {
			s = BuildCurrentDownlink(h)
		}
		if s == nil || s.Kind != "mcu0_wire_capture" || s.RecordCount != 1 || s.CRCValid == nil || !*s.CRCValid {
			t.Fatalf("%s summary=%#v", label, s)
		}
		for _, f := range s.Fields {
			if f.RawHex != "" && (f.ByteStart < 0 || f.ByteEnd >= len(raw) || f.ByteStart > f.ByteEnd) {
				t.Fatalf("%s incorrect field offsets %#v", label, f)
			}
		}
		if label == "SEND18:37:01.004" {
			for _, f := range s.Fields {
				if strings.Contains(f.Name, "requested_tag") && (!strings.Contains(f.Note, "不是NTC") || f.RawHex != "81") {
					t.Fatal("downlink interpreted as NTC observation")
				}
			}
		}
		if label == "RECV20:28:01.623" {
			found := false
			for _, f := range s.Fields {
				if strings.Contains(f.Value, "BB05") {
					found = true
				}
			}
			if !found {
				t.Fatal("BB05 not displayed")
			}
		}
		count++
	}
	if count != 50 {
		t.Fatalf("records %d", count)
	}
}
func TestMCU0WireDisplayFragmentsCoalescedAndBadCRC(t *testing.T) {
	h := "7E0B0001021138010084C028D4"
	s := BuildCurrentUplink(h + h)
	if s.RecordCount != 2 {
		t.Fatal("coalesced frame lost")
	}
	s = BuildCurrentUplink(h[:12])
	if s.RecordCount != 0 || len(s.Diagnostics) == 0 {
		t.Fatal("fragment called complete frame")
	}
	s = BuildCurrentUplink(h[:len(h)-2] + "00")
	if s.CRCValid == nil || *s.CRCValid {
		t.Fatal("bad CRC displayed valid")
	}
	for _, f := range s.Fields {
		if strings.Contains(f.Name, "current_temperature") {
			t.Fatal("invalid CRC decoded as valid temperature")
		}
	}
}
