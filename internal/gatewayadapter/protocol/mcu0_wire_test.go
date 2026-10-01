package protocol

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func realMCU0Vectors(t *testing.T) ([][]byte, []string) {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("..", "..", "..", "docs", "protocols", "mcu0-20261001", "frames.txt"))
	if e != nil {
		t.Fatal(e)
	}
	var frames [][]byte
	var labels []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		label, text, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatal("bad fixture")
		}
		r, e := hex.DecodeString(strings.ReplaceAll(text, " ", ""))
		if e != nil {
			t.Fatal(e)
		}
		frames = append(frames, r)
		labels = append(labels, label)
	}
	return frames, labels
}
func TestMCU0All50RealVectorsAndAllSplitBoundaries(t *testing.T) {
	frames, _ := realMCU0Vectors(t)
	if len(frames) != 50 {
		t.Fatalf("fixture has %d records", len(frames))
	}
	for i, wire := range frames {
		f, e := DecodeMCU0WireFrame(wire)
		if e != nil || !f.CRCValid || int(wire[1]) != len(wire)-2 {
			t.Fatalf("line %d frame=%#v err=%v", i+1, f, e)
		}
		if ValidateCRC16Modbus(wire) {
			t.Fatalf("line %d unexpectedly includes prefix in CRC", i+1)
		}
		for split := 0; split <= len(wire); split++ {
			var d MCU0StreamDecoder
			a, e := d.Feed(wire[:split])
			if e != nil {
				t.Fatal(e)
			}
			b, e := d.Feed(wire[split:])
			if e != nil {
				t.Fatal(e)
			}
			all := append(a, b...)
			if len(all) != 1 || !bytes.Equal(all[0].Wire, wire) || len(d.Pending()) != 0 {
				t.Fatalf("line %d split %d", i+1, split)
			}
		}
	}
	var joined []byte
	for _, f := range frames {
		joined = append(joined, f...)
	}
	for _, chunkSize := range []int{1, 2, 7, 13, 64, len(joined)} {
		var d MCU0StreamDecoder
		var got []MCU0WireFrame
		for i := 0; i < len(joined); i += chunkSize {
			end := i + chunkSize
			if end > len(joined) {
				end = len(joined)
			}
			v, e := d.Feed(joined[i:end])
			if e != nil {
				t.Fatal(e)
			}
			got = append(got, v...)
		}
		if len(got) != 50 || len(d.Pending()) != 0 {
			t.Fatal("coalesced stream incomplete")
		}
		for i, f := range got {
			if !bytes.Equal(f.Wire, frames[i]) {
				t.Fatal("wire altered")
			}
		}
	}
}
func TestMCU0RealUplinkModesHistoryAndState(t *testing.T) {
	frames, labels := realMCU0Vectors(t)
	var history []int
	realtime, observations := 0, 0
	for i, w := range frames {
		if !strings.HasPrefix(labels[i], "RECV") {
			continue
		}
		f, e := DecodeMCU0WireFrame(w)
		if e != nil {
			t.Fatal(e)
		}
		switch f.Body[2] {
		case 1, 2:
			v, e := DecodeMCU0TemperatureFrame(f.Body, true)
			if e != nil {
				t.Fatalf("line %d: %v", i+1, e)
			}
			if v.DeviceID != [2]byte{0, 1} {
				t.Fatal("ID changed")
			}
			if f.Body[2] == 1 {
				history = append(history, len(v.Samples))
				for n, s := range v.Samples {
					if int(s.Sequence) != n+1 {
						t.Fatal("sequence changed")
					}
				}
			} else {
				realtime++
				if len(v.Samples) != 1 || v.Samples[0].Celsius < 10 || v.Samples[0].Celsius > 14 {
					t.Fatal("temperature incorrect")
				}
			}
		case 5:
			v, e := DecodeMCU0StateObservation(f.Body)
			if e != nil {
				t.Fatal(e)
			}
			observations++
			if v.Tag.NTCFault || !v.Tag.CloseRelayActive || v.Clock != [3]byte{20, 28, 1} {
				t.Fatalf("BB05=%#v", v)
			}
		default:
			t.Fatalf("unexpected function at line %d", i+1)
		}
	}
	if len(history) != 3 || history[0] != 15 || history[1] != 11 || history[2] != 11 || observations != 1 || realtime != 36 {
		t.Fatalf("history %v realtime %d state %d", history, realtime, observations)
	}
}
func TestMCU0PayloadMarkerBadCRCAndInvalidBoundary(t *testing.T) {
	body := AppendCRC16Modbus([]byte{0, 1, 0x7e, 0x7e})
	wire := append([]byte{0x7e, byte(len(body))}, body...)
	f, e := DecodeMCU0WireFrame(wire)
	if e != nil || !f.CRCValid || !bytes.Equal(f.Body, body) {
		t.Fatal("payload 7E treated as delimiter")
	}
	bad := append([]byte(nil), wire...)
	bad[len(bad)-1] ^= 1
	var d MCU0StreamDecoder
	got, e := d.Feed(append(bad, wire...))
	if e != nil || len(got) != 2 || got[0].CRCValid || !got[1].CRCValid || !bytes.Equal(got[0].Wire, bad) {
		t.Fatal("bad CRC dropped or stream broken")
	}
	if _, e = d.Feed([]byte{0x7e, 0}); e == nil {
		t.Fatal("zero LEN accepted")
	}
	if _, e = d.Feed(wire); e == nil {
		t.Fatal("invalid boundary silently resynchronized")
	}
	d.Reset()
	if afterReset, resetErr := d.Feed(wire); resetErr != nil || len(afterReset) != 1 {
		t.Fatal("reset failed")
	}
}
