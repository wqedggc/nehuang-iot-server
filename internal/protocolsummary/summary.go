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
		Status:      "unknown",
		ByteLength:  len(payload),
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
			summary.Status = "decoded"
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
			summary.Status = "decoded"
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
		summary.Status = "decoded"
		summary.Fields = []domain.ProtocolSummaryField{
			field("slave_address", "从站地址", fmt.Sprintf("%d", payload[0])),
			field("function_code", "功能码", fmt.Sprintf("0x%02X", payload[1])),
			field("start_register", "起始寄存器", fmt.Sprintf("%d", binary.BigEndian.Uint16(payload[2:4]))),
			field("register_count", "寄存器数量", fmt.Sprintf("%d", binary.BigEndian.Uint16(payload[4:6]))),
			field("crc_expected", "计算 CRC", fmt.Sprintf("%04X", bodyCRC)),
		}
		return summary
	}

	if looksLikeMCU1Uplink(payload) {
		return buildMCU1Uplink(payload, summary)
	}

	msg := protocol.ClassifyPayload(payload)
	switch msg.Kind {
	case protocol.MessageAlarm:
		summary.Kind = "alarm"
		summary.Label = "告警候选包"
		summary.Description = "匹配到告警候选形态；字段语义未冻结，暂不写入告警业务表。"
		summary.Status = "partial"
	case protocol.MessageWorkData:
		summary.Kind = "work_data"
		summary.Label = "工作数据候选包"
		summary.Description = "匹配到工作数据候选形态；字段语义未冻结，暂不写入传感器业务表。"
		summary.Status = "partial"
	}
	return summary
}

func field(name, label, value string) domain.ProtocolSummaryField {
	return domain.ProtocolSummaryField{Name: name, Label: label, Value: value}
}

func buildMCU1Uplink(payload []byte, summary *domain.ProtocolSummary) *domain.ProtocolSummary {
	bb := payload[2]
	messageType, err := protocol.DecodeMCU1MessageType(protocol.MCU1DirectionUplink, bb)
	if err != nil {
		return summary
	}

	summary.Kind, summary.Label = mcu1UplinkName(messageType)
	summary.Description = "按照 MCU1 说明书第 14–15 页解释一条完整上行报文；该结果只用于联调展示，不代表已写入业务数据。"
	summary.Status = "partial"
	summary.Fields = []domain.ProtocolSummaryField{
		byteField("device_id", "采集执行器 ID", payload, 0, 1, "0x"+strings.ReplaceAll(rawHex(payload[0:2]), " ", ""), "decoded", "原始 2 字节标识；数值字节序待双方最终确认。"),
		byteField("function_code", "功能字节 BB", payload, 2, 2, mcu1FunctionLabel(bb), "decoded", "上行方向与 BB 共同确定报文类型。"),
		byteField("report_time", "报文时间 HH MM SS", payload, 3, 5, rawHex(payload[3:6])+"（原始字节）", "unknown", "时间是 3 字节，但 BCD/十进制编码和时区尚未确认。"),
	}
	summary.Diagnostics = append(summary.Diagnostics, domain.ProtocolDiagnostic{
		Code:     "time_encoding_unknown",
		Severity: "warning",
		Message:  "报文时间字段已定位，但编码方式和时区尚未确认。",
	})

	switch messageType {
	case protocol.MCU1MessageUplinkAutomaticStatusAndHistory:
		decodeMCU1Automatic(payload, summary)
	case protocol.MCU1MessageUplinkManualStatus:
		decodeMCU1Manual(payload, summary)
	case protocol.MCU1MessageUplinkFaultStatus:
		decodeMCU1Fault(payload, summary)
	}
	return summary
}

