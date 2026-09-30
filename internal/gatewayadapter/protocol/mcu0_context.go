package protocol

import (
	"fmt"
	"strings"
	"time"
)

var mcu0Beijing = time.FixedZone("Asia/Shanghai", 8*60*60)

// ResolveMCU0BinaryClock interprets ordinary binary HH/MM/SS, not BCD.
// beijingDate supplies explicitly verified Beijing calendar Y/M/D, not arrival date.
func ResolveMCU0BinaryClock(clock [3]byte, beijingDate time.Time, dateConfirmed bool) (time.Time, error) {
	if !dateConfirmed || beijingDate.IsZero() {
		return time.Time{}, fmt.Errorf("MCU0 Beijing date not confirmed")
	}
	if clock[0] > 23 || clock[1] > 59 || clock[2] > 59 {
		return time.Time{}, fmt.Errorf("invalid MCU0 binary clock")
	}
	return time.Date(beijingDate.Year(), beijingDate.Month(), beijingDate.Day(), int(clock[0]), int(clock[1]), int(clock[2]), 0, mcu0Beijing), nil
}

// MCU0Identity combines a verified cluster/gateway identity and an already-resolved
// numeric ID. It does not guess the raw ID byte order or allocate a database ID.
func MCU0Identity(cluster string, deviceID int, clusterConfirmed bool) (string, error) {
	if !clusterConfirmed || strings.TrimSpace(cluster) == "" {
		return "", fmt.Errorf("MCU0 cluster identity not confirmed")
	}
	if deviceID < 1 || deviceID > 100 {
		return "", fmt.Errorf("MCU0 device ID outside 1..100")
	}
	return fmt.Sprintf("%d:%s/%d", len(cluster), cluster, deviceID), nil
}

// MCU0HistoryWindow returns newest-first logical slots ONLY for a trusted,
// continuous, undelayed hourly batch. It does not map byte array/sequence order.
// Missing/intermittent records and delayed retransmissions must use another
// explicitly confirmed mapping; neither arrival time nor equal values fill gaps.
func MCU0HistoryWindow(exit time.Time, count int, continuousConfirmed, noDelayConfirmed bool) ([]time.Time, error) {
	if !continuousConfirmed || !noDelayConfirmed || exit.IsZero() {
		return nil, fmt.Errorf("MCU0 history continuity/no-delay not confirmed")
	}
	if count < 1 || count > 15 {
		return nil, fmt.Errorf("MCU0 history count outside 1..15")
	}
	local := exit.In(mcu0Beijing)
	if local.Minute() >= 5 {
		return nil, fmt.Errorf("MCU0 outlet time outside confirmed hourly window")
	}
	newest := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, mcu0Beijing)
	out := make([]time.Time, count)
	for i := range out {
		out[i] = newest.Add(-time.Duration(i) * 5 * time.Minute)
	}
	return out, nil
}
