package protocolsummary

import (
	"encoding/hex"
	"fmt"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
	"strings"
)

func BuildCurrentUplink(text string) *domain.ProtocolSummary {
	raw, e := hex.DecodeString(strings.TrimSpace(text))
	if e == nil && len(raw) > 0 && raw[0] == 0x7e {
		return buildMCU0WireCapture(raw, false)
	}
	return buildMCU0UnframedUplink(text)
}
func BuildCurrentDownlink(text string) *domain.ProtocolSummary {
	raw, e := hex.DecodeString(strings.TrimSpace(text))
	if e == nil && len(raw) > 0 && raw[0] == 0x7e {
		return buildMCU0WireCapture(raw, true)
	}
	return buildMCU0UnframedDownlink(text)
}
func buildMCU0WireCapture(raw []byte, down bool) *domain.ProtocolSummary {
	s := &domain.ProtocolSummary{Kind: "mcu0_wire_capture", Label: "MCU0 7E/LEN报文（50帧实证）", Status: "partial", ByteLength: len(raw), Description: "LEN包括正文CRC、不含7E/LEN；CRC不覆盖前缀。仅展示，不自动入库、下发确认或关联命令执行。"}
	var d protocol.MCU0StreamDecoder
	frames, e := d.Feed(raw)
	offset := 0
	valid := true
	for i, f := range frames {
		var inner *domain.ProtocolSummary
		body := f.Body
		if down && len(body) == 5 {
			inner = &domain.ProtocolSummary{Kind: "mcu0_clock", Label: "云端5字节校时候选", Status: "partial", Fields: []domain.ProtocolSummaryField{byteField("clock", "二进制时钟", body, 0, 2, fmt.Sprintf("%02d:%02d:%02d", body[0], body[1], body[2]), "decoded", "无ID/BB；样本证实存在5字节和8字节两种校时。日期与允许使用形式需配置确认。")}}
			appendMCU1CRC(body, inner)
		} else if !down && len(body) == 9 && body[2] == 5 {
			inner = &domain.ProtocolSummary{Kind: "mcu0_state_observation", Label: "BB05温控设置后状态观测", Status: "partial", Description: "无commandId，不作为某条命令执行成功确认。"}
			_, err := protocol.DecodeMCU0StateObservation(body)
			inner.Fields = []domain.ProtocolSummaryField{byteField("device_id", "ID原字节", body, 0, 1, rawHex(body[:2]), "unknown", "样本ID1为0001，更多设备仍需可靠集群映射"), byteField("clock", "二进制时钟", body, 3, 5, fmt.Sprintf("%02d:%02d:%02d", body[3], body[4], body[5]), "decoded", "北京时区；日期未给出，不使用接收日期猜测")}
			appendMCU1TagAndCRC(body, 6, inner)
			if err != nil {
				inner.Diagnostics = append(inner.Diagnostics, domain.ProtocolDiagnostic{Code: "mcu0_state_invalid", Severity: "error", Message: err.Error()})
			} else {
				inner.Fields = append(inner.Fields, derivedField("observation", "状态性质", "独立状态观测", "TCP发送与BB05之间没有commandId因果凭证"))
			}
		} else if down {
			inner = buildMCU0SampledDownlink(body)
		} else {
			inner = buildMCU0UnframedUplink(hex.EncodeToString(body))
		}
		prefix := fmt.Sprintf("frame_%02d_", i+1)
		s.Fields = append(s.Fields, byteField(prefix+"header", "包头", raw, offset, offset, "7E", "decoded", ""), byteField(prefix+"length", "LEN", raw, offset+1, offset+1, fmt.Sprintf("%d字节正文，含CRC", len(body)), "decoded", "单字节长度；不按7E分割正文"))
		if inner != nil {
			s.Fields = append(s.Fields, derivedField(prefix+"kind", "帧类型", inner.Label, inner.Description))
			for _, field := range inner.Fields {
				field.Name = prefix + field.Name
				if field.RawHex != "" {
					field.ByteStart += offset + 2
					field.ByteEnd += offset + 2
				}
				s.Fields = append(s.Fields, field)
			}
			s.Diagnostics = append(s.Diagnostics, inner.Diagnostics...)
		}
		if !f.CRCValid {
			valid = false
			s.Diagnostics = append(s.Diagnostics, domain.ProtocolDiagnostic{Code: "mcu0_wire_crc_invalid", Severity: "error", Message: fmt.Sprintf("第%d帧CRC无效；保留原始字节，不作业务投影", i+1)})
		}
		offset += len(f.Wire)
	}
	s.RecordCount = len(frames)
	if len(frames) > 0 {
		s.CRCValid = &valid
	}
	if e != nil || len(d.Pending()) > 0 {
		message := fmt.Sprintf("本TCP采集含%d字节未完成/边界未验证数据；不能把一次TCP read当一帧", len(d.Pending()))
		if e != nil {
			message += "；" + e.Error()
		}
		s.Diagnostics = append(s.Diagnostics, domain.ProtocolDiagnostic{Code: "mcu0_wire_incomplete", Severity: "warning", Message: message})
	}
	return s
}

// Direction-specific lengths are proven by the real capture, not inferred from BB alone.
func buildMCU0SampledDownlink(body []byte) *domain.ProtocolSummary {
	if len(body) < 8 {
		return buildMCU0UnframedDownlink(hex.EncodeToString(body))
	}
	label := ""
	switch {
	case len(body) == 8 && body[2] == 0:
		label = "带ID的校时（样本早期形式）"
	case len(body) == 8 && body[2] == 2:
		label = "历史收包确认BB02（当前发送禁用）"
	case len(body) == 9 && body[2] == 3:
		label = "进入手动模式的操作指令"
	case len(body) == 12 && body[2] == 1:
		label = "自动温控阈值指令"
	default:
		return buildMCU0UnframedDownlink(hex.EncodeToString(body))
	}
	s := &domain.ProtocolSummary{Kind: "mcu0_downlink", Label: label, Status: "partial", ByteLength: len(body), Description: "10/1真实下行样本；发送仅代表字节输出，不等于硬件执行。"}
	s.Fields = []domain.ProtocolSummaryField{byteField("device_id", "设备ID原字节", body, 0, 1, rawHex(body[:2]), "unknown", "样本标注ID1为0001；仍需可靠集群映射"), byteField("function_code", "下行BB", body, 2, 2, fmt.Sprintf("%02X", body[2]), "decoded", "与上行同BB可能有不同语义"), byteField("report_time", "二进制时钟", body, 3, 5, fmt.Sprintf("%02d:%02d:%02d", body[3], body[4], body[5]), "decoded", "普通binary，北京时间；日期未给出")}
	if body[3] > 23 || body[4] > 59 || body[5] > 59 {
		s.Fields[2].Status = "invalid"
	}
	if body[2] == 3 {
		s.Fields = append(s.Fields, byteField("requested_tag", "操作Tag原字节", body, 6, 6, rawHex(body[6:7]), "unknown", "下行操作位不是NTC实测状态，不按上行故障字典归因；无即时ACK"))
	}
	if body[2] == 1 {
		s.Fields = append(s.Fields, byteField("threshold_1", "第一组阈值原字节", body, 6, 7, rawHex(body[6:8]), "unknown", "2字节；上下阈值顺序/单位待确认，不能直接生成配置"), byteField("threshold_2", "第二组阈值原字节", body, 8, 9, rawHex(body[8:10]), "unknown", "2字节；不沿用旧MCU1定义猜测"))
	}
	appendMCU1CRC(body, s)
	return s
}
