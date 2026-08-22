package protocol

import (
	"testing"
)

func TestDecodeMCU1Tag(t *testing.T) {
	tests := []struct {
		name string
		raw  byte
		want MCU1Tag
	}{
		{
			name: "normal and stopped",
			raw:  0xC0,
			want: MCU1Tag{},
		},
		{
			name: "NTC fault uses inverse N bit semantics",
			raw:  0x80,
			want: MCU1Tag{NTCFault: true},
		},
		{
			name: "faults and limits can coexist while stopped",
			raw:  0xFC,
			want: MCU1Tag{
				RailFault:           true,
				OvertravelFault:     true,
				UpperLimitTriggered: true,
				LowerLimitTriggered: true,
			},
		},
		{
			name: "opening relay active",
			raw:  0xC2,
			want: MCU1Tag{OpenRelayActive: true},
		},
		{
			name: "closing relay active",
			raw:  0xC1,
			want: MCU1Tag{CloseRelayActive: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeMCU1Tag(tt.raw)
			if err != nil {
				t.Fatalf("DecodeMCU1Tag(0x%02X) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("DecodeMCU1Tag(0x%02X) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestEncodeMCU1Tag(t *testing.T) {
	tests := []struct {
		name string
		tag  MCU1Tag
		want byte
	}{
		{name: "normal and stopped", tag: MCU1Tag{}, want: 0xC0},
		{name: "NTC fault clears N bit", tag: MCU1Tag{NTCFault: true}, want: 0x80},
		{
			name: "faults and limits while stopped",
			tag: MCU1Tag{
				RailFault:           true,
				OvertravelFault:     true,
				UpperLimitTriggered: true,
				LowerLimitTriggered: true,
			},
			want: 0xFC,
		},
		{name: "opening relay active", tag: MCU1Tag{OpenRelayActive: true}, want: 0xC2},
		{name: "closing relay active", tag: MCU1Tag{CloseRelayActive: true}, want: 0xC1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeMCU1Tag(tt.tag)
			if err != nil {
				t.Fatalf("EncodeMCU1Tag(%+v) error = %v", tt.tag, err)
			}
			if got != tt.want {
				t.Fatalf("EncodeMCU1Tag(%+v) = 0x%02X, want 0x%02X", tt.tag, got, tt.want)
			}
		})
	}
}

func TestMCU1TagRejectsInvalidCombinations(t *testing.T) {
	tests := []struct {
		name string
		raw  byte
		tag  MCU1Tag
	}{
		{
			name: "fixed high bit is clear",
			raw:  0x40,
			tag:  MCU1Tag{},
		},
		{
			name: "opening and closing relays both active",
			raw:  0xC3,
			tag:  MCU1Tag{OpenRelayActive: true, CloseRelayActive: true},
		},
		{
			name: "upper limit blocks opening direction",
			raw:  0xCA,
			tag:  MCU1Tag{UpperLimitTriggered: true, OpenRelayActive: true},
		},
		{
			name: "lower limit blocks closing direction",
			raw:  0xC5,
			tag:  MCU1Tag{LowerLimitTriggered: true, CloseRelayActive: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" decode", func(t *testing.T) {
			if _, err := DecodeMCU1Tag(tt.raw); err == nil {
				t.Fatalf("DecodeMCU1Tag(0x%02X) error = nil, want rejection", tt.raw)
			}
		})

		if tt.name == "fixed high bit is clear" {
			continue
		}
		t.Run(tt.name+" encode", func(t *testing.T) {
			if _, err := EncodeMCU1Tag(tt.tag); err == nil {
				t.Fatalf("EncodeMCU1Tag(%+v) error = nil, want rejection", tt.tag)
			}
		})
	}
}

func TestDecodeMCU1MessageType(t *testing.T) {
	tests := []struct {
		name      string
		direction MCU1Direction
		bb        byte
		want      MCU1MessageType
	}{
		{name: "downlink initialization", direction: MCU1DirectionDownlink, bb: 0x00, want: MCU1MessageDownlinkInitialization},
		{name: "downlink automatic config and read", direction: MCU1DirectionDownlink, bb: 0x01, want: MCU1MessageDownlinkAutomaticConfigAndRead},
		{name: "downlink history received", direction: MCU1DirectionDownlink, bb: 0x02, want: MCU1MessageDownlinkHistoryReceived},
		{name: "downlink manual control", direction: MCU1DirectionDownlink, bb: 0x03, want: MCU1MessageDownlinkManualControl},
		{name: "uplink automatic status and history", direction: MCU1DirectionUplink, bb: 0x01, want: MCU1MessageUplinkAutomaticStatusAndHistory},
		{name: "uplink manual status", direction: MCU1DirectionUplink, bb: 0x02, want: MCU1MessageUplinkManualStatus},
		{name: "uplink fault status", direction: MCU1DirectionUplink, bb: 0x03, want: MCU1MessageUplinkFaultStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeMCU1MessageType(tt.direction, tt.bb)
			if err != nil {
				t.Fatalf("DecodeMCU1MessageType(%d, 0x%02X) error = %v", tt.direction, tt.bb, err)
			}
			if got != tt.want {
				t.Fatalf("DecodeMCU1MessageType(%d, 0x%02X) = %d, want %d", tt.direction, tt.bb, got, tt.want)
			}

			direction, bb, err := EncodeMCU1MessageType(got)
			if err != nil {
				t.Fatalf("EncodeMCU1MessageType(%d) error = %v", got, err)
			}
			if direction != tt.direction || bb != tt.bb {
				t.Fatalf("EncodeMCU1MessageType(%d) = (%d, 0x%02X), want (%d, 0x%02X)", got, direction, bb, tt.direction, tt.bb)
			}
		})
	}
}

func TestMCU1MessageTypeRejectsUnknownPairsAndTypes(t *testing.T) {
	tests := []struct {
		name      string
		direction MCU1Direction
		bb        byte
	}{
		{name: "unknown direction", direction: MCU1Direction(0), bb: 0x01},
		{name: "unknown downlink BB", direction: MCU1DirectionDownlink, bb: 0x04},
		{name: "undefined uplink BB zero", direction: MCU1DirectionUplink, bb: 0x00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeMCU1MessageType(tt.direction, tt.bb); err == nil {
				t.Fatalf("DecodeMCU1MessageType(%d, 0x%02X) error = nil, want rejection", tt.direction, tt.bb)
			}
		})
	}

	if _, _, err := EncodeMCU1MessageType(MCU1MessageType(0)); err == nil {
		t.Fatal("EncodeMCU1MessageType(0) error = nil, want rejection")
	}
}
