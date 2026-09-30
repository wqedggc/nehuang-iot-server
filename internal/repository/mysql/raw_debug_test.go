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
)

type rawTestState struct {
	duplicate, limited, failAudit bool
	inserts, commits, rollbacks   int
	claimed                       bool
}
type rawTestDriver struct{ s *rawTestState }
type rawTestConn struct{ s *rawTestState }
type rawTestTx struct{ s *rawTestState }
type rawTestRows struct {
	values []driver.Value
	done   bool
}

func (d rawTestDriver) Open(string) (driver.Conn, error) { return &rawTestConn{d.s}, nil }
func (c *rawTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c *rawTestConn) Close() error              { return nil }
func (c *rawTestConn) Begin() (driver.Tx, error) { return rawTestTx{c.s}, nil }
func (t rawTestTx) Commit() error                { t.s.commits++; return nil }
func (t rawTestTx) Rollback() error              { t.s.rollbacks++; return nil }
func (r *rawTestRows) Columns() []string {
	if len(r.values) == 3 {
		return []string{"a", "b", "c"}
	}
	return []string{"count"}
}
func (r *rawTestRows) Close() error { return nil }
func (r *rawTestRows) Next(v []driver.Value) error {
	if r.done || r.values == nil {
		return io.EOF
	}
	r.done = true
	copy(v, r.values)
	return nil
}
func (c *rawTestConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(q, "FOR UPDATE"):
		return &rawTestRows{values: []driver.Value{"TEST", "external", int64(1)}}, nil
	case strings.Contains(q, "command_id, command_hex"):
		if c.s.duplicate {
			return &rawTestRows{values: []driver.Value{int64(123), "0103", int64(2)}}, nil
		}
		return &rawTestRows{}, nil
	case strings.Contains(q, "COUNT(*)"):
		n := int64(0)
		if c.s.limited {
			n = 1
		}
		return &rawTestRows{values: []driver.Value{n}}, nil
	}
	return nil, errors.New("unexpected query")
}
func (c *rawTestConn) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(q, "UPDATE control_commands cc") {
		if c.s.claimed {
			return driver.RowsAffected(0), nil
		}
		c.s.claimed = true
		return driver.RowsAffected(1), nil
	}
	if strings.Contains(q, "INSERT INTO op_records") && c.s.failAudit {
		return nil, errors.New("audit failed")
	}
	if strings.Contains(q, "INSERT") {
		c.s.inserts++
		return driver.RowsAffected(1), nil
	}
	return nil, errors.New("unexpected exec")
}

var rawTestID atomic.Int64

func rawTestRepo(t *testing.T, s *rawTestState) *Repository {
	t.Helper()
	n := fmt.Sprintf("raw-test-%d", rawTestID.Add(1))
	sql.Register(n, rawTestDriver{s})
	db, e := sql.Open(n, "")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return &Repository{db: db}
}
func rawTestInput() domain.RawDebugCommandInput {
	return domain.RawDebugCommandInput{GatewayID: 1, GatewayName: "TEST", UserID: 2, Role: domain.RoleAdmin, CommandHex: "01 03", RequestID: strings.Repeat("a", 32)}
}
func TestRawDebugRepositoryAtomicAuditAndIdempotency(t *testing.T) {
	for _, tc := range []struct {
		name             string
		s                rawTestState
		err              error
		inserts, commits int
	}{{"new", rawTestState{}, nil, 2, 1}, {"duplicate", rawTestState{duplicate: true}, nil, 0, 1}, {"limited", rawTestState{limited: true}, domain.ErrRawDebugRateLimited, 0, 0}, {"audit-failure", rawTestState{failAudit: true}, errors.New("audit failed"), 1, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.s
			_, e := rawTestRepo(t, &s).CreateRawDebugCommand(context.Background(), rawTestInput())
			if tc.err == nil && e != nil {
				t.Fatal(e)
			}
			if tc.err != nil && (e == nil || e.Error() != tc.err.Error()) {
				t.Fatalf("error=%v", e)
			}
			if s.inserts != tc.inserts || s.commits != tc.commits {
				t.Fatalf("inserts=%d commits=%d", s.inserts, s.commits)
			}
			if tc.err != nil && s.rollbacks != 1 {
				t.Fatal("failed transaction did not roll back")
			}
		})
	}
}
func TestRawDebugClaimCanOnlySucceedOnce(t *testing.T) {
	s := rawTestState{}
	r := rawTestRepo(t, &s)
	if e := r.ClaimRawDebugCommand(context.Background(), 1, "external"); e != nil {
		t.Fatal(e)
	}
	if e := r.ClaimRawDebugCommand(context.Background(), 1, "external"); !errors.Is(e, domain.ErrRawDebugConflict) {
		t.Fatal(e)
	}
}
