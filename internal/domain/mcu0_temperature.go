package domain

// MCU0TemperatureBatch is resolved telemetry linked to a durable raw event.
// History never updates live point state. No command ID is inferred from telemetry.
type MCU0TemperatureBatch struct {
	RawEventID uint64
	PayloadHex string
	Samples    []SensorData
	Realtime   bool
}
