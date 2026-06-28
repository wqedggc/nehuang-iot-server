package http

import (
	"context"
	"encoding/json"
	"net/http"

	"nehuang-iot-server/internal/domain"
)

// adapterHandler 处理 /api/v1/adapter/* 端点的设备事件上报。
type adapterHandler struct {
	repo   adapterRepository
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
	// FindGatewayByExternalID 按 gateway_external_id 查找云边设备，不存在返回 nil。
	FindGatewayByExternalID(ctx context.Context, externalID string) (*domain.Gateway, error)
}

// ─── Adapter 请求模型 ───

// deviceEventEnvelope 设备事件外层信封，对应实施计划中的统一事件结构。
type deviceEventEnvelope struct {
	ProtocolVersion     string              `json:"protocol_version"`
	SourceType          string              `json:"source_type"`
	MessageID           string              `json:"message_id"`
	GatewayName         string              `json:"gateway_name"`
	GatewayExternalID   string              `json:"gateway_external_id"`
	RemoteIP            string              `json:"remote_ip"`
	RemotePort          int                 `json:"remote_port"`
	ReportedAtMS        string              `json:"reported_at_ms"`
	Events              []deviceEvent       `json:"events"`
}

// deviceEvent 设备事件内层，对应 events[] 中的单条事件。
type deviceEvent struct {
	EventType         string   `json:"event_type"` // register / heartbeat / telemetry / control_result
	Frequency         int      `json:"frequency"`
	DeviceExternalID  string   `json:"device_external_id"`
	FunctionType      uint8    `json:"function_type"`
	ChannelNo         int      `json:"channel_no"`
	Value             *float64 `json:"value"`
	Unit              string   `json:"unit"`
}

// handleDeviceEvents 处理 POST /api/v1/adapter/device-events。
// 认证使用 X-Adapter-Token header，不走用户 JWT。
func (h *adapterHandler) handleDeviceEvents(w http.ResponseWriter, r *http.Request) {
	// 校验 Adapter Token。
	token := r.Header.Get("X-Adapter-Token")
	if token == "" || token != h.adapterToken {
		writeCode(w, http.StatusUnauthorized, 401, "invalid or missing adapter token")
		return
	}

	var envelope deviceEventEnvelope
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		writeCode(w, http.StatusBadRequest, 400, "invalid request body: "+err.Error())
		return
	}

	// 将整个 envelope 序列化为快照字符串。
	payloadBytes, _ := json.Marshal(envelope)
	payloadSnapshot := string(payloadBytes)

	// 1. 先写 raw_device_events（原始事件保留，不做推断）。
	rawEvent := &domain.RawDeviceEvent{
		GatewayName:     strPtr(envelope.GatewayName),
		EventType:       domain.RawEventWork, // 默认 work，后续按 event_type 细分
		SourceType:      domain.SourceTypeAdapter,
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

	// 2. 按 gateway_external_id 查找或创建云边设备。
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

	// 3. 逐条处理 events[]。
	parseFailed := false
	var parseError string
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
				SourceType:        uint8(domain.SourceTypeAdapter),
			}
			gid, err := h.repo.UpsertGatewayByExternalID(r.Context(), gw)
			if err != nil {
				parseFailed = true
				parseError = "upsert gateway failed: " + err.Error()
				break
			}
			gatewayID = gid

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

		case "control_result":
			// control_result：预留，暂只记录原始事件。
			// 后续控制链路实现后再扩展。

		default:
			parseFailed = true
			parseError = "unknown event_type: " + ev.EventType
		}
	}

	// 4. 更新 raw_device_events 解析状态。
	if parseFailed {
		_ = h.repo.UpdateRawEventParseStatus(r.Context(), rawEventID, domain.ParseStatusFailed, strPtr(parseError))
	} else {
		_ = h.repo.UpdateRawEventParseStatus(r.Context(), rawEventID, domain.ParseStatusParsed, nil)
	}

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
