package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"unicode"
)

type MessageKind string

const (
	MessageRegister      MessageKind = "register"
	MessageHeartbeat     MessageKind = "heartbeat"
	MessageSyncAck       MessageKind = "sync_ack"
	MessageWorkData      MessageKind = "work_data"
	MessageAlarm         MessageKind = "alarm"
	MessageControlResult MessageKind = "control_result"
	MessageUnknown       MessageKind = "unknown"
)

type Message struct {
	Kind        MessageKind
	GatewayName string
	PayloadHex  string
	CRCValid    bool
	ASCII       string
}

func ClassifyPayload(payload []byte) Message {
	msg := Message{
		Kind:       MessageUnknown,
		PayloadHex: strings.ToUpper(hex.EncodeToString(payload)),
		CRCValid:   ValidateCRC16Modbus(payload),
	}

	trimmed := bytes.TrimSpace(payload)
	if isPrintableASCII(trimmed) {
		msg.ASCII = string(trimmed)
		upper := strings.ToUpper(msg.ASCII)
		if strings.HasPrefix(upper, "FP_") {
			gatewayName := NormalizeGatewayName(msg.ASCII)
			if !strings.HasPrefix(strings.ToUpper(gatewayName), "FP_") {
				return msg
			}
			msg.Kind = MessageRegister
			msg.GatewayName = gatewayName
			return msg
		}
		if strings.HasPrefix(upper, "NH_TELEMETRY|") {
			msg.Kind = MessageWorkData
			return msg
		}
		switch upper {
		case "HB", "PING", "HEART", "HEARTBEAT", "KEEPALIVE", "KEEPALIVE MESSAGE":
			msg.Kind = MessageHeartbeat
			return msg
		}
	}

	// MCU1 上行报文前两字节是采集执行器 ID，不是固定帧头。报文类型由
	// 第 3 字节 BB、总长度和 CRC 共同确定。
	if HasMCU1UplinkShape(payload) && msg.CRCValid {
		msg.Kind = MessageWorkData
		return msg
	}

	if len(payload) >= 7 && payload[0] == 0xAA && payload[1] == 0xAA {
		if isAlarmFrame(payload) {
			msg.Kind = MessageAlarm
		} else if msg.CRCValid {
			msg.Kind = MessageWorkData
		}
	}

	return msg
}

// HasMCU1UplinkShape reports whether payload has a documented MCU1 uplink BB
// and total length. CRC validation is intentionally separate so callers can
// still produce useful diagnostics for a damaged packet.
func HasMCU1UplinkShape(payload []byte) bool {
	if len(payload) < 3 {
		return false
	}
	switch payload[2] {
	case 0x01:
		return len(payload) == 10 || len(payload) == 45 || len(payload) == 48 || len(payload) == 51 || len(payload) == 54
	case 0x02:
		return len(payload) == 10 || len(payload) == 11
	case 0x03:
		return len(payload) == 9
	default:
		return false
	}
}

func NormalizeGatewayName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	for _, line := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '\r' || r == '\n'
	}) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[0]
		}
		return line
	}
	return ""
}

func GatewayExternalID(gatewayName string) string {
	name := NormalizeGatewayName(gatewayName)
	if strings.HasPrefix(strings.ToUpper(name), "FP_") {
		return name[3:]
	}
	if strings.HasPrefix(strings.ToUpper(name), "FP") {
		return strings.TrimLeft(name[2:], "_- ")
	}
	return name
}

func isPrintableASCII(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	for _, b := range data {
		r := rune(b)
		if r > unicode.MaxASCII || (!unicode.IsPrint(r) && !unicode.IsSpace(r)) {
			return false
		}
	}
	return true
}

func isAlarmFrame(payload []byte) bool {
	if len(payload) < 7 {
		return false
	}
	body := payload
	if len(payload) >= 9 && ValidateCRC16Modbus(payload) {
		body = payload[:len(payload)-2]
	}
	for i := 3; i+2 < len(body); i++ {
		if body[i] == 0xFF && (body[i+1] == 0xFF || body[i+2] == 0xFF) {
			return true
		}
	}
	return false
}
