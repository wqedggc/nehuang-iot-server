package http

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

// adapterHandler 处理 /api/v1/adapter/* 端点的设备事件上报。
type adapterHandler struct {
	repo         adapterRepository
	adapterToken string
}

// adapterRepository 定义 Adapter 所需的数据访问方法。
type adapterRepository interface {
	// InsertRawEvent 写入原始设备事件到 raw_device_events 表。
	// 返回插入后的 raw_event_id。
	InsertRawEvent(ctx context.Context, event *domain.RawDeviceEvent) (uint64, error)
	// UpsertGatewayByExternalID 按 gateway_external_id 创建或更新云边设备。
	UpsertGatewayByExternalID(ctx context.Context, gateway *domain.Gateway) (uint64, error)
	// UpsertCollectorNode 按 gateway_id + frequency + device_external_id 创建或更新采集执行器节点。
	// 返回 node_id。
	UpsertCollectorNode(ctx context.Context, node *domain.CollectorNode) (uint64, error)
	// UpsertDevicePoint 按 node_id + function_type + channel_no 创建或更新功能点。
	// 返回 point_id。
	UpsertDevicePoint(ctx context.Context, point *domain.DevicePoint) (uint64, error)
	// InsertSensorData 插入传感器时序数据。
	InsertSensorData(ctx context.Context, data *domain.SensorData) error
	// UpdateDevicePointRealtime 更新功能点实时状态（real_value, value_status, last_report_at_ms 等）。
	UpdateDevicePointRealtime(ctx context.Context, pointID uint64, realValue *float64, valueStatus domain.ValueStatus, lastReportAt interface{}) error
	// UpsertDeviceRealtimeMap 同步更新 device_realtime_maps 兼容投影表。
	UpsertDeviceRealtimeMap(ctx context.Context, point *domain.DevicePoint) error
	// UpdateRawEventParseStatus 更新原始事件的解析状态。
	UpdateRawEventParseStatus(ctx context.Context, rawEventID uint64, status domain.ParseStatus, errMsg *string) error
	// UpdateRawEventParseMeta 回填解析元数据（event_type / event_count / parsed_count / message_id）。
	UpdateRawEventParseMeta(ctx context.Context, rawEventID uint64, eventType domain.RawEventType, eventCount int, parsedCount int, messageID *string) error
	// FindGatewayByExternalID 按 gateway_external_id 查找云边设备，不存在返回 nil。
	FindGatewayByExternalID(ctx context.Context, externalID string) (*domain.Gateway, error)
	ListPendingControlCommands(ctx context.Context, gatewayExternalID string, limit int) ([]domain.ControlCommand, error)
	MarkControlCommandSent(ctx context.Context, commandID uint64, gatewayResponse *string) error
	ApplyControlCommandResult(ctx context.Context, input domain.AdapterCommandResultInput) error
}

// ─── Adapter 请求模型 ───

// deviceEventEnvelope 设备事件外层信封，对应实施计划中的统一事件结构。
type deviceEventEnvelope struct {
	ProtocolVersion   string        `json:"protocol_version"`
	SourceType        string        `json:"source_type"`
	MessageID         string        `json:"message_id"`
	GatewayName       string        `json:"gateway_name"`
	GatewayExternalID string        `json:"gateway_external_id"`
	RemoteIP          string        `json:"remote_ip"`
	RemotePort        int           `json:"remote_port"`
	ReportedAtMS      string        `json:"reported_at_ms"`
	Events            []deviceEvent `json:"events"`
}

// deviceEvent 设备事件内层，对应 events[] 中的单条事件。
type deviceEvent struct {
	EventType        string   `json:"event_type"` // register / heartbeat / telemetry / control_result
	Frequency        int      `json:"frequency"`
	DeviceExternalID string   `json:"device_external_id"`
	FunctionType     uint8    `json:"function_type"`
	ChannelNo        int      `json:"channel_no"`
	Value            *float64 `json:"value"`
	Unit             string   `json:"unit"`
}

