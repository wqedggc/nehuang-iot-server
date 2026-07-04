package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"nehuang-iot-server/internal/gatewayadapter/journal"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

func main() {
	rawPath := flag.String("raw-log", "", "path to raw frame JSONL file")
	flag.Parse()
	if *rawPath == "" {
		fmt.Fprintln(os.Stderr, "-raw-log is required")
		os.Exit(2)
	}

	f, err := os.Open(*rawPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open raw log: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	counts := map[protocol.MessageKind]int{}
	total := 0
	crcValid := 0
	crcInvalid := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var frame journal.Frame
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			fmt.Fprintf(os.Stderr, "skip invalid jsonl: %v\n", err)
			continue
		}
		payload, err := hex.DecodeString(strings.TrimSpace(frame.PayloadHex))
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip invalid hex raw_seq=%d: %v\n", frame.RawSeq, err)
			continue
		}
		msg := protocol.ClassifyPayload(payload)
		total++
		counts[msg.Kind]++
		if msg.CRCValid {
			crcValid++
		} else {
			crcInvalid++
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scan raw log: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("total=%d crc_valid=%d crc_invalid=%d\n", total, crcValid, crcInvalid)
	for _, kind := range []protocol.MessageKind{
		protocol.MessageRegister,
		protocol.MessageHeartbeat,
		protocol.MessageSyncAck,
		protocol.MessageWorkData,
		protocol.MessageAlarm,
		protocol.MessageControlResult,
		protocol.MessageUnknown,
	} {
		fmt.Printf("%s=%d\n", kind, counts[kind])
	}
}
