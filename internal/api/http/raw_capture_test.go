package http

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/gatewayadapter/protocol"
)

func TestRawCaptureRetainsDamagedCRCAndUnknownBytesWithoutProjection(t *testing.T) {
	damaged := protocol.AppendCRC16Modbus([]byte{0x03, 0xE9, 0x02, 0x01, 0x02, 0x03, 0x14, 0xC0})
	damaged[len(damaged)-1] ^= 0x01
	for _, tc := range []struct {
		name    string
		payload []byte
	}{{"damaged CRC", damaged}, {"unknown bytes", []byte{0xDE, 0xAD, 0xBE, 0xEF}}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockAdapterRepo{}
			handler := &adapterHandler{repo: repo, adapterToken: "test-token"}
			originalHex := strings.ToUpper(hex.EncodeToString(tc.payload))
			envelope := rawFrameEnvelope{ProtocolVersion: "nehuang-gateway-adapter-v1", SourceType: "gateway_adapter", MessageID: "conn-crc-7", CaptureID: "conn-crc-7", ConnectionID: "conn-crc", GatewayName: "FP_TEST", Direction: "inbound", PayloadHex: originalHex, ReceivedAt: "2026-09-30T08:00:00Z", RawSeq: 7}
			body, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/adapter/raw-frames", strings.NewReader(string(body)))
			req.Header.Set("X-Adapter-Token", "test-token")
			rec := httptest.NewRecorder()
			handler.handleRawFrames(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("capture rejected: %d %s", rec.Code, rec.Body.String())
			}
			if len(repo.insertedRawEvents) != 1 {
				t.Fatalf("raw records: %d", len(repo.insertedRawEvents))
			}
			raw := repo.insertedRawEvents[0]
			if raw.PayloadHex == nil || *raw.PayloadHex != originalHex {
				t.Fatalf("original bytes changed: %#v", raw.PayloadHex)
			}
			if raw.PayloadSnapshot == nil || *raw.PayloadSnapshot != string(body) {
				t.Fatal("capture envelope not retained exactly")
			}
			if raw.CaptureID == nil || *raw.CaptureID != "conn-crc-7" || raw.MessageID == nil || *raw.MessageID != "conn-crc-7" {
				t.Fatal("trace identity missing")
			}
			if raw.ParseStatus != domain.ParseStatusPending {
				t.Fatalf("unexpected parse state %v", raw.ParseStatus)
			}
			if len(repo.sensorData) != 0 || len(repo.upsertedPoints) != 0 || len(repo.upsertedNodes) != 0 {
				t.Fatal("unconfirmed raw payload projected business data")
			}
		})
	}
}

func TestOutboundRawCaptureNeverProjectsBusinessTelemetry(t *testing.T) {
	repo := &mockAdapterRepo{}
	h := &adapterHandler{repo: repo}
	payload := []byte("NH_TELEMETRY|frequency=1|device_external_id=TEST|function_type=1|value=25.3|unit=C")
	status, err := h.parseRawFrame(t.Context(), 1, rawFrameEnvelope{Direction: "outbound", GatewayName: "FP_TEST"}, payload)
	if err != nil || status != domain.ParseStatusPending || len(repo.sensorData) != 0 || len(repo.upsertedPoints) != 0 || len(repo.upsertedNodes) != 0 {
		t.Fatalf("outbound projected status %v %v", status, err)
	}
}