type rawFrameEnvelope struct {
	ProtocolVersion string `json:"protocol_version"`
	SourceType      string `json:"source_type"`
	MessageID       string `json:"message_id"`
	ConnectionID    string `json:"connection_id"`
	GatewayName     string `json:"gateway_name"`
	RemoteIP        string `json:"remote_ip"`
	RemotePort      int    `json:"remote_port"`
	Direction       string `json:"direction"`
	PayloadHex      string `json:"payload_hex"`
	ReceivedAt      string `json:"received_at"`
	RawSeq          uint64 `json:"raw_seq"`
}

// handleRawFrames stores one raw TCP frame captured by gateway-adapter.
//
// This endpoint is intentionally raw-first: it validates the envelope, writes the
// original bytes to raw_device_events, and only performs minimal stable parsing
// for register/heartbeat. Work/alarm frames stay pending until the hardware
// protocol is frozen.
func (h *adapterHandler) handleRawFrames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	if !h.validAdapterToken(r) {
		writeCode(w, http.StatusUnauthorized, 401, "invalid or missing adapter token")
		return
	}

	bodyBytes, err := readAllBody(r)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "failed to read request body: "+err.Error())
		return
	}
	var envelope rawFrameEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body: "+err.Error())
		return
	}

	payloadHex := strings.ToUpper(strings.TrimSpace(envelope.PayloadHex))
	payload, err := hex.DecodeString(payloadHex)
	if err != nil || payloadHex == "" {
		writeCode(w, http.StatusBadRequest, 400, "invalid payload_hex")
		return
	}
	if envelope.GatewayName == "" {
		trimmed := bytes.TrimSpace(payload)
		if bytes.HasPrefix(trimmed, []byte("FP_")) {
			envelope.GatewayName = string(trimmed)
		}
	}

	var receivedAt time.Time
	if strings.TrimSpace(envelope.ReceivedAt) != "" {
		receivedAt, err = time.Parse(time.RFC3339Nano, envelope.ReceivedAt)
		if err != nil {
			writeCode(w, http.StatusBadRequest, 400, "invalid received_at")
			return
		}
	}

	payloadSnapshot := string(bodyBytes)
	rawEvent := &domain.RawDeviceEvent{
		GatewayName:     strPtr(envelope.GatewayName),
		EventType:       rawFrameEventType(payload),
		SourceType:      sourceTypeFromEnvelope(envelope.SourceType),
		RemoteIP:        strPtr(envelope.RemoteIP),
		RemotePort:      intPtr(envelope.RemotePort),
		PayloadHex:      strPtr(payloadHex),
		PayloadSnapshot: &payloadSnapshot,
		ParseStatus:     domain.ParseStatusPending,
		MessageID:       strPtr(envelope.MessageID),
		EventCount:      1,
		ParsedCount:     0,
		ReceivedAtMS:    receivedAt,
	}
	rawEventID, err := h.repo.InsertRawEvent(r.Context(), rawEvent)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to insert raw frame: "+err.Error())
		return
	}

	status := domain.ParseStatusPending
	if parsedStatus, err := h.parseRawFrame(r.Context(), rawEventID, envelope, payload); err != nil {
		status = domain.ParseStatusFailed
		_ = h.repo.UpdateRawEventParseStatus(r.Context(), rawEventID, status, strPtr(err.Error()))
	} else {
		status = parsedStatus
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"raw_event_id": rawEventID,
			"parse_status": status.ParseStatusName(),
		},
	})
}

