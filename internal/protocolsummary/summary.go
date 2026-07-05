package protocolsummary

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

// Build returns a display-only summary for raw TCP bytes.
func Build(payloadHex string) *domain.ProtocolSummary {
	payloadHex = strings.ToUpper(strings.TrimSpace(payloadHex))
	payload, err := hex.DecodeString(payloadHex)
	if err != nil || len(payload) == 0 {
		return nil
	}

	summary := &domain.ProtocolSummary{
		Kind:        "unknown",
		Label:       "未知包",
		Description: "暂未匹配到已确认协议，只保留原始报文用于联调排查。",
	}
	if len(payload) >= 3 {
		valid := protocol.ValidateCRC16Modbus(payload)
		summary.CRCValid = &valid
	}

	trimmed := bytes.TrimSpace(payload)
	if isPrintableASCII(trimmed) {
		ascii := string(trimmed)
		upper := strings.ToUpper(ascii)
		summary.ASCII = ascii
		switch {
		case strings.HasPrefix(upper, "FP_"):
			summary.Kind = "register"
			summary.Label = "注册"
			summary.Description = "设备网关注册包。"
			summary.CRCValid = nil
			summary.Fields = []domain.ProtocolSummaryField{
				field("gateway_name", "设备网关名称", ascii),
				field("gateway_external_id", "设备网关外部 ID", protocol.GatewayExternalID(ascii)),
			}
			return summary
		case isHeartbeatASCII(upper):
			summary.Kind = "heartbeat"
			summary.Label = "心跳"
			summary.Description = "设备网关心跳包，用于刷新设备网关在线状态。"
			summary.CRCValid = nil
			summary.Fields = []domain.ProtocolSummaryField{
				field("heartbeat_text", "心跳文本", ascii),
			}
			return summary
		default:
			summary.Kind = "ascii"
			summary.Label = "ASCII 原始包"
			summary.Description = "可打印 ASCII 报文，但未匹配到已确认业务类型。"
			return summary
		}
	}

	if isModbusReadHoldingRegisters(payload) {
		bodyCRC := protocol.CRC16Modbus(payload[:len(payload)-2])
		summary.Kind = "modbus_read_holding_registers"
		summary.Label = "Modbus 读寄存器指令"
		summary.Description = "这是读保持寄存器指令，只作为联调下行/回显排查摘要，不写入温度业务数据。"
		summary.Fields = []domain.ProtocolSummaryField{
			field("slave_address", "从站地址", fmt.Sprintf("%d", payload[0])),
			field("function_code", "功能码", fmt.Sprintf("0x%02X", payload[1])),
			field("start_register", "起始寄存器", fmt.Sprintf("%d", binary.BigEndian.Uint16(payload[2:4]))),
			field("register_count", "寄存器数量", fmt.Sprintf("%d", binary.BigEndian.Uint16(payload[4:6]))),
			field("crc_expected", "计算 CRC", fmt.Sprintf("%04X", bodyCRC)),
		}
		return summary
	}

	msg := protocol.ClassifyPayload(payload)
	switch msg.Kind {
	case protocol.MessageAlarm:
		summary.Kind = "alarm"
		summary.Label = "告警候选包"
		summary.Description = "匹配到告警候选形态；字段语义未冻结，暂不写入告警业务表。"
	case protocol.MessageWorkData:
		summary.Kind = "work_data"
		summary.Label = "工作数据候选包"
		summary.Description = "匹配到工作数据候选形态；字段语义未冻结，暂不写入传感器业务表。"
	}
	return summary
}

func field(name, label, value string) domain.ProtocolSummaryField {
	return domain.ProtocolSummaryField{Name: name, Label: label, Value: value}
}

func isHeartbeatASCII(upper string) bool {
	switch upper {
	case "HB", "PING", "HEART", "HEARTBEAT", "KEEPALIVE", "KEEPALIVE MESSAGE":
		return true
	default:
		return false
	}
}

func isModbusReadHoldingRegisters(payload []byte) bool {
	return len(payload) == 8 && payload[1] == 0x03 && protocol.ValidateCRC16Modbus(payload)
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
