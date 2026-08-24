package http

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nehuang-iot-server/internal/domain"
)

type mockAdapterRepo struct {
	insertedRawEvents []domain.RawDeviceEvent
	rawEventID        uint64
	upsertedGateways  []domain.Gateway
	parseStatus       domain.ParseStatus
	parseStatusErr    *string
	parseMetaType     domain.RawEventType
	parseMetaCount    int
	parsedMetaCount   int
	parseMetaMessage  *string
	upsertedNodes     []domain.CollectorNode
	upsertedPoints    []domain.DevicePoint
	sensorData        []domain.SensorData
	realtimeUpdates   []domain.DevicePoint
	duplicateCapture  bool
}

func (m *mockAdapterRepo) InsertRawEvent(ctx context.Context, event *domain.RawDeviceEvent) (uint64, bool, error) {
	if m.rawEventID == 0 {
		m.rawEventID = 1001
	}
	event.RawEventID = m.rawEventID
	m.insertedRawEvents = append(m.insertedRawEvents, *event)
	return m.rawEventID, !m.duplicateCapture, nil
}

func (m *mockAdapterRepo) UpsertGatewayByExternalID(ctx context.Context, gateway *domain.Gateway) (uint64, error) {
	m.upsertedGateways = append(m.upsertedGateways, *gateway)
	return 2001, nil
}

func (m *mockAdapterRepo) UpdateRawEventParseStatus(ctx context.Context, rawEventID uint64, status domain.ParseStatus, errMsg *string) error {
	m.parseStatus = status
	m.parseStatusErr = errMsg
	return nil
}

func (m *mockAdapterRepo) UpdateRawEventParseMeta(ctx context.Context, rawEventID uint64, eventType domain.RawEventType, eventCount int, parsedCount int, messageID *string) error {
	m.parseMetaType = eventType
	m.parseMetaCount = eventCount
	m.parsedMetaCount = parsedCount
	m.parseMetaMessage = messageID
	return nil
}

func (m *mockAdapterRepo) UpsertCollectorNode(ctx context.Context, node *domain.CollectorNode) (uint64, error) {
	m.upsertedNodes = append(m.upsertedNodes, *node)
	return 3001, nil
}
func (m *mockAdapterRepo) UpsertDevicePoint(ctx context.Context, point *domain.DevicePoint) (uint64, error) {
	m.upsertedPoints = append(m.upsertedPoints, *point)
	return 4001, nil
}
func (m *mockAdapterRepo) InsertSensorData(ctx context.Context, data *domain.SensorData) error {
	m.sensorData = append(m.sensorData, *data)
	return nil
}
func (m *mockAdapterRepo) UpdateDevicePointRealtime(ctx context.Context, pointID uint64, realValue *float64, valueStatus domain.ValueStatus, lastReportAt interface{}) error {
	return nil
}
func (m *mockAdapterRepo) UpsertDeviceRealtimeMap(ctx context.Context, point *domain.DevicePoint) error {
	m.realtimeUpdates = append(m.realtimeUpdates, *point)
	return nil
}
func (m *mockAdapterRepo) FindGatewayByExternalID(ctx context.Context, externalID string) (*domain.Gateway, error) {
	return nil, nil
}

