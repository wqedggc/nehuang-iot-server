package protocol

import "nehuang-iot-server/internal/wirecrc"

// CRC16Modbus calculates CRC-16/MODBUS for data.
//
// Parameters:
//   - init: 0xFFFF
//   - polynomial: 0xA001
//   - output: low byte first when appended to a packet
func CRC16Modbus(data []byte) uint16 { return wirecrc.Modbus(data) }

func AppendCRC16Modbus(data []byte) []byte {
	crc := CRC16Modbus(data)
	out := make([]byte, 0, len(data)+2)
	out = append(out, data...)
	out = append(out, byte(crc&0xFF), byte((crc>>8)&0xFF))
	return out
}

func ValidateCRC16Modbus(packet []byte) bool { return wirecrc.ValidModbus(packet) }
