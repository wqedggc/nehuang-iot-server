package mysql

import "testing"

func TestResolveDeviceTreeBoundUserUsesPointBindingFirst(t *testing.T) {
	pointBound := map[uint64]uint64{
		1001: 11,
	}
	gatewayBound := map[uint64]uint64{
		2001: 22,
	}

	userID, ok := resolveDeviceTreeBoundUser(pointBound, gatewayBound, 2001, 1001)
	if !ok {
		t.Fatalf("expected point to be bound")
	}
	if userID != 11 {
		t.Fatalf("expected point binding user 11 to take precedence, got %d", userID)
	}
}

func TestResolveDeviceTreeBoundUserFallsBackToGatewayBinding(t *testing.T) {
	pointBound := map[uint64]uint64{}
	gatewayBound := map[uint64]uint64{
		2001: 22,
	}

	userID, ok := resolveDeviceTreeBoundUser(pointBound, gatewayBound, 2001, 1001)
	if !ok {
		t.Fatalf("expected point to inherit gateway binding")
	}
	if userID != 22 {
		t.Fatalf("expected gateway binding user 22, got %d", userID)
	}
}

func TestResolveDeviceTreeBoundUserUnbound(t *testing.T) {
	userID, ok := resolveDeviceTreeBoundUser(map[uint64]uint64{}, map[uint64]uint64{}, 2001, 1001)
	if ok {
		t.Fatalf("expected point to be unbound")
	}
	if userID != 0 {
		t.Fatalf("expected zero user id for unbound point, got %d", userID)
	}
}
