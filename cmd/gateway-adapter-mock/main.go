package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

type sampleFrame struct {
	name    string
	payload []byte
	note    string
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8081", "gateway adapter TCP address")
	frameName := flag.String("frame", "all", "frame to send: all, register, heartbeat, work, alarm, unknown")
	repeat := flag.Int("repeat", 1, "repeat count")
	delay := flag.Duration("delay", 300*time.Millisecond, "delay between frames")
	waitDownlink := flag.Bool("wait-downlink", false, "send bootstrap frames, then wait and print downlink bytes")
	waitTimeout := flag.Duration("wait-timeout", 0, "read timeout while waiting for downlink; 0 means wait until connection closes")
	flag.Parse()

	selectedFrameName := *frameName
	if *waitDownlink && strings.EqualFold(strings.TrimSpace(selectedFrameName), "all") {
		selectedFrameName = "register,heartbeat"
	}

	frames, err := selectFrames(selectedFrameName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *repeat <= 0 {
		*repeat = 1
	}

	conn, err := net.DialTimeout("tcp", *addr, 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect %s: %v\n", *addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	for i := 0; i < *repeat; i++ {
		for _, frame := range frames {
			if _, err := conn.Write(frame.payload); err != nil {
				fmt.Fprintf(os.Stderr, "send %s: %v\n", frame.name, err)
				os.Exit(1)
			}
			fmt.Printf("sent frame=%s bytes=%d hex=%s note=%s\n",
				frame.name,
				len(frame.payload),
				strings.ToUpper(hex.EncodeToString(frame.payload)),
				frame.note,
			)
			time.Sleep(*delay)
		}
	}

	if *waitDownlink {
		waitForDownlink(conn, *waitTimeout)
	}
}

func selectFrames(name string) ([]sampleFrame, error) {
	all := []sampleFrame{
		{
			name:    "register",
			payload: []byte("FP_TEST_ADMIN_002\n"),
			note:    "ASCII register; Adapter forwards raw bytes to /api/v1/adapter/raw-frames",
		},
		{
			name:    "heartbeat",
			payload: []byte("keepalive"),
			note:    "ASCII keepalive heartbeat; Adapter forwards raw bytes after gateway_name is known",
		},
		{
			name:    "heartbeat-message",
			payload: []byte("keepalive message"),
			note:    "ASCII keepalive message heartbeat variant observed in hardware debug output",
		},
		{
			name:    "modbus-read",
			payload: mustHex("03030000000185E8"),
			note:    "Modbus read holding register command sample; Server shows protocol summary only",
		},
		{
			name:    "work",
			payload: protocol.AppendCRC16Modbus(mustHex("AAAA0B010203")),
			note:    "CRC-valid placeholder work frame; Server keeps it pending until protocol fields freeze",
		},
		{
			name:    "alarm",
			payload: protocol.AppendCRC16Modbus(mustHex("AAAA0BFFFFFF01")),
			note:    "alarm-looking placeholder frame; Server keeps it pending until protocol fields freeze",
		},
		{
			name:    "unknown",
			payload: mustHex("01020304"),
			note:    "unknown bytes; Adapter keeps raw journal and Server stores raw frame",
		},
	}

	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" || key == "all" {
		return all, nil
	}
	byName := make(map[string]sampleFrame, len(all))
	for _, frame := range all {
		byName[frame.name] = frame
	}
	parts := strings.Split(key, ",")
	selected := make([]sampleFrame, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		frame, ok := byName[part]
		if !ok {
			return nil, fmt.Errorf("unknown frame %q; use all, register, heartbeat, heartbeat-message, modbus-read, work, alarm, or unknown", part)
		}
		selected = append(selected, frame)
	}
	return selected, nil
}

func mustHex(raw string) []byte {
	payload, err := hex.DecodeString(raw)
	if err != nil {
		panic(err)
	}
	return payload
}

func waitForDownlink(conn net.Conn, timeout time.Duration) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	buf := make([]byte, 4096)
	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetReadDeadline(deadline)
		}
		n, err := conn.Read(buf)
		if err != nil {
			if ok, err := isTimeout(err); ok {
				fmt.Fprintf(os.Stderr, "wait downlink timeout: %v\n", err)
				return
			}
			if err == io.EOF {
				fmt.Fprintln(os.Stderr, "connection closed by adapter")
				return
			}
			fmt.Fprintf(os.Stderr, "read downlink: %v\n", err)
			return
		}
		fmt.Printf("received downlink bytes=%d hex=%s\n", n, strings.ToUpper(hex.EncodeToString(buf[:n])))
	}
}

func isTimeout(err error) (bool, error) {
	if err == nil {
		return false, nil
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true, err
	}
	return false, err
}