// handleDeviceEvents 处理 POST /api/v1/adapter/device-events。
// 认证使用 X-Adapter-Token header，不走用户 JWT。
//
// 数据可靠性保证：
//  1. 先读 Body 全部字节 → 确保网络层数据完整接收
//  2. json.Unmarshal 解析 → 验证 JSON 格式
//  3. 完整 payload（含 protocol_version, source_type, message_id,
//     gateway_name, gateway_external_id, remote_ip, remote_port,
//     reported_at_ms, events[]）以 JSON 字符串存入 payload_snapshot
//  4. raw_device_events INSERT 必须在所有业务逻辑之前执行
//  5. INSERT 失败 → 直接返回 500，不执行任何业务逻辑
//  6. INSERT 成功 → 再按 events[] 逐条处理，失败不丢失原始数据
func (h *adapterHandler) handleDeviceEvents(w http.ResponseWriter, r *http.Request) {
	// 校验 Adapter Token。
	if !h.validAdapterToken(r) {
		writeCode(w, http.StatusUnauthorized, 401, "invalid or missing adapter token")
		return
	}

	// 1. 先读取原始 Body 字节，确保网络层数据完整接收。
	bodyBytes, err := readAllBody(r)
	if err != nil {
		writeCode(w, http.StatusBadRequest, 400, "failed to read request body: "+err.Error())
		return
	}

	// 2. 解析 JSON（验证格式）。
	var envelope deviceEventEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body: "+err.Error())
		return
	}

	// 3. 构建 payload_snapshot（完整原始数据，一字不丢）。
	//    使用原始 bodyBytes 而非 json.Marshal(envelope)，避免反序列化再序列化
	//    导致的字段顺序/精度/空白字符差异。
	payloadSnapshot := string(bodyBytes)
	sourceType := sourceTypeFromEnvelope(envelope.SourceType)

	// 4. 先写 raw_device_events（原始事件保留，不做推断）。
	//    这一步在所有业务逻辑之前，确保即使后续处理失败，原始数据也不丢失。
	rawEvent := &domain.RawDeviceEvent{
		GatewayName:     strPtr(envelope.GatewayName),
		EventType:       domain.RawEventWork, // 默认 work，后续按 event_type 细分
		SourceType:      sourceType,
		RemoteIP:        strPtr(envelope.RemoteIP),
		RemotePort:      intPtr(envelope.RemotePort),
		PayloadSnapshot: &payloadSnapshot,
		ParseStatus:     domain.ParseStatusPending,
	}

	rawEventID, err := h.repo.InsertRawEvent(r.Context(), rawEvent)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to insert raw event: "+err.Error())
		return
	}

	// 5. 按 gateway_external_id 查找或创建云边设备。
	gateway, err := h.repo.FindGatewayByExternalID(r.Context(), envelope.GatewayExternalID)
	if err != nil {
		_ = h.repo.UpdateRawEventParseStatus(r.Context(), rawEventID, domain.ParseStatusFailed, strPtr("find gateway error: "+err.Error()))
		writeCode(w, http.StatusInternalServerError, 500, "failed to find gateway")
		return
	}

	gatewayID := uint64(0)
	if gateway != nil {
		gatewayID = gateway.GatewayID
	}

	// 推断 event_type 用于回填。
	inferredEventType := domain.RawEventWork // 默认 work
	hasRegisterHeartbeat := false
	hasTelemetry := false
	for _, ev := range envelope.Events {
		switch ev.EventType {
		case "register", "heartbeat":
			hasRegisterHeartbeat = true
		case "telemetry":
			hasTelemetry = true
		}
	}
	if hasRegisterHeartbeat && !hasTelemetry {
		inferredEventType = domain.RawEventHeartbeat
	} else if hasTelemetry {
		inferredEventType = domain.RawEventWork
	}

	// 6. 逐条处理 events[]。
	parseFailed := false
	partialFailed := false
	var parseError string
	parsedCount := 0
	totalCount := len(envelope.Events)
	for _, ev := range envelope.Events {
		switch ev.EventType {
		case "register", "heartbeat":
			// register/heartbeat：刷新云边设备在线状态。
			gw := &domain.Gateway{
				GatewayExternalID: envelope.GatewayExternalID,
				GatewayName:       envelope.GatewayName,
				IP:                envelope.RemoteIP,
				Port:              envelope.RemotePort,
				OnlineStatus:      1, // 在线
				SourceType:        uint8(sourceType),
			}
			gid, err := h.repo.UpsertGatewayByExternalID(r.Context(), gw)
			if err != nil {
				parseFailed = true
				parseError = "upsert gateway failed: " + err.Error()
				break
			}
			gatewayID = gid
			parsedCount++

		case "telemetry":
			// telemetry：处理传感器数据上报。
			if ev.DeviceExternalID == "" || ev.FunctionType == 0 {
				parseFailed = true
				parseError = "telemetry missing device_external_id or function_type"
				break
			}
			if gatewayID == 0 {
				parseFailed = true
				parseError = "telemetry without known gateway"
				break
			}

			// 3a. 创建或更新采集执行器节点。
			node := &domain.CollectorNode{
				GatewayID:        gatewayID,
				Frequency:        ev.Frequency,
				DeviceExternalID: ev.DeviceExternalID,
				OnlineStatus:     1, // 有 telemetry 上报视为在线
				NodeStatus:       1, // active
			}
			nodeID, err := h.repo.UpsertCollectorNode(r.Context(), node)
			if err != nil {
				parseFailed = true
				parseError = "upsert collector node failed: " + err.Error()
				break
			}

			// 3b. 创建或更新功能点。
			point := &domain.DevicePoint{
				NodeID:           nodeID,
				GatewayID:        gatewayID,
				Frequency:        ev.Frequency,
				DeviceExternalID: ev.DeviceExternalID,
				FunctionType:     domain.FunctionType(ev.FunctionType),
				ChannelNo:        ev.ChannelNo,
				Unit:             ev.Unit,
				RealValue:        ev.Value,
				PointStatus:      1, // active
			}
			// 根据功能类型设置 value_status。
			if ev.Value != nil {
				point.ValueStatus = domain.ValueStatusNormal
				point.RealTemp = *ev.Value // 温度语义保留
			}
			pointID, err := h.repo.UpsertDevicePoint(r.Context(), point)
			if err != nil {
				parseFailed = true
				parseError = "upsert device point failed: " + err.Error()
				break
			}
			point.PointID = pointID // 回填 pointID，供后续 UpsertDeviceRealtimeMap 使用

			// 3c. 插入传感器时序数据。
			sd := &domain.SensorData{
				GatewayID:        gatewayID,
				NodeID:           &nodeID,
				PointID:          &pointID,
				Frequency:        ev.Frequency,
				DeviceExternalID: ev.DeviceExternalID,
				FunctionType:     domain.FunctionType(ev.FunctionType),
				Unit:             ev.Unit,
				RealValue:        ev.Value,
				RawEventID:       &rawEventID,
			}
			if ev.Value != nil {
				sd.RealTemp = *ev.Value
			}
			if err := h.repo.InsertSensorData(r.Context(), sd); err != nil {
				parseFailed = true
				parseError = "insert sensor data failed: " + err.Error()
				break
			}

			// 3d. 更新功能点实时状态。
			_ = h.repo.UpdateDevicePointRealtime(r.Context(), pointID, ev.Value, point.ValueStatus, nil)

			// 3e. 同步 device_realtime_maps 兼容投影表。
			_ = h.repo.UpsertDeviceRealtimeMap(r.Context(), point)

			parsedCount++

		case "control_result":
			// control_result：预留，暂只记录原始事件。
			// 后续控制链路实现后再扩展。

		default:
			parseFailed = true
			parseError = "unknown event_type: " + ev.EventType
		}
	}

	// 7. 回填解析元数据并更新状态。
	msgID := strPtr(envelope.MessageID)
	if err := h.repo.UpdateRawEventParseMeta(r.Context(), rawEventID, inferredEventType, totalCount, parsedCount, msgID); err != nil {
		log.Printf("[adapter] UpdateRawEventParseMeta failed: raw_event_id=%d err=%v", rawEventID, err)
	}

	var finalStatus domain.ParseStatus
	var finalError *string
	if parseFailed {
		// 有事件完全失败。
		if parsedCount > 0 {
			finalStatus = domain.ParseStatusPartial
			finalError = strPtr(parseError)
		} else {
			finalStatus = domain.ParseStatusFailed
			finalError = strPtr(parseError)
		}
	} else if partialFailed {
		finalStatus = domain.ParseStatusPartial
		finalError = strPtr(parseError)
	} else {
		finalStatus = domain.ParseStatusParsed
	}
	_ = h.repo.UpdateRawEventParseStatus(r.Context(), rawEventID, finalStatus, finalError)

	if parseFailed {
		writeCode(w, http.StatusUnprocessableEntity, 422, parseError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"raw_event_id": rawEventID,
			"gateway_id":   gatewayID,
			"status":       "ok",
		},
	})
}

