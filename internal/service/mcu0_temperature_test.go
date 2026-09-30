package service

import (
	"context"
	"errors"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
	"testing"
	"time"
)

type temperatureStore struct {
	batch domain.MCU0TemperatureBatch
	calls int
	err   error
}

func (s *temperatureStore) PersistMCU0TemperatureBatch(_ context.Context, b domain.MCU0TemperatureBatch) error {
	s.calls++
	s.batch = b
	return s.err
}

// Synthetic frames/policies for offline tests only: no production clock policy.
func TestMCU0IngestGatesPersistenceAndACK(t *testing.T) {
	raw := []byte{1, 2, 1, 12, 0, 0}
	for i := 0; i < 12; i++ {
		raw = append(raw, byte(i), 0, 0xFD)
	}
	raw = protocol.AppendCRC16Modbus(append(raw, 0xC0))
	clock := [3]byte{12, 0, 0}
	r := MCU0TemperatureResolution{GatewayID: 1, NodeID: 2, PointID: 3, Frequency: 1, DeviceExternalID: "test-only", IdentityConfirmed: true, TimeMappingConfirmed: true, LayoutConfirmed: true, ACKClock: &clock, ACKClockConfirmed: true}
	for i := 0; i < 12; i++ {
		r.SampleTimes = append(r.SampleTimes, time.Date(2026, 9, 30, 11, i*5, 0, 0, time.FixedZone("synthetic", 8*3600)))
	}
	for _, tc := range []struct {
		name       string
		unresolved bool
		fail       bool
	}{{"unresolved", true, false}, {"commit failed", false, true}, {"committed", false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			policy := r
			if tc.unresolved {
				policy.TimeMappingConfirmed = false
			}
			s := &temperatureStore{}
			if tc.fail {
				s.err = errors.New("commit failed")
			}
			out, err := IngestMCU0Temperature(context.Background(), s, 7, raw, true, policy)
			if tc.unresolved || tc.fail {
				if err == nil || len(out.ACK) != 0 {
					t.Fatal("unsafe ACK")
				}
				if tc.unresolved && s.calls != 0 {
					t.Fatal("unresolved projection")
				}
				return
			}
			if err != nil || len(out.ACK) != 8 || !protocol.ValidateCRC16Modbus(out.ACK) || len(s.batch.Samples) != 12 || s.batch.Realtime {
				t.Fatalf("%+v %v", out, err)
			}
			if s.batch.Samples[0].RealTemp != 25.3 || *s.batch.Samples[0].RawEventID != 7 {
				t.Fatal("wrong linked value")
			}
		})
	}
	r.ACKClockConfirmed = false
	s := &temperatureStore{}
	out, err := IngestMCU0Temperature(context.Background(), s, 8, raw, true, r)
	if err != nil || len(out.ACK) != 0 {
		t.Fatal("unconfirmed ACK clock")
	}
	manual := protocol.AppendCRC16Modbus([]byte{1, 2, 2, 12, 0, 0, 0xFF, 0xCE, 0xC0})
	r.SampleTimes = r.SampleTimes[:1]
	s = &temperatureStore{}
	out, err = IngestMCU0Temperature(context.Background(), s, 9, manual, true, r)
	if err != nil || !s.batch.Realtime || out.Mode != "manual" || len(out.ACK) != 0 || s.batch.Samples[0].RealTemp != -5 {
		t.Fatalf("manual %+v %v", out, err)
	}
}
