package mysql

import (
	"testing"

	"nehuang-iot-server/internal/domain"
)

func TestNewDeviceRealtimeMapProjectionUsesPointIdentity(t *testing.T) {
	realValue := 23.5
	point := &domain.DevicePoint{
		PointID:          2002,
		NodeID:           1001,
		GatewayID:        9001,
		Frequency:        433,
		DeviceExternalID: "dev-7",
		FunctionType:     domain.FuncTypeTemperature,
		ChannelNo:        2,
		Unit:             "C",
		RealValue:        &realValue,
		RealTemp:         23.5,
		ValueStatus:      domain.ValueStatusNormal,
	}

	projection := newDeviceRealtimeMapProjection(point)

	if projection.PointID != point.PointID {
		t.Fatalf("projection point id: want %d, got %d", point.PointID, projection.PointID)
	}
	if projection.ChannelNo != point.ChannelNo {
		t.Fatalf("projection channel no: want %d, got %d", point.ChannelNo, projection.ChannelNo)
	}
	if projection.FunctionType != uint8(point.FunctionType) {
		t.Fatalf("projection function type: want %d, got %d", point.FunctionType, projection.FunctionType)
	}
}
