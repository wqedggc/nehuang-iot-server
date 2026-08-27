package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nehuang-iot-server/internal/domain"
)

type adminWorkbenchRepo struct {
	mockAdminRepo
	farmer             *domain.User
	devices            []domain.MiniDevice
	detail             *domain.MiniDeviceDetail
	history            []domain.MiniHistoryPoint
	listedUserID       uint64
	historyPointID     uint64
	historyRange       string
	historyQueryCalled bool
	visible            bool
	checkedUserID      uint64
	checkedPointID     uint64
}

func (r *adminWorkbenchRepo) GetFarmerByID(ctx context.Context, userID uint64) (*domain.User, int, error) {
	return r.farmer, len(r.devices), nil
}

func (r *adminWorkbenchRepo) ListMiniDevices(ctx context.Context, userID uint64) ([]domain.MiniDevice, error) {
	r.listedUserID = userID
	return r.devices, nil
}

func (r *adminWorkbenchRepo) GetMiniDeviceDetail(ctx context.Context, pointID uint64) (*domain.MiniDeviceDetail, error) {
	return r.detail, nil
}

func (r *adminWorkbenchRepo) GetMiniDeviceHistory(ctx context.Context, pointID uint64, rangeType string) ([]domain.MiniHistoryPoint, error) {
	r.historyQueryCalled = true
	r.historyPointID = pointID
	r.historyRange = rangeType
	return r.history, nil
}

func (r *adminWorkbenchRepo) CheckBindRelation(ctx context.Context, userID uint64, pointID uint64) (bool, error) {
	r.checkedUserID = userID
	r.checkedPointID = pointID
	return r.visible, nil
}

func TestAdminFarmerDevicesReturnsBoundDeviceProjection(t *testing.T) {
	name := "张三"
	realValue := 23.6
	repo := &adminWorkbenchRepo{
		farmer: &domain.User{UserID: 42, DisplayName: &name},
		devices: []domain.MiniDevice{{
			PointID:          1001,
			NodeID:           501,
			GatewayID:        201,
			GatewayName:      "一号棚网关",
			NodeName:         "卷帘执行器",
			ShedNo:           "1号棚",
			FunctionName:     "温度",
			RealValue:        &realValue,
			Unit:             "℃",
			ValueStatus:      uint8(domain.ValueStatusNormal),
			ValueStatusLabel: "正常",
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/farmers/42/devices", nil)
	rec := httptest.NewRecorder()
	(&adminHandler{repo: repo}).handleFarmerDevices(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.listedUserID != 42 {
		t.Fatalf("want repository user 42, got %d", repo.listedUserID)
	}
	body := rec.Body.String()
	for _, want := range []string{`"point_id":"1001"`, `"gateway_name":"一号棚网关"`, `"real_value":23.6`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
}

func TestAdminFarmerDevicesReturns404ForUnknownFarmer(t *testing.T) {
	repo := &adminWorkbenchRepo{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/farmers/404/devices", nil)
	rec := httptest.NewRecorder()

	(&adminHandler{repo: repo}).handleFarmerDevices(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminDeviceHistoryReturnsPointMetadataAndSamples(t *testing.T) {
	bestMin := 18.0
	bestMax := 28.0
	value := 23.6
	repo := &adminWorkbenchRepo{
		farmer:  &domain.User{UserID: 42},
		visible: true,
		detail: &domain.MiniDeviceDetail{
			PointID:      1001,
			FunctionName: "温度",
			Unit:         "℃",
			BestMin:      &bestMin,
			BestMax:      &bestMax,
		},
		history: []domain.MiniHistoryPoint{{Time: "2026-08-27T00:00:00.000Z", RealTemp: 23.6, Value: &value}},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/farmers/42/devices/1001/history?range=7d", nil)
	rec := httptest.NewRecorder()
	(&adminHandler{repo: repo}).handleFarmerDeviceHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.historyPointID != 1001 || repo.historyRange != "7d" {
		t.Fatalf("unexpected history query: point=%d range=%s", repo.historyPointID, repo.historyRange)
	}
	if repo.checkedUserID != 42 || repo.checkedPointID != 1001 {
		t.Fatalf("unexpected visibility query: user=%d point=%d", repo.checkedUserID, repo.checkedPointID)
	}
	body := rec.Body.String()
	for _, want := range []string{`"point_id":"1001"`, `"func_name":"温度"`, `"real_temp":23.6`, `"value":23.6`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
}

func TestAdminDeviceHistoryRejectsInvalidRangeBeforeQuery(t *testing.T) {
	repo := &adminWorkbenchRepo{
		farmer: &domain.User{UserID: 42},
		detail: &domain.MiniDeviceDetail{PointID: 1001, FunctionName: "温度"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/farmers/42/devices/1001/history?range=forever", nil)
	rec := httptest.NewRecorder()

	(&adminHandler{repo: repo}).handleFarmerDeviceHistory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.historyQueryCalled {
		t.Fatal("invalid range must not query history")
	}
}

func TestAdminDeviceHistoryReturns404OutsideFarmerScope(t *testing.T) {
	repo := &adminWorkbenchRepo{
		farmer:  &domain.User{UserID: 42},
		visible: false,
		detail:  &domain.MiniDeviceDetail{PointID: 1001, FunctionName: "温度"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/farmers/42/devices/1001/history?range=1d", nil)
	rec := httptest.NewRecorder()

	(&adminHandler{repo: repo}).handleFarmerDeviceHistory(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.historyQueryCalled {
		t.Fatal("out-of-scope point must not query history")
	}
}
