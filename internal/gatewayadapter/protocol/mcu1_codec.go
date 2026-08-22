package protocol

import (
	"errors"
	"fmt"
)

const (
	mcu1TagMarkerMask     byte = 0x80
	mcu1TagNTCOnlineMask  byte = 0x40
	mcu1TagRailFaultMask  byte = 0x20
	mcu1TagOvertravelMask byte = 0x10
	mcu1TagUpperLimitMask byte = 0x08
	mcu1TagLowerLimitMask byte = 0x04
	mcu1TagOpenRelayMask  byte = 0x02
	mcu1TagCloseRelayMask byte = 0x01
)

// DecodeMCU1Tag decodes and validates a one-byte 1/N/R/S/V/W/T/U bitmap.
func DecodeMCU1Tag(raw byte) (MCU1Tag, error) {
	if raw&mcu1TagMarkerMask == 0 {
		return MCU1Tag{}, errors.New("invalid MCU1 Tag: fixed high bit is not set")
	}

	tag := MCU1Tag{
		NTCFault:            raw&mcu1TagNTCOnlineMask == 0,
		RailFault:           raw&mcu1TagRailFaultMask != 0,
		OvertravelFault:     raw&mcu1TagOvertravelMask != 0,
		UpperLimitTriggered: raw&mcu1TagUpperLimitMask != 0,
		LowerLimitTriggered: raw&mcu1TagLowerLimitMask != 0,
		OpenRelayActive:     raw&mcu1TagOpenRelayMask != 0,
		CloseRelayActive:    raw&mcu1TagCloseRelayMask != 0,
	}
	if err := validateMCU1Tag(tag); err != nil {
		return MCU1Tag{}, err
	}
	return tag, nil
}

// EncodeMCU1Tag validates a named MCU1 Tag and encodes it as 1/N/R/S/V/W/T/U.
func EncodeMCU1Tag(tag MCU1Tag) (byte, error) {
	if err := validateMCU1Tag(tag); err != nil {
		return 0, err
	}

	raw := mcu1TagMarkerMask
	if !tag.NTCFault {
		raw |= mcu1TagNTCOnlineMask
	}
	if tag.RailFault {
		raw |= mcu1TagRailFaultMask
	}
	if tag.OvertravelFault {
		raw |= mcu1TagOvertravelMask
	}
	if tag.UpperLimitTriggered {
		raw |= mcu1TagUpperLimitMask
	}
	if tag.LowerLimitTriggered {
		raw |= mcu1TagLowerLimitMask
	}
	if tag.OpenRelayActive {
		raw |= mcu1TagOpenRelayMask
	}
	if tag.CloseRelayActive {
		raw |= mcu1TagCloseRelayMask
	}
	return raw, nil
}

func validateMCU1Tag(tag MCU1Tag) error {
	if tag.OpenRelayActive && tag.CloseRelayActive {
		return errors.New("invalid MCU1 Tag: opening and closing relays are both active")
	}
	if tag.UpperLimitTriggered && tag.OpenRelayActive {
		return errors.New("invalid MCU1 Tag: upper limit blocks opening relay")
	}
	if tag.LowerLimitTriggered && tag.CloseRelayActive {
		return errors.New("invalid MCU1 Tag: lower limit blocks closing relay")
	}
	return nil
}

// DecodeMCU1MessageType resolves the named message type for a direction and BB.
func DecodeMCU1MessageType(direction MCU1Direction, bb byte) (MCU1MessageType, error) {
	switch direction {
	case MCU1DirectionDownlink:
		switch bb {
		case 0x00:
			return MCU1MessageDownlinkInitialization, nil
		case 0x01:
			return MCU1MessageDownlinkAutomaticConfigAndRead, nil
		case 0x02:
			return MCU1MessageDownlinkHistoryReceived, nil
		case 0x03:
			return MCU1MessageDownlinkManualControl, nil
		}
	case MCU1DirectionUplink:
		switch bb {
		case 0x01:
			return MCU1MessageUplinkAutomaticStatusAndHistory, nil
		case 0x02:
			return MCU1MessageUplinkManualStatus, nil
		case 0x03:
			return MCU1MessageUplinkFaultStatus, nil
		}
	default:
		return 0, fmt.Errorf("invalid MCU1 direction: %d", direction)
	}
	return 0, fmt.Errorf("unsupported MCU1 message: direction=%d BB=0x%02X", direction, bb)
}

// EncodeMCU1MessageType returns the direction and BB represented by messageType.
func EncodeMCU1MessageType(messageType MCU1MessageType) (MCU1Direction, byte, error) {
	switch messageType {
	case MCU1MessageDownlinkInitialization:
		return MCU1DirectionDownlink, 0x00, nil
	case MCU1MessageDownlinkAutomaticConfigAndRead:
		return MCU1DirectionDownlink, 0x01, nil
	case MCU1MessageDownlinkHistoryReceived:
		return MCU1DirectionDownlink, 0x02, nil
	case MCU1MessageDownlinkManualControl:
		return MCU1DirectionDownlink, 0x03, nil
	case MCU1MessageUplinkAutomaticStatusAndHistory:
		return MCU1DirectionUplink, 0x01, nil
	case MCU1MessageUplinkManualStatus:
		return MCU1DirectionUplink, 0x02, nil
	case MCU1MessageUplinkFaultStatus:
		return MCU1DirectionUplink, 0x03, nil
	default:
		return 0, 0, fmt.Errorf("invalid MCU1 message type: %d", messageType)
	}
}
