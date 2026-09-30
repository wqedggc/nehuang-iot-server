package protocol

import "testing"

// All frame fixtures below are synthetic, not received from hardware.
func TestMCU0ConfirmedTemperatureEncoding(t *testing.T) {
	for _, tc := range []struct {
		raw    []byte
		tenths int16
	}{{[]byte{0, 0xFD}, 253}, {[]byte{0xFF, 0xCE}, -50}, {[]byte{0x80, 0}, -32768}, {[]byte{0x7F, 0xFF}, 32767}} {
		v, err := DecodeMCU0Temperature(tc.raw)
		if err != nil || v != float64(tc.tenths)/10 {
			t.Fatalf("%X %v %v", tc.raw, v, err)
		}
		out := EncodeMCU0Temperature(tc.tenths)
		if string(out) != string(tc.raw) {
			t.Fatal("encode mismatch")
		}
	}
	if _, err := DecodeMCU0Temperature([]byte{1}); err == nil {
		t.Fatal("short temperature accepted")
	}
}
func TestMCU0TemperatureFrameValidationAndMode(t *testing.T) {
	real := AppendCRC16Modbus([]byte{1, 2, 2, 12, 0, 0, 0xFF, 0xCE, 0xC0})
	f, err := DecodeMCU0TemperatureFrame(real, true)
	if err != nil || f.Mode != "manual" || len(f.Samples) != 1 || f.Samples[0].Celsius != -5 {
		t.Fatalf("%+v %v", f, err)
	}
	if _, err := DecodeMCU0TemperatureFrame(real, false); err == nil {
		t.Fatal("unconfirmed boundary accepted")
	}
	bad := append([]byte(nil), real...)
	bad[len(bad)-1] ^= 1
	if _, err := DecodeMCU0TemperatureFrame(bad, true); err == nil {
		t.Fatal("CRC accepted")
	}
	history := []byte{1, 2, 1, 12, 0, 0}
	for i := 0; i < 12; i++ {
		history = append(history, byte(i), 0, 0xFD)
	}
	history = AppendCRC16Modbus(append(history, 0xC0))
	f, err = DecodeMCU0TemperatureFrame(history, true)
	if err != nil || f.Mode != "automatic" || len(f.Samples) != 12 {
		t.Fatalf("%+v %v", f, err)
	}
	if _, err := DecodeMCU0TemperatureFrame(append(history, real...), true); err == nil {
		t.Fatal("coalesced frames accepted")
	}
	ack := BuildMCU0HistoryACK([2]byte{1, 2}, [3]byte{12, 0, 0})
	if len(ack) != 8 || ack[2] != 2 || !ValidateCRC16Modbus(ack) {
		t.Fatalf("bad ACK %X", ack)
	}
}

// Synthetic variable-size records; neither array time order nor sequence time
// meaning is inferred. Production still requires confirmed frame/layout policy.
func TestMCU0VariableHistoryPreservesWireOrder(t *testing.T) {
	for _, count := range []int{1, 3, 12, 15} {
		raw := []byte{1, 2, 1, 1, 0, 0}
		for i := 0; i < count; i++ {
			raw = append(raw, byte(40-i), 0xFF, 0xCE)
		}
		raw = AppendCRC16Modbus(append(raw, 0xC0))
		f, err := DecodeMCU0TemperatureFrame(raw, true)
		if err != nil || len(f.Samples) != count {
			t.Fatalf("count %d: %+v %v", count, f, err)
		}
		for i, s := range f.Samples {
			if s.Sequence != byte(40-i) || s.Celsius != -5 {
				t.Fatal("wire order/value changed")
			}
		}
	}
	raw := []byte{1, 2, 1, 1, 0, 0}
	for i := 0; i < 16; i++ {
		raw = append(raw, byte(i), 0, 1)
	}
	raw = AppendCRC16Modbus(append(raw, 0xC0))
	if _, err := DecodeMCU0TemperatureFrame(raw, true); err == nil {
		t.Fatal("more than ring capacity accepted")
	}
	malformed := AppendCRC16Modbus([]byte{1, 2, 1, 1, 0, 0, 1, 0, 0xC0})
	if _, err := DecodeMCU0TemperatureFrame(malformed, true); err == nil {
		t.Fatal("incomplete history record accepted")
	}
}
