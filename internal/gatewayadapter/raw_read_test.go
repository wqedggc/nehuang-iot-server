package gatewayadapter

import (
	"io"
	"nehuang-iot-server/internal/gatewayadapter/journal"
	"net"
	"testing"
	"time"
)

type lastReadConn struct {
	net.Conn
	bytes []byte
	reads int
}

func (c *lastReadConn) Read(b []byte) (int, error) {
	c.reads++
	if c.reads > 1 {
		return 0, io.EOF
	}
	return copy(b, c.bytes), io.EOF
}
func (c *lastReadConn) Close() error { return nil }
func (c *lastReadConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234}
}
func (c *lastReadConn) SetReadDeadline(time.Time) error { return nil }
func TestLastTCPReadBytesAreJournaledBeforeEOF(t *testing.T) {
	jw, e := journal.NewWriter(t.TempDir(), 1024*1024)
	if e != nil {
		t.Fatal(e)
	}
	defer jw.Close()
	s := NewServer(Config{ReadBufferSize: 64, RawDiskMaxUsedPct: 100}, jw, NewLogger(io.Discard))
	c := &lastReadConn{bytes: []byte{0x7e, 0x03, 0xde, 0xad, 0xbe}}
	s.handleConn(c)
	records := jw.Pending(10)
	if len(records) != 1 || records[0].PayloadHex != "7E03DEADBE" || records[0].CaptureKind != "tcp_read" {
		t.Fatalf("final bytes lost: %#v", records)
	}
}
