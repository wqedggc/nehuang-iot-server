package protocolsummary

import (
	"encoding/hex"
	"fmt"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
	"strings"
)

// BuildCurrentUplink adds display-only MCU0 candidates based on the 9/30 reply.
// Matching a length is not proof of a TCP frame or permission to persist values.
func BuildCurrentUplink(payloadHex string) *domain.ProtocolSummary {
	raw, err := hex.DecodeString(strings.TrimSpace(payloadHex))
	if err != nil || len(raw) < 3 {
		return Build(payloadHex)
	}
	if !((raw[2] == 1 && (len(raw) == 10 || (len(raw) >= 12 && len(raw) <= 54 && (len(raw)-9)%3 == 0))) || (raw[2] == 2 && len(raw) == 11)) {
		return Build(payloadHex)
	}
	s := &domain.ProtocolSummary{Kind: "mcu0_temperature_candidate", Label: "MCU0温度候选报文（9/30确认）", Status: "partial", ByteLength: len(raw), Description: "仅按已确认温度编码展示；完整帧、设备身份、时钟/日期映射仍需验证，不据此写库或确认命令完成。"}
	s.Fields = []domain.ProtocolSummaryField{byteField("device_id", "设备ID原字节", raw, 0, 1, rawHex(raw[:2]), "unknown", "端序与设备映射待确认"), byteField("report_time", "时钟原字节", raw, 3, 5, rawHex(raw[3:6]), "unknown", "时间编码、时区、跨天归属待确认")}
	appendMCU1TagAndCRC(raw, len(raw)-3, s)
	f, err := protocol.DecodeMCU0TemperatureFrame(raw, true)
	if err != nil {
		s.Diagnostics = append(s.Diagnostics, domain.ProtocolDiagnostic{Code: "mcu0_candidate_invalid", Severity: "error", Message: err.Error()})
		return s
	}
	mode := fmt.Sprintf("自动历史：5分钟记录，本包%d组（线序保留，时间映射待确认）", len(f.Samples))
	if f.Mode == "manual" {
		mode = "手动实时：4分钟回显，无即时控制ACK"
	}
	s.Fields = append(s.Fields, derivedField("temperature_mode", "温度模式", mode, "上下行BB必须区分；状态观测不等于commandId执行确认"))
	for i, sample := range f.Samples {
		offset := 6
		name := "current_temperature"
		if f.Mode == "automatic" {
			offset = 7 + i*3
			name = fmt.Sprintf("history_record_%02d", i+1)
		}
		status := "decoded"
		note := "有符号int16，大端，实际值×10；确认来源见docs/protocols/mcu0-0910/confirmation-20260930.md"
		if f.Tag.NTCFault {
			status = "invalid"
			note += "；NTC故障，仅显示原始数值，不作有效测量"
		}
		s.Fields = append(s.Fields, byteField(name, "温度", raw, offset, offset+1, fmt.Sprintf("%.1f ℃", sample.Celsius), status, note))
	}
	return s
}
func BuildCurrentDownlink(payloadHex string) *domain.ProtocolSummary {
	raw, err := hex.DecodeString(strings.TrimSpace(payloadHex))
	if err == nil && len(raw) == 8 && raw[2] == 2 {
		s := &domain.ProtocolSummary{Kind: "mcu0_history_ack", Label: "MCU0历史收包确认候选", ByteLength: 8, Status: "partial", Description: "9/30确认：设备ID2+BB02+时钟3+CRC2。清缓存/重发及时间关联仍待确认；不是控制执行ACK。"}
		s.Fields = []domain.ProtocolSummaryField{byteField("device_id", "设备ID", raw, 0, 1, rawHex(raw[:2]), "unknown", "地址映射待确认"), byteField("report_time", "批次时钟", raw, 3, 5, rawHex(raw[3:6]), "unknown", "时钟编码与批次关联待确认")}
		appendMCU1CRC(raw, s)
		return s
	}
	s := BuildDownlink(payloadHex)
	if s != nil && err == nil && len(raw) == 9 && raw[2] == 3 {
		s.Description += "；9/30确认BB03进入手动模式，开关无即时返回，等4分钟回显；不能据周期状态判定commandId完成。"
	}
	return s
}