func decodeMCU1Automatic(payload []byte, summary *domain.ProtocolSummary) {
	switch {
	case len(payload) == 10 && payload[6] == 0x00:
		summary.Fields = append(summary.Fields,
			byteField("history_marker", "历史记录标记", payload, 6, 6, "无历史温度记录", "decoded", "说明书定义初次上电无历史记录时该字节为 00。"),
		)
		appendMCU1TagAndCRC(payload, 7, summary)
	case len(payload) >= 45 && len(payload) <= 54 && (len(payload)-9)%3 == 0:
		recordCount := (len(payload) - 9) / 3
		if recordCount < 12 || recordCount > 15 {
			setInvalidLength(summary, len(payload), "BB=01 的历史记录应为 12–15 组，每组 3 字节。")
			return
		}
		summary.Status = "partial"
		summary.RecordCount = recordCount
		for i := 0; i < recordCount; i++ {
			start := 6 + i*3
			sequence := payload[start]
			temperatureHex := fmt.Sprintf("%02X%02X", payload[start+1], payload[start+2])
			summary.Fields = append(summary.Fields, byteField(
				fmt.Sprintf("history_record_%02d", i+1),
				fmt.Sprintf("历史温度记录 %02d", i+1),
				payload,
				start,
				start+2,
				fmt.Sprintf("序号 0x%02X，温度 0x%s", sequence, temperatureHex),
				"unknown",
				"记录边界已确认；温度的字节序、符号位和缩放规则待确认。",
			))
		}
		summary.Diagnostics = append(summary.Diagnostics, domain.ProtocolDiagnostic{
			Code:     "temperature_encoding_unknown",
			Severity: "warning",
			Message:  "历史温度记录已切分，但温度换算规则尚未确认。",
		})
		appendMCU1TagAndCRC(payload, len(payload)-3, summary)
	default:
		setInvalidLength(summary, len(payload), "BB=01 应为 10 字节无历史包，或 45/48/51/54 字节历史包。")
	}
}

func decodeMCU1Manual(payload []byte, summary *domain.ProtocolSummary) {
	if len(payload) != 10 && len(payload) != 11 {
		setInvalidLength(summary, len(payload), "BB=02 手动状态上报按当前文档应为 10 或 11 字节；当前温度宽度需用样例最终确认。")
		return
	}
	summary.Status = "partial"
	tagOffset := len(payload) - 3
	summary.Fields = append(summary.Fields, byteField(
		"current_temperature",
		"当前温度 XX",
		payload,
		6,
		tagOffset-1,
		"换算规则待确认",
		"unknown",
		fmt.Sprintf("已按 Tag 前的 %d 字节切分；准确宽度、字节序、符号位和缩放规则待确认。", tagOffset-6),
	))
	summary.Diagnostics = append(summary.Diagnostics, domain.ProtocolDiagnostic{
		Code:     "temperature_encoding_unknown",
		Severity: "warning",
		Message:  "当前温度字段已定位，但温度换算规则尚未确认。",
	})
	appendMCU1TagAndCRC(payload, tagOffset, summary)
}

func decodeMCU1Fault(payload []byte, summary *domain.ProtocolSummary) {
	if len(payload) != 9 {
		setInvalidLength(summary, len(payload), "BB=03 故障状态上报应为 9 字节。")
		return
	}
	appendMCU1TagAndCRC(payload, 6, summary)
}

func appendMCU1TagAndCRC(payload []byte, tagOffset int, summary *domain.ProtocolSummary) {
	if tagOffset < 0 || tagOffset+2 >= len(payload) {
		setInvalidLength(summary, len(payload), "报文中没有足够字节容纳 Tag 和 2 字节 CRC。")
		return
	}

	tagRaw := payload[tagOffset]
	tagField := byteField("tag", "I/O 状态 Tag", payload, tagOffset, tagOffset, formatBinary8(tagRaw), "decoded", "位序按 1/N/R/S/V/W/T/U 从高到低解释。")
	tagField.Bits = buildMCU1TagBits(tagRaw)
	if _, err := protocol.DecodeMCU1Tag(tagRaw); err != nil {
		tagField.Status = "invalid"
		tagField.Note = err.Error()
		summary.Status = "invalid"
		summary.Diagnostics = append(summary.Diagnostics, domain.ProtocolDiagnostic{
			Code:     "invalid_tag",
			Severity: "error",
			Message:  err.Error(),
		})
	}
	summary.Fields = append(summary.Fields, tagField)

	crcOffset := len(payload) - 2
	computed := protocol.CRC16Modbus(payload[:crcOffset])
	received := uint16(payload[crcOffset]) | uint16(payload[crcOffset+1])<<8
	crcStatus := "decoded"
	crcValue := fmt.Sprintf("通过；收到 0x%04X，计算 0x%04X", received, computed)
	if summary.CRCValid == nil || !*summary.CRCValid {
		crcStatus = "invalid"
		crcValue = fmt.Sprintf("不通过；收到 0x%04X，计算 0x%04X", received, computed)
		summary.Status = "invalid"
		summary.Diagnostics = append(summary.Diagnostics, domain.ProtocolDiagnostic{
			Code:     "crc_mismatch",
			Severity: "error",
			Message:  "CRC-16/MODBUS 校验不通过。",
		})
	}
	summary.Fields = append(summary.Fields, byteField(
		"crc",
		"CRC-16/MODBUS",
		payload,
		crcOffset,
		crcOffset+1,
		crcValue,
		crcStatus,
		fmt.Sprintf("校验覆盖字节 0..%d；报文内 CRC 为低字节在前。", crcOffset-1),
	))
}

