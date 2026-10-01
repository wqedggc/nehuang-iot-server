package mysql

import (
	"context"
	"database/sql"
	"time"
)

// ─── Dashboard 统计 ───
// 所有雪花 ID 返回 uint64，调用方用 idStr() 序列化为字符串。
// rangeCutoff 由 service 层计算后传入，避免 repository 层耦合 range 逻辑。

// CountUsers 统计用户数量（status <> 3，即不含已删除）。
// 返回 total_users, admin_count, farmer_count, active_farmer_count, disabled_farmer_count。
func (r *Repository) CountUsers(ctx context.Context) (totalUsers int64, adminCount int64, farmerCount int64, activeFarmerCount int64, disabledFarmerCount int64, err error) {
	// 总用户数（不含已删除）
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE status <> 3`,
	).Scan(&totalUsers); err != nil {
		return
	}

	// 管理员数量（user_type=1, status<>3）
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE user_type = 1 AND status <> 3`,
	).Scan(&adminCount); err != nil {
		return
	}

	// 农户数量（user_type=2, status<>3）
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE user_type = 2 AND status <> 3`,
	).Scan(&farmerCount); err != nil {
		return
	}

	// 活跃农户（user_type=2, status=1）
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE user_type = 2 AND status = 1`,
	).Scan(&activeFarmerCount); err != nil {
		return
	}

	// 禁用农户（user_type=2, status=2）
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE user_type = 2 AND status = 2`,
	).Scan(&disabledFarmerCount); err != nil {
		return
	}

	return
}

// CountDeviceScale 统计设备规模。
// 返回 gateway_total, gateway_online, gateway_offline, collector_node_total,
//
//	device_point_total, bound_point_count, unbound_point_count。
func (r *Repository) CountDeviceScale(ctx context.Context) (gatewayTotal int64, gatewayOnline int64, gatewayOffline int64, collectorNodeTotal int64, devicePointTotal int64, boundPointCount int64, unboundPointCount int64, err error) {
	// 云边设备总数
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cloud_gateways`,
	).Scan(&gatewayTotal); err != nil {
		return
	}

	// 在线云边设备
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cloud_gateways WHERE online_status = 1`,
	).Scan(&gatewayOnline); err != nil {
		return
	}

	// 离线云边设备
	gatewayOffline = gatewayTotal - gatewayOnline

	// 采集执行器总数（node_status=1）
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM collector_nodes WHERE node_status = 1`,
	).Scan(&collectorNodeTotal); err != nil {
		return
	}

	// 功能点总数（point_status <> 3，即不含已删除）
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM device_points WHERE point_status <> 3`,
	).Scan(&devicePointTotal); err != nil {
		return
	}

	// 已绑定功能点数（bind_status=1）
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT point_id) FROM bind_relations WHERE bind_status = 1`,
	).Scan(&boundPointCount); err != nil {
		return
	}

	// 未绑定功能点数
	unboundPointCount = devicePointTotal - boundPointCount

	return
}

// CountDevicePointStatus 统计功能点状态分类。
// 返回 normal_point_count, abnormal_point_count, offline_point_count。
func (r *Repository) CountDevicePointStatus(ctx context.Context) (normalCount int64, abnormalCount int64, offlineCount int64, err error) {
	// normal: value_status = 1
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM device_points WHERE point_status <> 3 AND value_status = 1`,
	).Scan(&normalCount); err != nil {
		return
	}

	// abnormal: value_status IN (2,3,4,5) — low, high, open, closed
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM device_points WHERE point_status <> 3 AND value_status IN (2,3,4,5)`,
	).Scan(&abnormalCount); err != nil {
		return
	}

	// offline: value_status = 6
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM device_points WHERE point_status <> 3 AND value_status = 6`,
	).Scan(&offlineCount); err != nil {
		return
	}

	return
}

// LatestReportTime 获取 device_points.last_report_at_ms 最大值。
func (r *Repository) LatestReportTime(ctx context.Context) (*time.Time, error) {
	var t sql.NullTime
	if err := r.db.QueryRowContext(ctx,
		`SELECT MAX(last_report_at_ms) FROM device_points WHERE point_status <> 3`,
	).Scan(&t); err != nil {
		return nil, err
	}
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}

// CountIngest 统计数据上报（按 received_at_ms / sampled_at_ms 过滤 rangeCutoff）。
// 返回 raw_event_count, telemetry_count, register_count, heartbeat_count,
//
//	work_count, control_result_count。
func (r *Repository) CountIngest(ctx context.Context, rangeCutoff time.Time) (rawEventCount int64, telemetryCount int64, registerCount int64, heartbeatCount int64, workCount int64, controlResultCount int64, err error) {
	// Read the raw-event range once instead of scanning it separately for each type.
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*),
 COALESCE(SUM(event_type = 1), 0), COALESCE(SUM(event_type = 2), 0),
 COALESCE(SUM(event_type = 3), 0), COALESCE(SUM(event_type = 4), 0)
 FROM raw_device_events WHERE received_at_ms >= ?`, rangeCutoff).Scan(
		&rawEventCount, &registerCount, &heartbeatCount, &workCount, &controlResultCount)
	if err != nil {
		return
	}
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sensor_data WHERE sampled_at_ms >= ?`, rangeCutoff).Scan(&telemetryCount)
	return
}

// CountHeartbeatStats 统计心跳成功率。
// TODO: 当前以 parse_status=2（已解析）视为成功，后续 telemetry 完善后改为真实成功率。
// 返回 heartbeat_total, heartbeat_success, heartbeat_failed。
func (r *Repository) CountHeartbeatStats(ctx context.Context, rangeCutoff time.Time) (heartbeatTotal int64, heartbeatSuccess int64, heartbeatFailed int64, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*),
 COALESCE(SUM(parse_status = 2), 0), COALESCE(SUM(parse_status = 3), 0)
 FROM raw_device_events WHERE received_at_ms >= ? AND event_type = 2`, rangeCutoff).Scan(
		&heartbeatTotal, &heartbeatSuccess, &heartbeatFailed)
	return
}

// CountControlStats 统计控制命令（按 requested_at_ms 过滤 rangeCutoff）。
// 返回 control_command_count, control_success_count, control_failed_count, control_timeout_count。
func (r *Repository) CountControlStats(ctx context.Context, rangeCutoff time.Time) (commandCount int64, successCount int64, failedCount int64, timeoutCount int64, err error) {
	// 控制命令总数
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM control_commands WHERE requested_at_ms >= ?`, rangeCutoff,
	).Scan(&commandCount); err != nil {
		return
	}

	// command_status=4: succeeded
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM control_commands WHERE requested_at_ms >= ? AND command_status = 4`, rangeCutoff,
	).Scan(&successCount); err != nil {
		return
	}

	// command_status=5: failed
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM control_commands WHERE requested_at_ms >= ? AND command_status = 5`, rangeCutoff,
	).Scan(&failedCount); err != nil {
		return
	}

	// command_status=6: timeout
	if err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM control_commands WHERE requested_at_ms >= ? AND command_status = 6`, rangeCutoff,
	).Scan(&timeoutCount); err != nil {
		return
	}

	return
}