func (h *adapterHandler) parseRawFrame(ctx context.Context, rawEventID uint64, envelope rawFrameEnvelope, payload []byte) (domain.ParseStatus, error) {
	trimmed := bytes.TrimSpace(payload)
	msg := protocol.ClassifyPayload(payload)
	if msg.Kind == protocol.MessageRegister {
		gatewayName := string(trimmed)
		if envelope.GatewayName != "" {
			gatewayName = envelope.GatewayName
		}
		gatewayExternalID := gatewayExternalIDFromName(gatewayName)
		gatewayID, err := h.repo.UpsertGatewayByExternalID(ctx, &domain.Gateway{
			GatewayExternalID: gatewayExternalID,
			GatewayName:       gatewayName,
			IP:                envelope.RemoteIP,
			Port:              envelope.RemotePort,
			OnlineStatus:      1,
			SourceType:        uint8(domain.SourceTypeGateway),
		})
		if err != nil {
			return domain.ParseStatusFailed, err
		}
		_ = gatewayID
		_ = h.repo.UpdateRawEventParseMeta(ctx, rawEventID, domain.RawEventRegister, 1, 1, strPtr(envelope.MessageID))
		_ = h.repo.UpdateRawEventParseStatus(ctx, rawEventID, domain.ParseStatusParsed, nil)
		return domain.ParseStatusParsed, nil
	}

	if msg.Kind == protocol.MessageHeartbeat {
		gatewayName := strings.TrimSpace(envelope.GatewayName)
		if gatewayName == "" {
			return domain.ParseStatusFailed, errRawFrameHeartbeatMissingGateway
		}
		gatewayExternalID := gatewayExternalIDFromName(gatewayName)
		_, err := h.repo.UpsertGatewayByExternalID(ctx, &domain.Gateway{
			GatewayExternalID: gatewayExternalID,
			GatewayName:       gatewayName,
			IP:                envelope.RemoteIP,
			Port:              envelope.RemotePort,
			OnlineStatus:      1,
			SourceType:        uint8(domain.SourceTypeGateway),
		})
		if err != nil {
			return domain.ParseStatusFailed, err
		}
		_ = h.repo.UpdateRawEventParseMeta(ctx, rawEventID, domain.RawEventHeartbeat, 1, 1, strPtr(envelope.MessageID))
		_ = h.repo.UpdateRawEventParseStatus(ctx, rawEventID, domain.ParseStatusParsed, nil)
		return domain.ParseStatusParsed, nil
	}

	return domain.ParseStatusPending, nil
}

