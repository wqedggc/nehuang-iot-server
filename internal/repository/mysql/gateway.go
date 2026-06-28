package mysql

import (
	"context"
	"database/sql"
	"time"

	"nehuang-iot-server/internal/domain"
)

func (r *Repository) ListGateways(ctx context.Context, limit int) ([]domain.Gateway, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT
  gateway_id,
  gateway_external_id,
  gateway_name,
  ip,
  port,
  gateway_sn,
  online_status,
  source_type,
  last_heartbeat_at_ms,
  created_at,
  updated_at
FROM cloud_gateways
ORDER BY updated_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gateways := make([]domain.Gateway, 0)
	for rows.Next() {
		var gateway domain.Gateway
		if err := rows.Scan(
			&gateway.GatewayID,
			&gateway.GatewayExternalID,
			&gateway.GatewayName,
			&gateway.IP,
			&gateway.Port,
			&gateway.GatewaySN,
			&gateway.OnlineStatus,
			&gateway.SourceType,
			&gateway.LastHeartbeatAtMS,
			&gateway.CreatedAt,
			&gateway.UpdatedAt,
		); err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return gateways, nil
}

func (r *Repository) Dashboard(ctx context.Context) (map[string]any, error) {
	var gatewayCount int64
	var onlineGatewayCount int64
	var collectorCount int64
	var farmerCount int64
	var sensorDataCount int64

	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cloud_gateways`).Scan(&gatewayCount); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cloud_gateways WHERE online_status = 1`).Scan(&onlineGatewayCount); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM collector_nodes WHERE node_status = 1`).Scan(&collectorCount); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE user_type = 2 AND status = 1`).Scan(&farmerCount); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sensor_data`).Scan(&sensorDataCount); err != nil {
		return nil, err
	}

	lastRawEventAt, err := scanNullableTime(ctx, r.db, `SELECT MAX(received_at_ms) FROM raw_device_events`)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"gateway_count":        gatewayCount,
		"online_gateway_count": onlineGatewayCount,
		"collector_count":      collectorCount,
		"farmer_count":         farmerCount,
		"today_reports":        sensorDataCount,
		"last_raw_event_at_ms": lastRawEventAt,
	}, nil
}

func scanNullableTime(ctx context.Context, db *sql.DB, query string) (*time.Time, error) {
	var value sql.NullTime
	if err := db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return nil, err
	}
	if !value.Valid {
		return nil, nil
	}
	return &value.Time, nil
}
