package protocol

import (
	"testing"
	"time"
)

func TestMCU0BinaryBeijingClock(t *testing.T) {
	d := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	got, err := ResolveMCU0BinaryClock([3]byte{0x17, 59, 59}, d, true)
	if err != nil || got.Hour() != 23 || got.Location().String() != "Asia/Shanghai" {
		t.Fatalf("%v %v", got, err)
	}
	if _, err = ResolveMCU0BinaryClock([3]byte{0x23, 0, 0}, d, true); err == nil {
		t.Fatal("invalid binary hour accepted")
	}
	if _, err = ResolveMCU0BinaryClock([3]byte{1, 0, 0}, d, false); err == nil {
		t.Fatal("unconfirmed date accepted")
	}
}
func TestMCU0IdentityIsClusterScoped(t *testing.T) {
	a, err := MCU0Identity("cluster-a", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := MCU0Identity("cluster-b", 1, true)
	if a == b {
		t.Fatal("clusters collided")
	}
	for _, id := range []int{0, 101} {
		if _, err := MCU0Identity("a", id, true); err == nil {
			t.Fatal("invalid ID")
		}
	}
	if _, err := MCU0Identity("a", 1, false); err == nil {
		t.Fatal("unverified gateway identity")
	}
}
func TestMCU0HistoryWindowRequiresContinuityAndNoDelay(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	exit := time.Date(2026, 10, 1, 1, 4, 59, 0, loc)
	times, err := MCU0HistoryWindow(exit, 12, true, true)
	if err != nil || len(times) != 12 || times[0].Hour() != 1 || times[0].Minute() != 0 || times[11].Hour() != 0 || times[11].Minute() != 5 {
		t.Fatalf("%v %v", times, err)
	}
	for _, count := range []int{1, 3, 15} {
		v, err := MCU0HistoryWindow(exit, count, true, true)
		if err != nil || len(v) != count {
			t.Fatal("count")
		}
	}
	midnight := time.Date(2026, 10, 1, 0, 1, 0, 0, loc)
	v, _ := MCU0HistoryWindow(midnight, 12, true, true)
	if v[11].Day() != 30 {
		t.Fatal("cross midnight")
	}
	for _, gate := range [][2]bool{{false, true}, {true, false}, {false, false}} {
		if _, err := MCU0HistoryWindow(exit, 12, gate[0], gate[1]); err == nil {
			t.Fatal("unknown batch accepted")
		}
	}
	if _, err := MCU0HistoryWindow(time.Date(2026, 10, 1, 1, 5, 0, 0, loc), 12, true, true); err == nil {
		t.Fatal("delayed exit accepted")
	}
}