func TestHandleRawFramesParsesMockTelemetry(t *testing.T) {
	repo := &mockAdapterRepo{}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	payload := "NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=1|channel_no=1|value=26.5|unit=C"
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-1-3","connection_id":"conn-1","gateway_name":"FP_TEST_ADMIN_001","remote_ip":"127.0.0.1","remote_port":63254,"direction":"inbound","payload_hex":"` + strings.ToUpper(hex.EncodeToString([]byte(payload))) + `","received_at":"2026-07-04T18:08:05Z","raw_seq":3}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()

	handler.handleRawFrames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleRawFrames: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.insertedRawEvents[0].EventType != domain.RawEventWork {
		t.Fatalf("event type: want work, got %v", repo.insertedRawEvents[0].EventType)
	}
	if repo.insertedRawEvents[0].CaptureID == nil || *repo.insertedRawEvents[0].CaptureID != "conn-1-3" {
		t.Fatalf("capture id: %+v", repo.insertedRawEvents[0].CaptureID)
	}
	if repo.parseStatus != domain.ParseStatusParsed {
		t.Fatalf("parse status: want parsed, got %v err=%v", repo.parseStatus, repo.parseStatusErr)
	}
	if len(repo.upsertedGateways) != 1 {
		t.Fatalf("gateway upserts: want 1, got %d", len(repo.upsertedGateways))
	}
	if len(repo.upsertedNodes) != 1 {
		t.Fatalf("node upserts: want 1, got %d", len(repo.upsertedNodes))
	}
	if repo.upsertedNodes[0].DeviceExternalID != "1001" || repo.upsertedNodes[0].Frequency != 1 || repo.upsertedNodes[0].OnlineStatus != 1 {
		t.Fatalf("node mismatch: %+v", repo.upsertedNodes[0])
	}
	if len(repo.upsertedPoints) != 1 {
		t.Fatalf("point upserts: want 1, got %d", len(repo.upsertedPoints))
	}
	point := repo.upsertedPoints[0]
	if point.FunctionType != domain.FuncTypeTemperature || point.ChannelNo != 1 || point.Unit != "C" || point.RealValue == nil || *point.RealValue != 26.5 || point.ValueStatus != domain.ValueStatusNormal {
		t.Fatalf("point mismatch: %+v", point)
	}
	if len(repo.sensorData) != 1 {
		t.Fatalf("sensor data inserts: want 1, got %d", len(repo.sensorData))
	}
	if repo.sensorData[0].PointID == nil || *repo.sensorData[0].PointID != 4001 {
		t.Fatalf("sensor point id mismatch: %+v", repo.sensorData[0].PointID)
	}
	if len(repo.realtimeUpdates) != 1 || repo.realtimeUpdates[0].PointID != 4001 {
		t.Fatalf("realtime update mismatch: %+v", repo.realtimeUpdates)
	}
	if repo.parseMetaType != domain.RawEventWork || repo.parseMetaCount != 1 || repo.parsedMetaCount != 1 {
		t.Fatalf("parse meta mismatch: type=%v count=%d parsed=%d", repo.parseMetaType, repo.parseMetaCount, repo.parsedMetaCount)
	}
}

func TestHandleRawFramesRejectsMockTelemetryMissingRequiredFields(t *testing.T) {
	repo := &mockAdapterRepo{}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	payload := "NH_TELEMETRY|frequency=1|function_type=1|channel_no=1|value=26.5|unit=C"
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-1-3","connection_id":"conn-1","gateway_name":"FP_TEST_ADMIN_001","remote_ip":"127.0.0.1","remote_port":63254,"direction":"inbound","payload_hex":"` + strings.ToUpper(hex.EncodeToString([]byte(payload))) + `","received_at":"2026-07-04T18:08:05Z","raw_seq":3}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()

	handler.handleRawFrames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleRawFrames should still store raw and return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.parseStatus != domain.ParseStatusFailed {
		t.Fatalf("parse status: want failed, got %v", repo.parseStatus)
	}
	if repo.parseStatusErr == nil || !strings.Contains(*repo.parseStatusErr, "device_external_id") {
		t.Fatalf("parse error should mention missing field, got %#v", repo.parseStatusErr)
	}
	if len(repo.upsertedNodes) != 0 || len(repo.upsertedPoints) != 0 || len(repo.sensorData) != 0 {
		t.Fatalf("invalid telemetry must not project business data")
	}
}
func (m *mockAdapterRepo) ListPendingControlCommands(ctx context.Context, gatewayExternalID string, limit int) ([]domain.ControlCommand, error) {
	return nil, nil
}
func (m *mockAdapterRepo) MarkControlCommandSent(ctx context.Context, commandID uint64, gatewayResponse *string) error {
	return nil
}
func (m *mockAdapterRepo) ApplyControlCommandResult(ctx context.Context, input domain.AdapterCommandResultInput) error {
	return nil
}

func TestHandleRawFramesStoresRawPayloadAndParsesRegister(t *testing.T) {
	repo := &mockAdapterRepo{}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-1-1","connection_id":"conn-1","gateway_name":"","remote_ip":"127.0.0.1","remote_port":63254,"direction":"inbound","payload_hex":"46505F544553545F41444D494E5F3030310A","received_at":"2026-07-04T18:08:04.79022Z","raw_seq":1}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()

	handler.handleRawFrames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleRawFrames: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.insertedRawEvents) != 1 {
		t.Fatalf("InsertRawEvent calls: want 1, got %d", len(repo.insertedRawEvents))
	}
	inserted := repo.insertedRawEvents[0]
	if inserted.PayloadSnapshot == nil || *inserted.PayloadSnapshot != body {
		t.Fatalf("payload snapshot must preserve raw request body")
	}
	if inserted.PayloadHex == nil || *inserted.PayloadHex != "46505F544553545F41444D494E5F3030310A" {
		t.Fatalf("payload hex mismatch: %#v", inserted.PayloadHex)
	}
	if len(repo.upsertedGateways) != 1 {
		t.Fatalf("UpsertGatewayByExternalID calls: want 1, got %d", len(repo.upsertedGateways))
	}
	if repo.upsertedGateways[0].GatewayExternalID != "TEST_ADMIN_001" {
		t.Fatalf("gateway external id: want TEST_ADMIN_001, got %q", repo.upsertedGateways[0].GatewayExternalID)
	}
	if repo.upsertedGateways[0].GatewayName != "FP_TEST_ADMIN_001" {
		t.Fatalf("gateway name: want FP_TEST_ADMIN_001, got %q", repo.upsertedGateways[0].GatewayName)
	}
	if repo.parseStatus != domain.ParseStatusParsed {
		t.Fatalf("parse status: want parsed, got %v", repo.parseStatus)
	}
	if repo.parseMetaType != domain.RawEventRegister || repo.parseMetaCount != 1 || repo.parsedMetaCount != 1 {
		t.Fatalf("parse meta mismatch: type=%v count=%d parsed=%d", repo.parseMetaType, repo.parseMetaCount, repo.parsedMetaCount)
	}
}

