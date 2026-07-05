package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	gatewayName := flag.String("gateway", "FP_TEST_ADMIN_002", "gateway name used in the register frame")
	frameName := flag.String("frame", "all", "frame to send: all, register, heartbeat, heartbeat-message, modbus-read, work, alarm, unknown")
	repeat := flag.Int("repeat", 1, "repeat count")
	delay := flag.Duration("delay", 300*time.Millisecond, "delay between frames")
	loop := flag.Bool("loop", false, "keep the TCP connection open and send periodic mock frames")
	duration := flag.Duration("duration", 0, "loop duration; 0 means run until interrupted")
	heartbeatInterval := flag.Duration("heartbeat-interval", 10*time.Second, "heartbeat interval in loop mode")
	workInterval := flag.Duration("work-interval", 0, "work frame interval in loop mode; 0 disables periodic work frames")
	alarmInterval := flag.Duration("alarm-interval", 0, "alarm frame interval in loop mode; 0 disables periodic alarm frames")
	unknownInterval := flag.Duration("unknown-interval", 0, "unknown frame interval in loop mode; 0 disables periodic unknown frames")
	modbusReadInterval := flag.Duration("modbus-read-interval", 0, "modbus-read frame interval in loop mode; 0 disables periodic modbus-read frames")
	waitDownlink := flag.Bool("wait-downlink", false, "send bootstrap frames, then wait and print downlink bytes")
	waitTimeout := flag.Duration("wait-timeout", 0, "read timeout while waiting for downlink; 0 means wait until connection closes")
	flag.Parse()

	selectedFrameName := *frameName
	if *waitDownlink && strings.EqualFold(strings.TrimSpace(selectedFrameName), "all") {
		selectedFrameName = "register,heartbeat"
	}

	frames, err := selectFrames(selectedFrameName, *gatewayName)
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

	if *loop {
		if err := runLoop(conn, *gatewayName, *duration, *heartbeatInterval, *workInterval, *alarmInterval, *unknownInterval, *modbusReadInterval, *waitDownlink, *waitTimeout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	for i := 0; i < *repeat; i++ {
		for _, frame := range frames {
			if err := sendFrame(conn, frame); err != nil {
				fmt.Fprintf(os.Stderr, "send %s: %v\n", frame.name, err)
				os.Exit(1)
			}
			time.Sleep(*delay)
		}
	}

	if *waitDownlink {
		waitForDownlink(conn, *waitTimeout)
	}
}

func selectFrames(name string, gatewayName string) ([]sampleFrame, error) {
	gatewayName = strings.TrimSpace(gatewayName)
	if gatewayName == "" {
		gatewayName = "FP_TEST_ADMIN_002"
	}
	all := []sampleFrame{
		{
			name:    "register",
			payload: []byte(gatewayName + "\n"),
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

func runLoop(conn net.Conn, gatewayName string, duration time.Duration, heartbeatInterval time.Duration, workInterval time.Duration, alarmInterval time.Duration, unknownInterval time.Duration, modbusReadInterval time.Duration, waitDownlink bool, waitTimeout time.Duration) error {
	if heartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat-interval must be greater than 0 in loop mode")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	frames, err := selectFrames("register,heartbeat,work,alarm,unknown,modbus-read", gatewayName)
	if err != nil {
		return err
	}
	byName := make(map[string]sampleFrame, len(frames))
	for _, frame := range frames {
		byName[frame.name] = frame
	}

	if waitDownlink {
		go waitForDownlink(conn, waitTimeout)
	}

	if err := sendFrame(conn, byName["register"]); err != nil {
		return err
	}
	if err := sendFrame(conn, byName["heartbeat"]); err != nil {
		return err
	}

	type periodicFrame struct {
		frame  sampleFrame
		ticker *time.Ticker
	}
	periodic := []periodicFrame{
		{frame: byName["heartbeat"], ticker: time.NewTicker(heartbeatInterval)},
	}
	if workInterval > 0 {
		periodic = append(periodic, periodicFrame{frame: byName["work"], ticker: time.NewTicker(workInterval)})
	}
	if alarmInterval > 0 {
		periodic = append(periodic, periodicFrame{frame: byName["alarm"], ticker: time.NewTicker(alarmInterval)})
	}
	if unknownInterval > 0 {
		periodic = append(periodic, periodicFrame{frame: byName["unknown"], ticker: time.NewTicker(unknownInterval)})
	}
	if modbusReadInterval > 0 {
		periodic = append(periodic, periodicFrame{frame: byName["modbus-read"], ticker: time.NewTicker(modbusReadInterval)})
	}
	defer func() {
		for _, item := range periodic {
			item.ticker.Stop()
		}
	}()

	fmt.Printf("loop started gateway=%s heartbeat_interval=%s duration=%s\n", strings.TrimSpace(gatewayName), heartbeatInterval, duration)
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "loop stopped: %v\n", ctx.Err())
			return nil
		default:
		}
		for _, item := range periodic {
			select {
			case <-ctx.Done():
				fmt.Fprintf(os.Stderr, "loop stopped: %v\n", ctx.Err())
				return nil
			case <-item.ticker.C:
				if err := sendFrame(conn, item.frame); err != nil {
					return err
				}
			default:
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func sendFrame(conn net.Conn, frame sampleFrame) error {
	if _, err := conn.Write(frame.payload); err != nil {
		return err
	}
	fmt.Printf("sent frame=%s bytes=%d hex=%s note=%s\n",
		frame.name,
		len(frame.payload),
		strings.ToUpper(hex.EncodeToString(frame.payload)),
		frame.note,
	)
	return nil
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
