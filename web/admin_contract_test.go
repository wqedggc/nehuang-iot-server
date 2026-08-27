package web

import (
	"strings"
	"testing"
)

func readAdminAsset(t *testing.T, name string) string {
	t.Helper()
	b, err := Static.ReadFile("static/admin/" + name)
	if err != nil {
		t.Fatalf("read admin asset %s: %v", name, err)
	}
	return string(b)
}

func readStaticAsset(t *testing.T, name string) string {
	t.Helper()
	b, err := Static.ReadFile("static/" + name)
	if err != nil {
		t.Fatalf("read static asset %s: %v", name, err)
	}
	return string(b)
}

func TestAdminDashboardUsesBackendControlContract(t *testing.T) {
	app := readAdminAsset(t, "app.js")
	for _, absent := range []string{"control.control_pending_count", "control.control_sent_count", "control.control_succeeded_count"} {
		if strings.Contains(app, absent) {
			t.Fatalf("admin dashboard still reads unsupported field %q", absent)
		}
	}
	for _, present := range []string{"control.control_success_count", "control.control_failed_count", "control.control_timeout_count"} {
		if !strings.Contains(app, present) {
			t.Fatalf("admin dashboard must read backend field %q", present)
		}
	}
}

func TestAdminDashboardDoesNotAddRawFactsToTelemetryProjection(t *testing.T) {
	app := readAdminAsset(t, "app.js")
	misleading := "(toNum(ingest.raw_event_count) || 0) + (toNum(ingest.telemetry_count) || 0)"
	if strings.Contains(app, misleading) {
		t.Fatal("raw facts and telemetry projections must be displayed separately")
	}
}

func TestAdminFarmerWorkbenchUsesFarmerScopedAdminEndpoints(t *testing.T) {
	client := readStaticAsset(t, "shared/api-client.js")
	for _, present := range []string{"getFarmerDevices", "getFarmerDeviceHistory", "/api/v1/admin/farmers/", "/devices/"} {
		if !strings.Contains(client, present) {
			t.Fatalf("admin API client missing farmer workbench contract %q", present)
		}
	}
	app := readAdminAsset(t, "app.js")
	for _, present := range []string{"farmer-workbench", "getFarmerDevices", "getFarmerDeviceHistory"} {
		if !strings.Contains(app, present) {
			t.Fatalf("admin UI missing farmer workbench behavior %q", present)
		}
	}
}

func TestAdminDashboardUsesDenseActionableLayout(t *testing.T) {
	app := readAdminAsset(t, "app.js")
	for _, present := range []string{"dashboard-health-strip", "dashboard-priority-list", "dashboard-pipeline", "未终态"} {
		if !strings.Contains(app, present) {
			t.Fatalf("admin dashboard missing actionable layout marker %q", present)
		}
	}
	for _, absent := range []string{"已绑定 ", " / 未绑定 ", " / 未知 "} {
		if strings.Contains(app, absent) {
			t.Fatalf("admin dashboard still exposes misleading summary %q", absent)
		}
	}
}
