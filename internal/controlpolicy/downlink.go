// Package controlpolicy owns the shared single-command safety boundary.
package controlpolicy

import (
	"fmt"
	"nehuang-iot-server/internal/wirecrc"
)

// ValidateDownlink accepts exactly one known MCU0 downlink, bare or 7E/LEN.
// Unknown schemas, multi-frame buffers and trailing bytes are rejected.
// Legacy single-byte actions are an explicit existing business contract only.
func ValidateDownlink(raw []byte, allowLegacy bool) error {
	if allowLegacy && len(raw) == 1 && (raw[0] == 0xf0 || raw[0] == 0x0f || raw[0] == 0) {
		return nil
	}
	body := raw
	if len(raw) > 0 && raw[0] == 0x7e {
		if len(raw) < 2 || int(raw[1])+2 != len(raw) {
			return fmt.Errorf("exactly one complete 7E/LEN frame required; no trailing bytes")
		}
		body = raw[2:]
	}
	if !wirecrc.ValidModbus(body) {
		return fmt.Errorf("complete single-frame CRC-16/Modbus required")
	}
	if len(body) == 5 {
		if body[0] > 23 || body[1] > 59 || body[2] > 59 {
			return fmt.Errorf("invalid binary clock")
		}
		return nil
	}
	if len(body) < 8 {
		return fmt.Errorf("unconfirmed downlink schema")
	}
	if body[2] == 2 {
		return fmt.Errorf("BB02 cache-clear command is disabled")
	}
	if !((body[2] == 0 && len(body) == 8) || (body[2] == 3 && len(body) == 9) || (body[2] == 1 && len(body) == 12)) {
		return fmt.Errorf("only confirmed single clock/manual/threshold downlinks are enabled")
	}
	if body[3] > 23 || body[4] > 59 || body[5] > 59 {
		return fmt.Errorf("invalid binary clock")
	}
	if body[2] == 3 && body[6]&0x80 == 0 {
		return fmt.Errorf("manual command Tag marker required")
	}
	return nil
}