func TestHandleRawFramesParsesCoalescedRegisterWithoutDirtyGatewayName(t *testing.T) {
	repo := &mockAdapterRepo{}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	payload := []byte("FP_TEST_ADMIN_002\nkeepalive")
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-1-1","connection_id":"conn-1","gateway_name":"","remote_ip":"127.0.0.1","remote_port":63254,"direction":"inbound","payload_hex":"` + strings.ToUpper(hex.EncodeToString(payload)) + `","received_at":"2026-07-04T18:08:04.79022Z","raw_seq":1}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()

	handler.handleRawFrames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleRawFrames: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.upsertedGateways) != 1 {
		t.Fatalf("UpsertGatewayByExternalID calls: want 1, got %d", len(repo.upsertedGateways))
	}
	if got, want := repo.upsertedGateways[0].GatewayExternalID, "TEST_ADMIN_002"; got != want {
		t.Fatalf("gateway external id: want %q, got %q", want, got)
	}
	if got, want := repo.upsertedGateways[0].GatewayName, "FP_TEST_ADMIN_002"; got != want {
		t.Fatalf("gateway name: want %q, got %q", want, got)
	}
}

func TestHandleRawFramesTrimsEnvelopeGatewayName(t *testing.T) {
	repo := &mockAdapterRepo{}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-1-2","connection_id":"conn-1","gateway_name":" FP_TEST_ADMIN_001\n","remote_ip":"127.0.0.1","remote_port":63254,"direction":"inbound","payload_hex":"6B656570616C697665","received_at":"2026-07-04T18:08:05.79022Z","raw_seq":2}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()

	handler.handleRawFrames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleRawFrames: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.insertedRawEvents) != 1 {
		t.Fatalf("InsertRawEvent calls: want 1, got %d", len(repo.insertedRawEvents))
	}
	if repo.insertedRawEvents[0].GatewayName == nil || *repo.insertedRawEvents[0].GatewayName != "FP_TEST_ADMIN_001" {
		t.Fatalf("gateway_name should be trimmed, got %#v", repo.insertedRawEvents[0].GatewayName)
	}
}

func TestHandleRawFramesKeepsUnknownPayloadPending(t *testing.T) {
	repo := &mockAdapterRepo{}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-1-2","connection_id":"conn-1","gateway_name":"FP_TEST_ADMIN_001","remote_ip":"127.0.0.1","remote_port":63254,"direction":"inbound","payload_hex":"01020304","received_at":"2026-07-04T18:08:05Z","raw_seq":2}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()

	handler.handleRawFrames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleRawFrames: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.insertedRawEvents) != 1 {
		t.Fatalf("InsertRawEvent calls: want 1, got %d", len(repo.insertedRawEvents))
	}
	if repo.insertedRawEvents[0].ParseStatus != domain.ParseStatusPending {
		t.Fatalf("insert parse status: want pending, got %v", repo.insertedRawEvents[0].ParseStatus)
	}
	if repo.insertedRawEvents[0].EventType != domain.RawEventUnknown {
		t.Fatalf("event type: want unknown, got %v", repo.insertedRawEvents[0].EventType)
	}
	if repo.parseStatus != 0 {
		t.Fatalf("unknown payload should not update parse status, got %v", repo.parseStatus)
	}
	if len(repo.upsertedGateways) != 0 {
		t.Fatalf("unknown payload should not upsert gateway")
	}
	var resp map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["data"]["parse_status"] != domain.ParseStatusPending.ParseStatusName() {
		t.Fatalf("response parse_status mismatch: %#v", resp)
	}
}

