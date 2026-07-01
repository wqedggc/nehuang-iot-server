package mysql

import (
	"context"
	"time"

	"nehuang-iot-server/internal/service"
)

// MarkOffline corrects stale online statuses using the provided cutoff time.
func (r *Repository) MarkOffline(ctx context.Context, cutoff time.Time) (service.OfflineScanResult, error) {
	var result service.OfflineScanResult

	res, err := r.db.ExecContext(ctx, `
UPDATE cloud_gateways
   SET online_status = 0, updated_at = NOW(3)
 WHERE online_status = 1
   AND last_heartbeat_at_ms < ?`, cutoff)
	if err != nil {
		return result, err
	}
	result.Gateways, _ = res.RowsAffected()

	res, err = r.db.ExecContext(ctx, `
UPDATE collector_nodes
   SET online_status = 0, updated_at = NOW(3)
 WHERE online_status = 1
   AND node_status = 1
   AND last_report_at_ms IS NOT NULL
   AND last_report_at_ms < ?`, cutoff)
	if err != nil {
		return result, err
	}
	result.Collectors, _ = res.RowsAffected()

	res, err = r.db.ExecContext(ctx, `
UPDATE device_points
   SET value_status = 6, updated_at = NOW(3)
 WHERE point_status = 1
   AND value_status <> 6
   AND last_report_at_ms IS NOT NULL
   AND last_report_at_ms < ?`, cutoff)
	if err != nil {
		return result, err
	}
	result.DevicePoints, _ = res.RowsAffected()

	res, err = r.db.ExecContext(ctx, `
UPDATE device_realtime_maps drm
JOIN device_points dp ON dp.point_id = drm.point_id
   SET drm.value_status = 6, drm.updated_at = NOW(3)
 WHERE dp.point_status = 1
   AND drm.value_status <> 6
   AND dp.last_report_at_ms IS NOT NULL
   AND dp.last_report_at_ms < ?`, cutoff)
	if err != nil {
		return result, err
	}
	result.RealtimeMaps, _ = res.RowsAffected()

	return result, nil
}
