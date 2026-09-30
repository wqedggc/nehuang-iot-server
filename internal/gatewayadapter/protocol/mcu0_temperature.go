package protocol

import (
	"encoding/binary"
	"fmt"
)

// MCU0TemperatureFrame preserves unresolved address and clock bytes without guessing.
type MCU0TemperatureFrame struct {
	DeviceID [2]byte
	Clock    [3]byte
	Mode     string
	Tag      MCU1Tag
	Samples  []MCU0TemperatureSample
}
type MCU0TemperatureSample struct {
	Sequence byte
	Celsius  float64
}

func DecodeMCU0Temperature(raw []byte) (float64, error) {
	if len(raw) != 2 {
		return 0, fmt.Errorf("temperature requires 2 bytes")
	}
	return float64(int16(binary.BigEndian.Uint16(raw))) / 10, nil
}

// EncodeMCU0Temperature takes exact tenths to avoid rounding arbitrary floats.
func EncodeMCU0Temperature(tenths int16) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, uint16(tenths))
	return out
}

// DecodeMCU0TemperatureFrame requires a trusted, explicitly validated frame boundary.
// The layout uses the PPT's one-byte history sequence; complete hardware vectors
// must validate it before production activation. Records remain in wire order;
// their sequence-to-time mapping is not inferred. Supports the confirmed 1..15 count.
func DecodeMCU0TemperatureFrame(raw []byte, boundaryConfirmed bool) (MCU0TemperatureFrame, error) {
	var f MCU0TemperatureFrame
	if !boundaryConfirmed {
		return f, fmt.Errorf("MCU0 frame boundary not confirmed")
	}
	if len(raw) < 9 || len(raw) > 54 {
		return f, fmt.Errorf("unsupported MCU0 temperature length %d", len(raw))
	}
	if !ValidateCRC16Modbus(raw) {
		return f, fmt.Errorf("MCU0 CRC mismatch")
	}
	copy(f.DeviceID[:], raw[:2])
	copy(f.Clock[:], raw[3:6])
	tag, err := DecodeMCU1Tag(raw[len(raw)-3])
	if err != nil {
		return f, err
	}
	f.Tag = tag
	switch raw[2] {
	case 1:
		f.Mode = "automatic"
		if len(raw) == 10 && raw[6] == 0 {
			return f, nil
		}
		if len(raw) < 12 || (len(raw)-9)%3 != 0 {
			return f, fmt.Errorf("automatic history requires 1..15 complete records")
		}
		count := (len(raw) - 9) / 3
		for i := 0; i < count; i++ {
			offset := 6 + i*3
			v, _ := DecodeMCU0Temperature(raw[offset+1 : offset+3])
			f.Samples = append(f.Samples, MCU0TemperatureSample{Sequence: raw[offset], Celsius: v})
		}
	case 2:
		f.Mode = "manual"
		if len(raw) != 11 {
			return f, fmt.Errorf("manual realtime requires 2-byte temperature")
		}
		v, _ := DecodeMCU0Temperature(raw[6:8])
		f.Samples = []MCU0TemperatureSample{{Celsius: v}}
	default:
		return f, fmt.Errorf("not an MCU0 temperature function")
	}
	return f, nil
}

// BuildMCU0HistoryACK is a pure constructor, not permission to transmit or clear
// a device cache. Callers must supply a confirmed batch clock after durable commit.
func BuildMCU0HistoryACK(device [2]byte, clock [3]byte) []byte {
	return AppendCRC16Modbus([]byte{device[0], device[1], 2, clock[0], clock[1], clock[2]})
}
