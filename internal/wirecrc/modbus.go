// Package wirecrc contains the shared wire CRC primitive, without transport dependencies.
package wirecrc

func Modbus(data []byte) uint16 {
	c := uint16(0xffff)
	for _, b := range data {
		c ^= uint16(b)
		for i := 0; i < 8; i++ {
			if c&1 != 0 {
				c = c>>1 ^ 0xa001
			} else {
				c >>= 1
			}
		}
	}
	return c
}
func ValidModbus(packet []byte) bool {
	if len(packet) < 3 {
		return false
	}
	return Modbus(packet[:len(packet)-2]) == uint16(packet[len(packet)-2])|uint16(packet[len(packet)-1])<<8
}