func TestHandleRawFramesParsesKeepaliveHeartbeat(t *testing.T) {
	repo := &mockAdapterRepo{}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-1-2","connection_id":"conn-1","gateway_name":"FP_TEST_ADMIN_001","remote_ip":"127.0.0.1","remote_port":63254,"direction":"inbound","payload_hex":"6B656570616C697665","received_at":"2026-07-04T18:08:05Z","raw_seq":2}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()

	handler.handleRawFrames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleRawFrames: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.insertedRawEvents) != 1 {
		t.Fatalf("InsertRawEvent calls: want 1, got %d", len(repo.insertedRawEvents))
	}
	if repo.insertedRawEvents[0].EventType != domain.RawEventHeartbeat {
		t.Fatalf("event type: want heartbeat, got %v", repo.insertedRawEvents[0].EventType)
	}
	if len(repo.upsertedGateways) != 1 {
		t.Fatalf("UpsertGatewayByExternalID calls: want 1, got %d", len(repo.upsertedGateways))
	}
	if repo.upsertedGateways[0].GatewayExternalID != "TEST_ADMIN_001" {
		t.Fatalf("gateway external id: want TEST_ADMIN_001, got %q", repo.upsertedGateways[0].GatewayExternalID)
	}
	if repo.parseStatus != domain.ParseStatusParsed {
		t.Fatalf("parse status: want parsed, got %v", repo.parseStatus)
	}
	if repo.parseMetaType != domain.RawEventHeartbeat || repo.parseMetaCount != 1 || repo.parsedMetaCount != 1 {
		t.Fatalf("parse meta mismatch: type=%v count=%d parsed=%d", repo.parseMetaType, repo.parseMetaCount, repo.parsedMetaCount)
	}
}

func TestHandleRawFramesStoresAlarmPayloadAsAlarmPending(t *testing.T) {
	repo := &mockAdapterRepo{}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-1-4","connection_id":"conn-1","gateway_name":"FP_TEST_ADMIN_001","remote_ip":"127.0.0.1","remote_port":63254,"direction":"inbound","payload_hex":"AAAA0BFFFFFF012CD5","received_at":"2026-07-04T18:08:05Z","raw_seq":4}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()

	handler.handleRawFrames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleRawFrames: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.insertedRawEvents) != 1 {
		t.Fatalf("InsertRawEvent calls: want 1, got %d", len(repo.insertedRawEvents))
	}
	if repo.insertedRawEvents[0].EventType != domain.RawEventAlarm {
		t.Fatalf("event type: want alarm, got %v", repo.insertedRawEvents[0].EventType)
	}
	if repo.insertedRawEvents[0].ParseStatus != domain.ParseStatusPending {
		t.Fatalf("parse status: want pending, got %v", repo.insertedRawEvents[0].ParseStatus)
	}
	if repo.parseStatus != 0 {
		t.Fatalf("alarm payload should not update parse status, got %v", repo.parseStatus)
	}
}

func TestHandleRawFramesRejectsMissingTokenAndInvalidHex(t *testing.T) {
	handler := &adapterHandler{repo: &mockAdapterRepo{}, adapterToken: "test-token"}
	body := `{"payload_hex":"not-hex"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.handleRawFrames(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: want 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec = httptest.NewRecorder()
	handler.handleRawFrames(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid hex: want 400, got %d", rec.Code)
	}
}

func TestHandleRawFramesRejectsMissingCaptureIdentity(t *testing.T) {
	handler := &adapterHandler{repo: &mockAdapterRepo{}, adapterToken: "test-token"}
	body := `{"direction":"inbound","payload_hex":"0102","raw_seq":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()
	handler.handleRawFrames(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing message_id: want 400, got %d", rec.Code)
	}
}

func TestHandleRawFramesDoesNotProjectDuplicateCapture(t *testing.T) {
	repo := &mockAdapterRepo{duplicateCapture: true}
	handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
	payload := "NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=1|channel_no=1|value=26.5|unit=C"
	body := `{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"conn-duplicate-1","connection_id":"conn-duplicate","direction":"inbound","payload_hex":"` + strings.ToUpper(hex.EncodeToString([]byte(payload))) + `","received_at":"2026-07-04T18:08:05Z","raw_seq":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(body))
	req.Header.Set("X-Adapter-Token", "test-token")
	rec := httptest.NewRecorder()
	handler.handleRawFrames(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("duplicate capture: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.sensorData) != 0 || len(repo.upsertedPoints) != 0 {
		t.Fatalf("duplicate capture projected business data: sensors=%d points=%d", len(repo.sensorData), len(repo.upsertedPoints))
	}
	if !strings.Contains(rec.Body.String(), `"duplicate":true`) {
		t.Fatalf("duplicate response missing marker: %s", rec.Body.String())
	}
}
