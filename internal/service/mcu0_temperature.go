package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
	"strings"
	"time"
)

// MCU0TemperatureStore must atomically commit all samples and parse state, and
// suppress duplicate projection of the same raw event. Success means durable commit.
type MCU0TemperatureStore interface {
	PersistMCU0TemperatureBatch(context.Context, domain.MCU0TemperatureBatch) error
}

// MCU0TemperatureResolution comes only from operator-validated identity and time
// policy, never from arrival time or a guessed ID endian/date mapping.
type MCU0TemperatureResolution struct {
	GatewayID, NodeID, PointID                               uint64
	Frequency                                                int
	DeviceExternalID                                         string
	SampleTimes                                              []time.Time
	IdentityConfirmed, TimeMappingConfirmed, LayoutConfirmed bool
	ACKClock                                                 *[3]byte
	ACKClockConfirmed                                        bool
}
type MCU0TemperatureResult struct {
	Mode    string
	Samples int
	ACK     []byte
}

func IngestMCU0Temperature(ctx context.Context, store MCU0TemperatureStore, rawEventID uint64, raw []byte, boundaryConfirmed bool, r MCU0TemperatureResolution) (MCU0TemperatureResult, error) {
	var result MCU0TemperatureResult
	f, err := protocol.DecodeMCU0TemperatureFrame(raw, boundaryConfirmed)
	if err != nil {
		return result, err
	}
	result.Mode = f.Mode
	if !r.IdentityConfirmed || !r.TimeMappingConfirmed || !r.LayoutConfirmed {
		return result, fmt.Errorf("MCU0 identity/time/layout policy not confirmed")
	}
	if rawEventID == 0 || r.GatewayID == 0 || r.NodeID == 0 || r.PointID == 0 || r.Frequency <= 0 || r.DeviceExternalID == "" {
		return result, fmt.Errorf("MCU0 resolved identity incomplete")
	}
	if len(f.Samples) == 0 {
		return result, fmt.Errorf("MCU0 frame has no temperature samples")
	}
	if f.Tag.NTCFault {
		return result, fmt.Errorf("MCU0 NTC fault: temperature not projected")
	}
	if len(r.SampleTimes) != len(f.Samples) {
		return result, fmt.Errorf("MCU0 sample time mapping incomplete")
	}
	batch := domain.MCU0TemperatureBatch{RawEventID: rawEventID, PayloadHex: strings.ToUpper(hex.EncodeToString(raw)), Realtime: f.Mode == "manual"}
	for i, s := range f.Samples {
		if r.SampleTimes[i].IsZero() {
			return result, fmt.Errorf("MCU0 sample time missing")
		}
		v := s.Celsius
		batch.Samples = append(batch.Samples, domain.SensorData{GatewayID: r.GatewayID, NodeID: &r.NodeID, PointID: &r.PointID, Frequency: r.Frequency, DeviceExternalID: r.DeviceExternalID, SampledAtMS: r.SampleTimes[i], RealValue: &v, RealTemp: v, FunctionType: domain.FuncTypeTemperature, Unit: "℃", RawEventID: &rawEventID})
	}
	if store == nil {
		return result, fmt.Errorf("MCU0 temperature store unavailable")
	}
	if err := store.PersistMCU0TemperatureBatch(ctx, batch); err != nil {
		return result, err
	}
	result.Samples = len(batch.Samples)
	if f.Mode == "automatic" && r.ACKClockConfirmed && r.ACKClock != nil {
		result.ACK = protocol.BuildMCU0HistoryACK(f.DeviceID, *r.ACKClock)
	}
	return result, nil
}
