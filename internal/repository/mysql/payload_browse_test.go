package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"nehuang-iot-server/internal/domain"
	"strings"
	"testing"
	"time"
)

type browseState struct {
	queries []string
	args    [][]driver.NamedValue
}
type browseDriver struct{ s *browseState }
type browseConn struct{ s *browseState }

func (d browseDriver) Open(string) (driver.Conn, error) { return &browseConn{d.s}, nil }
func (c *browseConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c *browseConn) Close() error              { return nil }
func (c *browseConn) Begin() (driver.Tx, error) { return nil, errors.New("read only") }
func (c *browseConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.s.queries = append(c.s.queries, q)
	c.s.args = append(c.s.args, args)
	if strings.Contains(q, "COUNT(*)") {
		return &dashboardRows{values: []driver.Value{int64(81)}}, nil
	}
	return &dashboardRows{values: []driver.Value{int64(7), "FP_TEST", int64(1), int64(3), int64(1), "", "message", int64(1), int64(0), "", int64(0), time.Now(), "7E03", "{\"direction\":\"inbound\"}"}}, nil
}
func TestPayloadBrowsingPagesBeforeLoadingPayloads(t *testing.T) {
	s := &browseState{}
	n := fmt.Sprintf("payload-browse-%d", rawTestID.Add(1))
	sql.Register(n, browseDriver{s})
	db, e := sql.Open(n, "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	source := uint8(1)
	start, end, keyword := "2026-10-01 00:00:00", "2026-10-02 00:00:00", "AA BB"
	p := domain.PayloadEventQuery{Page: 3, Size: 20, SourceType: &source, Start: &start, End: &end, Keyword: &keyword}
	items, total, e := New(db).ListPayloadEvents(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	if total != 81 || len(items) != 1 || items[0].RawEventID != 7 || items[0].PayloadHex != "7E03" {
		t.Fatalf("result changed: %v %d", items, total)
	}
	if len(s.queries) != 2 {
		t.Fatalf("queries=%d", len(s.queries))
	}
	for _, q := range s.queries {
		if !strings.Contains(q, "IGNORE INDEX (uk_raw_source_capture)") || !strings.Contains(q, "received_at_ms >= ?") || !strings.Contains(q, "received_at_ms <= ?") {
			t.Fatalf("browse filters/plan missing: %s", q)
		}
	}
	q := s.queries[1]
	inner := strings.Split(strings.Split(q, "JOIN (")[1], ") page")[0]
	if strings.Contains(inner, "payload_snapshot") || strings.Contains(strings.Split(inner, "FROM")[0], "payload_hex") {
		t.Fatal("wide payloads included in sort projection")
	}
	if !strings.Contains(q, "page.raw_event_id DESC") || !strings.Contains(q, "e.raw_event_id = page.raw_event_id") {
		t.Fatal("page ordering/join lost")
	}
	a := s.args[1]
	if a[len(a)-2].Value != int64(20) || a[len(a)-1].Value != int64(40) {
		t.Fatalf("pagination changed: %v", a)
	}
	for i := range s.args[0] {
		if s.args[0][i].Value != a[i].Value {
			t.Fatal("count/list filters differ")
		}
	}
}
