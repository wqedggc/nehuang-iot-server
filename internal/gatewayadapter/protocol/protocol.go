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
			msg.Kind = MessageRegister
			msg.GatewayName = msg.ASCII
			return msg
		}
		switch upper {
		case "HB", "PING", "HEART", "HEARTBEAT", "KEEPALIVE", "KEEPALIVE MESSAGE":
			msg.Kind = MessageHeartbeat
			return msg
		}
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

func GatewayExternalID(gatewayName string) string {
	name := strings.TrimSpace(gatewayName)
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