func buildMCU1TagBits(raw byte) []domain.ProtocolSummaryBit {
	type bitDefinition struct {
		position int
		bitIndex int
		symbol   string
		zero     string
		one      string
	}
	definitions := []bitDefinition{
		{position: 8, bitIndex: 7, symbol: "1", zero: "Tag 标识无效", one: "Tag 标识有效"},
		{position: 7, bitIndex: 6, symbol: "N", zero: "NTC 掉线", one: "NTC 在线"},
		{position: 6, bitIndex: 5, symbol: "R", zero: "轨道正常", one: "卷帘机轨道故障"},
		{position: 5, bitIndex: 4, symbol: "S", zero: "过卷/过放正常", one: "卷帘机过卷或过放故障"},
		{position: 4, bitIndex: 3, symbol: "V", zero: "未触发上限位", one: "已触发上限位"},
		{position: 3, bitIndex: 2, symbol: "W", zero: "未触发下限位", one: "已触发下限位"},
		{position: 2, bitIndex: 1, symbol: "T", zero: "开启继电器已停止", one: "开启继电器已启动"},
		{position: 1, bitIndex: 0, symbol: "U", zero: "关闭继电器已停止", one: "关闭继电器已启动"},
	}

	bits := make([]domain.ProtocolSummaryBit, 0, len(definitions))
	for _, definition := range definitions {
		value := uint8((raw >> definition.bitIndex) & 0x01)
		meaning := definition.zero
		if value == 1 {
			meaning = definition.one
		}
		status := "confirmed"
		if definition.symbol == "1" && value == 0 {
			status = "invalid"
		}
		bits = append(bits, domain.ProtocolSummaryBit{
			Position: definition.position,
			BitIndex: definition.bitIndex,
			Symbol:   definition.symbol,
			Value:    value,
			Meaning:  meaning,
			Status:   status,
		})
	}
	return bits
}

func setInvalidLength(summary *domain.ProtocolSummary, got int, expected string) {
	summary.Status = "invalid"
	summary.Diagnostics = append(summary.Diagnostics, domain.ProtocolDiagnostic{
		Code:     "invalid_length",
		Severity: "error",
		Message:  fmt.Sprintf("报文长度为 %d 字节；%s", got, expected),
	})
}

func byteField(name, label string, payload []byte, start, end int, value, status, note string) domain.ProtocolSummaryField {
	field := domain.ProtocolSummaryField{
		Name:      name,
		Label:     label,
		Value:     value,
		ByteStart: start,
		ByteEnd:   end,
		Status:    status,
		Note:      note,
	}
	if start >= 0 && end >= start && end < len(payload) {
		field.RawHex = rawHex(payload[start : end+1])
	}
	return field
}

func rawHex(data []byte) string {
	parts := make([]string, len(data))
	for i, b := range data {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, " ")
}

func formatBinary8(raw byte) string {
	return fmt.Sprintf("%08b", raw)
}

func looksLikeMCU1Uplink(payload []byte) bool {
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

func mcu1UplinkName(messageType protocol.MCU1MessageType) (string, string) {
	switch messageType {
	case protocol.MCU1MessageUplinkAutomaticStatusAndHistory:
		return "mcu1_uplink_automatic", "MCU1 自动状态与历史温度上报"
	case protocol.MCU1MessageUplinkManualStatus:
		return "mcu1_uplink_manual", "MCU1 手动状态上报"
	case protocol.MCU1MessageUplinkFaultStatus:
		return "mcu1_uplink_fault", "MCU1 故障状态上报"
	default:
		return "mcu1_uplink", "MCU1 上行报文"
	}
}

func mcu1FunctionLabel(bb byte) string {
	switch bb {
	case 0x01:
		return "自动状态与历史温度上报"
	case 0x02:
		return "手动状态上报"
	case 0x03:
		return "故障状态上报"
	default:
		return fmt.Sprintf("未知 BB 0x%02X", bb)
	}
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
