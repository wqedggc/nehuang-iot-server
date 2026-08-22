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
	collectorIDHex := flag.String("collector-id-hex", "AAAA", "MCU1 collector ID as exactly two hex bytes, for example 03E9")
	frameName := flag.String("frame", "all", "frame to send: all, register, heartbeat, heartbeat-message, telemetry, telemetry-temp, telemetry-soil, telemetry-relay, modbus-read, mcu1, mcu1-auto, mcu1-auto-history, mcu1-manual, mcu1-fault, binary-work, alarm, unknown")
	repeat := flag.Int("repeat", 1, "repeat count")
	delay := flag.Duration("delay", 300*time.Millisecond, "delay between frames")
	loop := flag.Bool("loop", false, "keep the TCP connection open and send periodic mock frames")
	duration := flag.Duration("duration", 0, "loop duration; 0 means run until interrupted")
	heartbeatInterval := flag.Duration("heartbeat-interval", 10*time.Second, "heartbeat interval in loop mode")
	workInterval := flag.Duration("work-interval", 30*time.Second, "mock telemetry interval in loop mode; 0 disables periodic telemetry frames")
	alarmInterval := flag.Duration("alarm-interval", 0, "alarm frame interval in loop mode; 0 disables periodic alarm frames")
	unknownInterval := flag.Duration("unknown-interval", 0, "unknown frame interval in loop mode; 0 disables periodic unknown frames")
	modbusReadInterval := flag.Duration("modbus-read-interval", 0, "modbus-read frame interval in loop mode; 0 disables periodic modbus-read frames")
	waitDownlink := flag.Bool("wait-downlink", false, "send bootstrap frames, then wait and print downlink bytes")
	waitTimeout := flag.Duration("wait-timeout", 0, "read timeout while waiting for downlink; 0 means wait until connection closes")
	flag.Parse()

	selectedFrameName := *frameName
	if *waitDownlink && strings.EqualFold(strings.TrimSpace(selectedFrameName), "all") {
		selectedFrameName = "register,heartbeat"
	} else if *loop && strings.EqualFold(strings.TrimSpace(selectedFrameName), "all") {
		selectedFrameName = "register,heartbeat,telemetry"
	}

	collectorID, err := parseCollectorIDHex(*collectorIDHex)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	frames, err := selectFrames(selectedFrameName, *gatewayName, collectorID)
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
		if err := runLoop(conn, *gatewayName, frames, *duration, *heartbeatInterval, *workInterval, *alarmInterval, *unknownInterval, *modbusReadInterval, *waitDownlink, *waitTimeout, *delay); err != nil {
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

func parseCollectorIDHex(raw string) ([]byte, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(strings.TrimPrefix(cleaned, "0x"), "0X")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	decoded, err := hex.DecodeString(cleaned)
	if err != nil || len(decoded) != 2 {
		return nil, fmt.Errorf("collector-id-hex must be exactly two hex bytes, got %q", raw)
	}
	return decoded, nil
}

func selectFrames(name string, gatewayName string, collectorID []byte) ([]sampleFrame, error) {
	gatewayName = strings.TrimSpace(gatewayName)
	if gatewayName == "" {
		gatewayName = "FP_TEST_ADMIN_002"
	}
	if len(collectorID) != 2 {
		return nil, fmt.Errorf("collector ID must contain exactly two bytes")
	}
	mcu1Body := func(rest ...byte) []byte {
		body := append([]byte(nil), collectorID...)
		return append(body, rest...)
	}
	historyBody := mcu1Body(0x01, 0x01, 0x02, 0x03)
	for sequence := byte(1); sequence <= 12; sequence++ {
		historyBody = append(historyBody, sequence, 0x00, 0x14+sequence)
	}
	historyBody = append(historyBody, 0xC0)
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
			name:    "telemetry-temp",
			payload: []byte("NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=1|channel_no=1|value=26.5|unit=C"),
			note:    "mock telemetry temperature frame; Server projects it to collector/device point data",
		},
		{
			name:    "telemetry-soil",
			payload: []byte("NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=2|channel_no=1|value=42.0|unit=%"),
			note:    "mock telemetry soil moisture frame; Server projects it to collector/device point data",
		},
		{
			name:    "telemetry-relay",
			payload: []byte("NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=4|channel_no=1|value=1|unit=state"),
			note:    "mock telemetry relay state frame; Server projects it to collector/device point data",
		},
		{
			name:    "mcu1-auto",
			payload: protocol.AppendCRC16Modbus(mcu1Body(0x01, 0x01, 0x02, 0x03, 0x00, 0xC0)),
			note:    "MCU1 BB=01 automatic status without history; raw time=01 02 03, Tag=C0 normal/stopped",
		},
		{
			name:    "mcu1-auto-history",
			payload: protocol.AppendCRC16Modbus(historyBody),
			note:    "MCU1 BB=01 automatic status with 12 raw history-temperature records; conversion remains pending protocol confirmation",
		},
		{
			name:    "mcu1-manual",
			payload: protocol.AppendCRC16Modbus(mcu1Body(0x02, 0x01, 0x02, 0x03, 0x19, 0xC0)),
			note:    "MCU1 BB=02 manual status; raw time=01 02 03, temperature=19, Tag=C0 normal/stopped",
		},
		{
			name:    "mcu1-fault",
			payload: protocol.AppendCRC16Modbus(mcu1Body(0x03, 0x01, 0x02, 0x03, 0xE0)),
			note:    "MCU1 BB=03 fault status; raw time=01 02 03, Tag=E0 rail fault",
		},
		{
			name:    "binary-work",
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
			note:    "unknown bytes; Adapter keeps raw journal and forwards only when configured",
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
		if part == "telemetry" || part == "work" {
			for _, telemetryName := range []string{"telemetry-temp", "telemetry-soil", "telemetry-relay"} {
				selected = append(selected, byName[telemetryName])
			}
			continue
		}
		if part == "mcu1" {
			for _, mcu1Name := range []string{"mcu1-auto", "mcu1-auto-history", "mcu1-manual", "mcu1-fault"} {
				selected = append(selected, byName[mcu1Name])
			}
			continue
		}
		frame, ok := byName[part]
		if !ok {
			return nil, fmt.Errorf("unknown frame %q; use all, register, heartbeat, heartbeat-message, telemetry, telemetry-temp, telemetry-soil, telemetry-relay, modbus-read, mcu1, mcu1-auto, mcu1-auto-history, mcu1-manual, mcu1-fault, binary-work, alarm, or unknown", part)
		}
		selected = append(selected, frame)
	}
	return selected, nil
}

