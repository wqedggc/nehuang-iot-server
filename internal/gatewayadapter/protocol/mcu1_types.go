package protocol

// MCU1Direction identifies whether an MCU1 message travels to or from the MCU.
type MCU1Direction uint8

const (
	MCU1DirectionDownlink MCU1Direction = iota + 1
	MCU1DirectionUplink
)

// MCU1MessageType names a message by the combination of its direction and BB.
type MCU1MessageType uint8

const (
	MCU1MessageDownlinkInitialization MCU1MessageType = iota + 1
	MCU1MessageDownlinkAutomaticConfigAndRead
	MCU1MessageDownlinkHistoryReceived
	MCU1MessageDownlinkManualControl
	MCU1MessageUplinkAutomaticStatusAndHistory
	MCU1MessageUplinkManualStatus
	MCU1MessageUplinkFaultStatus
)

// MCU1Tag is the named form of the one-byte 1/N/R/S/V/W/T/U status bitmap.
// NTCFault is intentionally fault-oriented: it is true when the raw N bit is 0.
type MCU1Tag struct {
	NTCFault            bool
	RailFault           bool
	OvertravelFault     bool
	UpperLimitTriggered bool
	LowerLimitTriggered bool
	OpenRelayActive     bool
	CloseRelayActive    bool
}
