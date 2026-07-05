package mysql

import (
	"context"
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
  access_mode,
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
			&gateway.AccessMode,
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
		gateway.AccessMode = string(domain.NormalizeGatewayAccessMode(gateway.AccessMode))
		gateways = append(gateways, gateway)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return gateways, nil
}

func (r *Repository) UpdateGatewayAccessMode(ctx context.Context, gatewayID uint64, mode domain.GatewayAccessMode) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE cloud_gateways
SET access_mode = ?, updated_at = ?
WHERE gateway_id = ?`, string(domain.NormalizeGatewayAccessMode(string(mode))), time.Now(), gatewayID)
	return err
}

// Dashboard 统计已迁移至 dashboard_repository.go（独立 SQL 方法），
// 由 service.DashboardService 聚合调用。旧 mock 实现已移除。