func runLoop(conn net.Conn, gatewayName string, frames []sampleFrame, duration time.Duration, heartbeatInterval time.Duration, workInterval time.Duration, alarmInterval time.Duration, unknownInterval time.Duration, modbusReadInterval time.Duration, waitDownlink bool, waitTimeout time.Duration, frameDelay time.Duration) error {
	if len(frames) == 0 {
		return fmt.Errorf("at least one frame must be selected in loop mode")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	if waitDownlink {
		go waitForDownlink(conn, waitTimeout)
	}

	for index, frame := range frames {
		if err := sendFrame(conn, frame); err != nil {
			return err
		}
		if frameDelay > 0 && index < len(frames)-1 {
			time.Sleep(frameDelay)
		}
	}

	type periodicFrame struct {
		frame  sampleFrame
		ticker *time.Ticker
	}
	periodic := make([]periodicFrame, 0, len(frames))
	for _, frame := range frames {
		var interval time.Duration
		switch {
		case frame.name == "heartbeat" || frame.name == "heartbeat-message":
			interval = heartbeatInterval
		case strings.HasPrefix(frame.name, "telemetry-") || strings.HasPrefix(frame.name, "mcu1-") || frame.name == "binary-work":
			interval = workInterval
		case frame.name == "alarm":
			interval = alarmInterval
		case frame.name == "unknown":
			interval = unknownInterval
		case frame.name == "modbus-read":
			interval = modbusReadInterval
		}
		if interval > 0 {
			periodic = append(periodic, periodicFrame{frame: frame, ticker: time.NewTicker(interval)})
		}
	}
	defer func() {
		for _, item := range periodic {
			item.ticker.Stop()
		}
	}()

	selectedNames := make([]string, 0, len(frames))
	for _, frame := range frames {
		selectedNames = append(selectedNames, frame.name)
	}
	fmt.Printf("loop started gateway=%s frames=%s heartbeat_interval=%s work_interval=%s duration=%s\n", strings.TrimSpace(gatewayName), strings.Join(selectedNames, ","), heartbeatInterval, workInterval, duration)
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
				if frameDelay > 0 {
					time.Sleep(frameDelay)
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
