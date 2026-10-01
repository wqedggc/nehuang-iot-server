package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

type dashboardDriver struct{ state *dashboardSQLState }
type dashboardSQLState struct {
	calls  int
	empty  bool
	cutoff time.Time
}
type dashboardConn struct{ state *dashboardSQLState }
type dashboardRows struct {
	values []driver.Value
	done   bool
}

func (d dashboardDriver) Open(string) (driver.Conn, error) { return &dashboardConn{d.state}, nil }
func (c *dashboardConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c *dashboardConn) Close() error { return nil }
func (c *dashboardConn) Begin() (driver.Tx, error) {
	return nil, errors.New("dashboard must be read-only")
}
func (r *dashboardRows) Columns() []string {
	v := make([]string, len(r.values))
	for i := range v {
		v[i] = fmt.Sprint(i)
	}
	return v
}
func (r *dashboardRows) Close() error { return nil }
func (r *dashboardRows) Next(v []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	copy(v, r.values)
	return nil
}
func (c *dashboardConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.calls++
	if len(args) != 1 || args[0].Value != c.state.cutoff {
		return nil, errors.New("range cutoff missing or changed")
	}
	var v []driver.Value
	switch {
	case strings.Contains(q, "SUM(event_type = 1)"):
		if !strings.Contains(q, "SUM(event_type = 4)") || !strings.Contains(q, "received_at_ms >= ?") {
			return nil, errors.New("event aggregation incomplete")
		}
		v = []driver.Value{int64(11), int64(2), int64(3), int64(4), int64(1)}
	case strings.Contains(q, "SUM(parse_status = 2)"):
		if !strings.Contains(q, "event_type = 2") || !strings.Contains(q, "SUM(parse_status = 3)") {
			return nil, errors.New("heartbeat filter incomplete")
		}
		v = []driver.Value{int64(3), int64(1), int64(1)}
	case strings.Contains(q, "FROM sensor_data"):
		v = []driver.Value{int64(5)}
	default:
		return nil, errors.New("unexpected repeated dashboard scan")
	}
	if c.state.empty {
		if len(v) > 1 && !strings.Contains(q, "COALESCE") {
			return nil, errors.New("empty SUM must be coalesced")
		}
		for i := range v {
			v[i] = int64(0)
		}
	}
	return &dashboardRows{values: v}, nil
}
func TestDashboardAggregatesBoundedScansAndEmptyRanges(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprint(empty), func(t *testing.T) {
			s := &dashboardSQLState{empty: empty, cutoff: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
			name := fmt.Sprintf("dashboard-%d", rawTestID.Add(1))
			sql.Register(name, dashboardDriver{s})
			db, e := sql.Open(name, "")
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			r := New(db)
			raw, telemetry, register, heartbeat, work, result, e := r.CountIngest(context.Background(), s.cutoff)
			if e != nil {
				t.Fatal(e)
			}
			total, success, failed, e := r.CountHeartbeatStats(context.Background(), s.cutoff)
			if e != nil {
				t.Fatal(e)
			}
			want := []int64{11, 5, 2, 3, 4, 1, 3, 1, 1}
			got := []int64{raw, telemetry, register, heartbeat, work, result, total, success, failed}
			for i := range got {
				if empty {
					want[i] = 0
				}
				if got[i] != want[i] {
					t.Fatalf("metric %d: got %d want %d", i, got[i], want[i])
				}
			}
			if s.calls != 3 {
				t.Fatalf("dashboard ingest/heartbeat used %d scans, want 3", s.calls)
			}
		})
	}
}
