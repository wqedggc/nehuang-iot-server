package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"nehuang-iot-server/internal/domain"
	"strings"
	"sync"
	"testing"
	"time"
)

type lifecycleDriver struct{ state *lifecycleDB }
type lifecycleDB struct {
	status           domain.CommandStatus
	updates, results int
	finished         any
	timeoutArgs      []driver.NamedValue
}
type lifecycleConn struct{ state *lifecycleDB }
type lifecycleTx struct{}
type lifecycleRows struct {
	status domain.CommandStatus
	read   bool
}

func (d *lifecycleDriver) Open(string) (driver.Conn, error)  { return &lifecycleConn{d.state}, nil }
func (c *lifecycleConn) Prepare(string) (driver.Stmt, error) { panic("unexpected prepare") }
func (c *lifecycleConn) Close() error                        { return nil }
func (c *lifecycleConn) Begin() (driver.Tx, error)           { return lifecycleTx{}, nil }
func (lifecycleTx) Commit() error                            { return nil }
func (lifecycleTx) Rollback() error                          { return nil }
func (c *lifecycleConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(q, "FOR UPDATE") {
		panic("result must lock command")
	}
	return &lifecycleRows{status: c.state.status}, nil
}
func (r *lifecycleRows) Columns() []string {
	return strings.Split("id,gateway,realtime,node,point,frequency,device,source,type,mode,status,user", ",")
}
func (r *lifecycleRows) Close() error { return nil }
func (r *lifecycleRows) Next(v []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	copy(v, []driver.Value{int64(1), int64(2), nil, nil, nil, int64(1), "device", int64(1), int64(1), int64(1), int64(r.status), nil})
	return nil
}
func (c *lifecycleConn) ExecContext(_ context.Context, q string, a []driver.NamedValue) (driver.Result, error) {
	switch {
	case strings.Contains(q, "INSERT INTO control_results"):
		c.state.results++
	case strings.Contains(q, "COALESCE(finished_at_ms"):
		if !strings.Contains(q, "command_status IN (?, ?)") {
			panic("ack must share final-result TTL")
		}
		c.state.timeoutArgs = a
	case strings.Contains(q, "UPDATE control_commands"):
		c.state.updates++
		c.state.status = domain.CommandStatus(a[0].Value.(int64))
		c.state.finished = a[1].Value
	default:
		panic("unexpected SQL")
	}
	return driver.RowsAffected(1), nil
}

var lifecycleDriverMu sync.Mutex
var lifecycleDriverID int

func lifecycleRepo(t *testing.T, s *lifecycleDB) *Repository {
	t.Helper()
	lifecycleDriverMu.Lock()
	lifecycleDriverID++
	name := "control-lifecycle-" + time.Now().Format("150405.000000000")
	sql.Register(name, &lifecycleDriver{s})
	lifecycleDriverMu.Unlock()
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &Repository{db: db}
}
func TestControlResultsPreserveTerminalStateAndAudit(t *testing.T) {
	s := &lifecycleDB{status: domain.CommandStatusSent}
	r := lifecycleRepo(t, s)
	for _, status := range []domain.CommandStatus{domain.CommandStatusAck, domain.CommandStatusAck, domain.CommandStatusSucceeded, domain.CommandStatusAck, domain.CommandStatusFailed} {
		if err := r.ApplyControlCommandResult(context.Background(), domain.AdapterCommandResultInput{CommandID: 1, Status: status}); err != nil {
			t.Fatal(err)
		}
		if status == domain.CommandStatusAck && s.status == domain.CommandStatusAck && s.finished != nil {
			t.Fatal("ack set finished time")
		}
	}
	if s.status != domain.CommandStatusSucceeded || s.updates != 2 || s.results != 5 || s.finished == nil {
		t.Fatalf("unexpected state %+v", s)
	}
}
func TestControlTimeoutIncludesAckWithOriginalSendCutoff(t *testing.T) {
	s := &lifecycleDB{}
	r := lifecycleRepo(t, s)
	cutoff := time.Now()
	if _, err := r.MarkControlCommandsTimeout(context.Background(), cutoff); err != nil {
		t.Fatal(err)
	}
	a := s.timeoutArgs
	if len(a) != 9 || a[6].Value != int64(domain.CommandStatusSent) || a[7].Value != int64(domain.CommandStatusAck) || a[8].Value != cutoff {
		t.Fatalf("unexpected timeout args %v", a)
	}
}
