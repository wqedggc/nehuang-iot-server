package protocol

import "fmt"

// MCU0WireFrame follows all 50 hardware vectors received 2026-10-01:
// 7E | one-byte body length (including CRC) | body | CRC low, high.
// The CRC covers body bytes only, excluding 7E and LEN. No escaping is inferred.
type MCU0WireFrame struct {
	Wire     []byte
	Body     []byte // Includes the original trailing CRC bytes.
	CRCValid bool
}

// MCU0StreamDecoder handles split/coalesced reads for this explicitly selected
// wire format. It never scans for 7E inside a declared body. Invalid headers
// stop decoding until the caller resets; raw capture must remain authoritative.
type MCU0StreamDecoder struct {
	pending []byte
	failed  bool
}

func (d *MCU0StreamDecoder) Feed(chunk []byte) ([]MCU0WireFrame, error) {
	if d.failed {
		return nil, fmt.Errorf("MCU0 stream requires reset after invalid boundary")
	}
	d.pending = append(d.pending, chunk...)
	var frames []MCU0WireFrame
	for len(d.pending) >= 2 {
		if d.pending[0] != 0x7e || d.pending[1] < 3 {
			d.failed = true
			return frames, fmt.Errorf("invalid MCU0 7E/LEN boundary")
		}
		length := int(d.pending[1]) + 2
		if len(d.pending) < length {
			break
		}
		wire := append([]byte(nil), d.pending[:length]...)
		frames = append(frames, MCU0WireFrame{Wire: wire, Body: wire[2:], CRCValid: ValidateCRC16Modbus(wire[2:])})
		d.pending = d.pending[length:]
	}
	return frames, nil
}
func (d *MCU0StreamDecoder) Pending() []byte { return append([]byte(nil), d.pending...) }
func (d *MCU0StreamDecoder) Reset()          { d.pending = nil; d.failed = false }
func DecodeMCU0WireFrame(wire []byte) (MCU0WireFrame, error) {
	var d MCU0StreamDecoder
	frames, err := d.Feed(wire)
	if err != nil {
		return MCU0WireFrame{}, err
	}
	if len(frames) != 1 || len(d.Pending()) != 0 {
		return MCU0WireFrame{}, fmt.Errorf("one complete MCU0 wire frame required")
	}
	return frames[0], nil
}

type MCU0StateObservation struct {
	DeviceID [2]byte
	Clock    [3]byte
	Tag      MCU1Tag
}

// BB05 is a state observation, without commandId or proof of which command
// caused it. Fault meanings beyond the documented Tag are not inferred.
func DecodeMCU0StateObservation(body []byte) (MCU0StateObservation, error) {
	var s MCU0StateObservation
	if len(body) != 9 || body[2] != 5 {
		return s, fmt.Errorf("BB05 state requires 9-byte body")
	}
	if !ValidateCRC16Modbus(body) {
		return s, fmt.Errorf("MCU0 state CRC mismatch")
	}
	if body[3] > 23 || body[4] > 59 || body[5] > 59 {
		return s, fmt.Errorf("invalid MCU0 binary clock")
	}
	tag, err := DecodeMCU1Tag(body[6])
	if err != nil {
		return s, err
	}
	copy(s.DeviceID[:], body[:2])
	copy(s.Clock[:], body[3:6])
	s.Tag = tag
	return s, nil
}