func rawFrameEventType(payload []byte) domain.RawEventType {
	switch protocol.ClassifyPayload(payload).Kind {
	case protocol.MessageRegister:
		return domain.RawEventRegister
	case protocol.MessageHeartbeat:
		return domain.RawEventHeartbeat
	case protocol.MessageAlarm:
		return domain.RawEventAlarm
	case protocol.MessageControlResult:
		return domain.RawEventControlResult
	case protocol.MessageWorkData:
		return domain.RawEventWork
	default:
		return domain.RawEventUnknown
	}
}

func gatewayExternalIDFromName(gatewayName string) string {
	name := strings.TrimSpace(gatewayName)
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "FP_") {
		return name[3:]
	}
	if strings.HasPrefix(upper, "FP") {
		return strings.TrimLeft(name[2:], "_- ")
	}
	return name
}

var errRawFrameHeartbeatMissingGateway = rawFrameParseError("heartbeat missing gateway_name")

type rawFrameParseError string

func (e rawFrameParseError) Error() string { return string(e) }

// readAllBody 读取请求体全部字节，失败返回错误。
// 与 json.NewDecoder(r.Body).Decode() 不同，此函数先完整读取原始字节，
// 保证 payload_snapshot 中存储的是硬件原始数据，而非 Go 结构体二次序列化结果。
func readAllBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

func sourceTypeFromEnvelope(sourceType string) domain.SourceType {
	switch strings.TrimSpace(strings.ToLower(sourceType)) {
	case "mock_script", "mock", "mock_device_events":
		return domain.SourceTypeMock
	case "gateway", "gateway_adapter", "tcp_gateway":
		return domain.SourceTypeGateway
	case "adapter", "adapter_api":
		return domain.SourceTypeAdapter
	case "manual":
		return domain.SourceTypeManual
	default:
		return domain.SourceTypeAdapter
	}
}

// strPtr 返回字符串指针，nil 时返回 nil。
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// intPtr 返回整数指针。
func intPtr(i int) *int {
	return &i
}

func (h *adapterHandler) validAdapterToken(r *http.Request) bool {
	token := r.Header.Get("X-Adapter-Token")
	return token != "" && token == h.adapterToken
}
