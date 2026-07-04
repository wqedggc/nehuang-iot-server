package main

import (
	"encoding/hex"
	"flag"
	"fmt"
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
	flag.Parse()

	frames, err := selectFrames(*frameName)
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
}

func selectFrames(name string) ([]sampleFrame, error) {
	all := []sampleFrame{
		{
			name:    "register",
			payload: []byte("FP_TEST_ADMIN_001\n"),
			note:    "ASCII register; Adapter forwards register to /api/v1/adapter/device-events",
		},
		{
			name:    "heartbeat",
			payload: []byte("HEARTBEAT\n"),
			note:    "ASCII heartbeat; Adapter forwards heartbeat after gateway_name is known",
		},
		{
			name:    "work",
			payload: protocol.AppendCRC16Modbus(mustHex("AAAA0B010203")),
			note:    "CRC-valid placeholder work frame; first version records raw only",
		},
		{
			name:    "alarm",
			payload: protocol.AppendCRC16Modbus(mustHex("AAAA0BFFFFFF01")),
			note:    "alarm-looking placeholder frame; first version records raw only",
		},
		{
			name:    "unknown",
			payload: mustHex("01020304"),
			note:    "unknown bytes; Adapter must keep raw journal and continue",
		},
	}

	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" || key == "all" {
		return all, nil
	}
	for _, frame := range all {
		if frame.name == key {
			return []sampleFrame{frame}, nil
		}
	}
	return nil, fmt.Errorf("unknown frame %q; use all, register, heartbeat, work, alarm, or unknown", name)
}

func mustHex(raw string) []byte {
	payload, err := hex.DecodeString(raw)
	if err != nil {
		panic(err)
	}
	return payload
}
