package protocol

// CRC16Modbus calculates CRC-16/MODBUS for data.
//
// Parameters:
//   - init: 0xFFFF
//   - polynomial: 0xA001
//   - output: low byte first when appended to a packet
func CRC16Modbus(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func AppendCRC16Modbus(data []byte) []byte {
	crc := CRC16Modbus(data)
	out := make([]byte, 0, len(data)+2)
	out = append(out, data...)
	out = append(out, byte(crc&0xFF), byte((crc>>8)&0xFF))
	return out
}

func ValidateCRC16Modbus(packet []byte) bool {
	if len(packet) < 3 {
		return false
	}
	body := packet[:len(packet)-2]
	got := uint16(packet[len(packet)-2]) | uint16(packet[len(packet)-1])<<8
	return CRC16Modbus(body) == got
}
