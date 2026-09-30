package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"nehuang-iot-server/internal/domain"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A transactional in-memory driver verifies commit/rollback without real MySQL.
type tempTestState struct {
	status                 uint8
	hex                    string
	stored, live           int
	failInsert, failCommit bool
	commits, rollbacks     int
}
type tempTestDriver struct{ s *tempTestState }
type tempTestConn struct {
	s             *tempTestState
	inserts, live int
	parsed        bool
}
type tempTestRows struct {
	values []driver.Value
	done   bool
}

func (d tempTestDriver) Open(string) (driver.Conn, error)   { return &tempTestConn{s: d.s}, nil }
func (c *tempTestConn) Prepare(string) (driver.Stmt, error) { panic("unexpected prepare") }
func (c *tempTestConn) Close() error                        { return nil }
func (c *tempTestConn) Begin() (driver.Tx, error) {
	c.inserts = 0
	c.live = 0
	c.parsed = false
	return c, nil
}
func (c *tempTestConn) Commit() error {
	if c.s.failCommit {
		return errors.New("commit failure")
	}
	c.s.stored += c.inserts
	c.s.live += c.live
	if c.parsed {
		c.s.status = 2
	}
	c.s.commits++
	return nil
}
func (c *tempTestConn) Rollback() error { c.s.rollbacks++; return nil }
func (c *tempTestConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(q, "FOR UPDATE") {
		panic("missing lock")
	}
	if strings.Contains(q, "raw_device_events") {
		return &tempTestRows{values: []driver.Value{int64(c.s.status), c.s.hex}}, nil
	}
	return &tempTestRows{values: []driver.Value{int64(3)}}, nil
}
func (r *tempTestRows) Columns() []string {
	if len(r.values) == 2 {
		return []string{"status", "hex"}
	}
	return []string{"point"}
}
func (r *tempTestRows) Close() error { return nil }
func (r *tempTestRows) Next(v []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	copy(v, r.values)
	return nil
}
func (c *tempTestConn) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	switch {
	case strings.Contains(q, "INSERT INTO sensor_data"):
		c.inserts++
		if c.s.failInsert && c.inserts == 2 {
			return nil, errors.New("insert failure")
		}
	case strings.Contains(q, "UPDATE device_points"):
		if !strings.Contains(q, "last_report_at_ms <= ?") {
			panic("late realtime must not overwrite latest")
		}
		c.live++
	case strings.Contains(q, "UPDATE raw_device_events"):
		c.parsed = true
	default:
		panic("unexpected SQL")
	}
	return driver.RowsAffected(1), nil
}

var tempTestID atomic.Int64

func tempRepo(t *testing.T, s *tempTestState) *Repository {
	t.Helper()
	name := fmt.Sprintf("mcu0-temp-%d", tempTestID.Add(1))
	sql.Register(name, tempTestDriver{s})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &Repository{db: db}
}
func TestMCU0TemperatureTransactionAndDuplicateProjection(t *testing.T) {
	node, point, raw := uint64(2), uint64(3), uint64(7)
	v := 25.3
	for _, tc := range []struct {
		name                             string
		realtime, failInsert, failCommit bool
	}{{"history", false, false, false}, {"realtime", true, false, false}, {"partial insert rollback", false, true, false}, {"commit failure", false, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := &tempTestState{status: 1, hex: "0102", failInsert: tc.failInsert, failCommit: tc.failCommit}
			r := tempRepo(t, s)
			b := domain.MCU0TemperatureBatch{RawEventID: raw, PayloadHex: "0102", Realtime: tc.realtime}
			for i := 0; i < 2; i++ {
				b.Samples = append(b.Samples, domain.SensorData{GatewayID: 1, NodeID: &node, PointID: &point, Frequency: 1, DeviceExternalID: "synthetic", RawEventID: &raw, RealValue: &v, RealTemp: v, SampledAtMS: time.Now(), FunctionType: 1})
			}
			err := r.PersistMCU0TemperatureBatch(context.Background(), b)
			if tc.failInsert || tc.failCommit {
				if err == nil || s.stored != 0 || s.status != 1 {
					t.Fatalf("non-atomic %+v %v", s, err)
				}
				return
			}
			if err != nil || s.stored != 2 || s.status != 2 {
				t.Fatalf("%+v %v", s, err)
			}
			if !tc.realtime && s.live != 0 {
				t.Fatal("history overwrote realtime")
			}
			if err = r.PersistMCU0TemperatureBatch(context.Background(), b); err != nil || s.stored != 2 {
				t.Fatal("duplicate projection")
			}
			b.PayloadHex = "DEAD"
			if r.PersistMCU0TemperatureBatch(context.Background(), b) == nil {
				t.Fatal("wrong raw payload accepted")
			}
		})
	}
}
