package http

import (
	"context"
	"encoding/json"
	"nehuang-iot-server/internal/domain"
	"net/http/httptest"
	"strings"
	"testing"
)

type contractRepo struct{ *mockAdminRepo }

func (r *contractRepo) ListCollectors(context.Context) ([]domain.CollectorNode, error) {
	id := uint64(9007199254740993)
	return []domain.CollectorNode{{NodeID: id, GatewayID: id, LastRawEventID: &id}}, nil
}
func (r *contractRepo) ListRawEvents(context.Context, int) ([]domain.RawDeviceEvent, error) {
	id := uint64(9007199254740993)
	return []domain.RawDeviceEvent{{RawEventID: id, GatewayID: &id}}, nil
}
func TestLegacyListsPreserveSnowflakeIDsAsStrings(t *testing.T) {
	h := &adminHandler{repo: &contractRepo{&mockAdminRepo{}}}
	for _, kind := range []string{"collectors", "raw-events"} {
		t.Run(kind, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/api/v1/admin/"+kind, nil)
			if kind == "collectors" {
				h.handleCollectors(w, r)
			} else {
				h.handleRawEvents(w, r)
			}
			var p struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if len(p.Data) != 1 {
				t.Fatal(w.Body.String())
			}
			item := p.Data[0]
			key := "node_id"
			if kind == "raw-events" {
				key = "raw_event_id"
			}
			if item[key] != "9007199254740993" || item["gateway_id"] != "9007199254740993" {
				t.Fatalf("ID contract lost: %v", item)
			}
			if kind == "collectors" && item["last_raw_event_id"] != "9007199254740993" {
				t.Fatal(item)
			}
			if _, ok := item["NodeID"]; ok {
				t.Fatal("Go field names leaked")
			}
			if kind == "raw-events" && item["point_id"] != nil {
				t.Fatal("nil ID changed")
			}
		})
	}
}
func TestFarmerInvalidStatusRejectedBeforeRepository(t *testing.T) {
	h := &adminHandler{repo: &mockAdminRepo{}}
	for _, status := range []string{"0", "99"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("PATCH", "/api/v1/admin/farmers/42", strings.NewReader(`{"status":`+status+`}`))
		h.handleFarmerUpdate(w, r)
		if w.Code != 400 {
			t.Fatalf("status %s accepted: %d", status, w.Code)
		}
	}
}
