package mysql

import (
	"testing"

	"nehuang-iot-server/internal/domain"
)

func TestBuildDevicePointConfigUpdateIncludesOnlyPointFields(t *testing.T) {
	pointName := "A棚温度"
	shedNo := "A棚"
	unit := "C"
	runMode := domain.RunModeManu
	controllable := uint8(1)

	sets, args := buildDevicePointConfigUpdate(domain.DevicePointUpdateInput{
		PointName:          &pointName,
		ShedNo:             &shedNo,
		Unit:               &unit,
		RunMode:            &runMode,
		ControllableStatus: &controllable,
	})

	if len(sets) != 4 {
		t.Fatalf("point sets: want 4, got %d: %v", len(sets), sets)
	}
	if len(args) != 4 {
		t.Fatalf("point args: want 4, got %d: %v", len(args), args)
	}
	for _, clause := range sets {
		if clause == "shed_no = ?" {
			t.Fatalf("shed_no belongs to collector_nodes, not device_points")
		}
	}
}

func TestBuildRealtimeMapConfigUpdateIncludesCompatibilityFields(t *testing.T) {
	shedNo := "A棚"
	unit := "C"
	bestMin := 18.0
	bestMax := 28.0
	pollInterval := 300
	pointStatus := uint8(1)

	sets, args := buildRealtimeMapConfigUpdate(domain.DevicePointUpdateInput{
		ShedNo:       &shedNo,
		Unit:         &unit,
		BestMin:      &bestMin,
		BestMax:      &bestMax,
		PollInterval: &pollInterval,
		PointStatus:  &pointStatus,
	})

	if len(sets) != 5 {
		t.Fatalf("realtime map sets: want 5, got %d: %v", len(sets), sets)
	}
	if len(args) != 5 {
		t.Fatalf("realtime map args: want 5, got %d: %v", len(args), args)
	}
	for _, clause := range sets {
		if clause == "point_status = ?" {
			t.Fatalf("point_status should not be mirrored to device_realtime_maps")
		}
	}
}
