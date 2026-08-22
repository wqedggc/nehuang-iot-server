package mysql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"log"
	"strconv"
	"strings"
	"time"

	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/idgen"
)

// ─── 原始设备事件 ───

// InsertRawEvent 写入原始设备事件到 raw_device_events 表。
// 返回插入后的 raw_event_id。
func (r *Repository) InsertRawEvent(ctx context.Context, event *domain.RawDeviceEvent) (uint64, error) {
	if event.RawEventID == 0 {
		event.RawEventID = idgen.NextID()
	}
	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now
	if event.ReceivedAtMS.IsZero() {
		event.ReceivedAtMS = now
	}
	if event.ParseStatus == 0 {
		event.ParseStatus = domain.ParseStatusPending
	}
	if event.SourceType == 0 {
		event.SourceType = domain.SourceTypeAdapter
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO raw_device_events
  (raw_event_id, gateway_id, node_id, point_id,
   gateway_name, event_type, source_type,
   remote_ip, remote_port, payload_hex, payload_snapshot,
   parse_status, parse_error, message_id, event_count, parsed_count,
   received_at_ms, created_at, updated_at)
VALUES
  (?, ?, ?, ?,
   ?, ?, ?,
   ?, ?, ?, ?,
   ?, ?, ?, ?, ?,
   ?, ?, ?)`,
		event.RawEventID,
		nullableUint64(event.GatewayID),
		nullableUint64(event.NodeID),
		nullableUint64(event.PointID),
		nullableString(event.GatewayName),
		uint8(event.EventType),
		uint8(event.SourceType),
		nullableString(event.RemoteIP),
		nullableInt(event.RemotePort),
		nullableString(event.PayloadHex),
		nullableString(event.PayloadSnapshot),
		uint8(event.ParseStatus),
		nullableString(event.ParseError),
		nullableString(event.MessageID),
		event.EventCount,
		event.ParsedCount,
		event.ReceivedAtMS,
		event.CreatedAt,
		event.UpdatedAt,
	)
	if err != nil {
		return 0, err
	}
	return event.RawEventID, nil
}

// UpdateRawEventParseStatus 更新原始事件的解析状态。
func (r *Repository) UpdateRawEventParseStatus(ctx context.Context, rawEventID uint64, status domain.ParseStatus, errMsg *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE raw_device_events SET parse_status = ?, parse_error = ? WHERE raw_event_id = ?`,
		uint8(status), nullableString(errMsg), rawEventID,
	)
	return err
}

// UpdateRawEventParseMeta 回填解析元数据（event_type / event_count / parsed_count / message_id）。
func (r *Repository) UpdateRawEventParseMeta(ctx context.Context, rawEventID uint64, eventType domain.RawEventType, eventCount int, parsedCount int, messageID *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE raw_device_events SET event_type = ?, event_count = ?, parsed_count = ?, message_id = ? WHERE raw_event_id = ?`,
		uint8(eventType), eventCount, parsedCount, nullableString(messageID), rawEventID,
	)
	return err
}

// ListRawEvents 返回最近的原始设备事件列表。
func (r *Repository) ListRawEvents(ctx context.Context, limit int) ([]domain.RawDeviceEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  raw_event_id, gateway_id, node_id, point_id,
  gateway_name, event_type, source_type,
  remote_ip, remote_port, payload_hex, payload_snapshot,
  parse_status, parse_error, message_id, event_count, parsed_count,
  received_at_ms, created_at, updated_at
FROM raw_device_events
ORDER BY received_at_ms DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]domain.RawDeviceEvent, 0)
	for rows.Next() {
		var e domain.RawDeviceEvent
		if err := rows.Scan(
			&e.RawEventID,
			scanNullUint64(&e.GatewayID),
			scanNullUint64(&e.NodeID),
			scanNullUint64(&e.PointID),
			scanNullString(&e.GatewayName),
			&e.EventType,
			&e.SourceType,
			scanNullString(&e.RemoteIP),
			scanNullInt(&e.RemotePort),
			scanNullString(&e.PayloadHex),
			scanNullString(&e.PayloadSnapshot),
			&e.ParseStatus,
			scanNullString(&e.ParseError),
			scanNullString(&e.MessageID),
			&e.EventCount,
			&e.ParsedCount,
			&e.ReceivedAtMS,
			&e.CreatedAt,
			&e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ListPayloadEvents 分页查询 payload 事件列表，支持多条件筛选。
func (r *Repository) ListPayloadEvents(ctx context.Context, params domain.PayloadEventQuery) ([]domain.PayloadEventListItem, int, error) {
	// 构建 WHERE 条件
	var conditions []string
	var args []interface{}

	if params.SourceType != nil {
		conditions = append(conditions, "source_type = ?")
		args = append(args, *params.SourceType)
	}
	if params.EventType != nil {
		conditions = append(conditions, "event_type = ?")
		args = append(args, *params.EventType)
	}
	if params.ParseStatus != nil {
		conditions = append(conditions, "parse_status = ?")
		args = append(args, *params.ParseStatus)
	}
	if params.Start != nil {
		conditions = append(conditions, "received_at_ms >= ?")
		args = append(args, *params.Start)
	}
	if params.End != nil {
		conditions = append(conditions, "received_at_ms <= ?")
		args = append(args, *params.End)
	}
	if params.Keyword != nil && *params.Keyword != "" {
		conditions = append(conditions, "(gateway_name LIKE ? OR message_id LIKE ? OR payload_hex LIKE ? OR payload_hex LIKE ?)")
		keyword := strings.TrimSpace(*params.Keyword)
		kw := "%" + keyword + "%"
		hexKw := "%" + normalizePayloadHexKeyword(keyword) + "%"
		args = append(args, kw, kw, kw, hexKw)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	// 查询总数
	var total int
	countQuery := "SELECT COUNT(*) FROM raw_device_events " + whereClause
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 查询列表
	offset := (params.Page - 1) * params.Size
	listQuery := `SELECT raw_event_id, COALESCE(gateway_name,''), source_type, event_type, parse_status,
		COALESCE(parse_error,''), COALESCE(message_id,''), event_count, parsed_count,
		COALESCE(remote_ip,''), COALESCE(remote_port,0), received_at_ms,
		COALESCE(payload_hex,''), COALESCE(payload_snapshot,'')
	FROM raw_device_events ` + whereClause +
		` ORDER BY received_at_ms DESC LIMIT ? OFFSET ?`
	listArgs := append(args, params.Size, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.PayloadEventListItem, 0, params.Size)
	for rows.Next() {
		var item domain.PayloadEventListItem
		var payloadHex string
		var payloadSnapshot string
		var receivedAt time.Time
		if err := rows.Scan(
			&item.RawEventID, &item.GatewayName, &item.SourceType, &item.EventType, &item.ParseStatus,
			&item.ParseError, &item.MessageID, &item.EventCount, &item.ParsedCount,
			&item.RemoteIP, &item.RemotePort, &receivedAt, &payloadHex, &payloadSnapshot,
		); err != nil {
			return nil, 0, err
		}
		item.SourceTypeName = domain.SourceType(item.SourceType).SourceTypeName()
		item.IsMock = isMockPayloadEvent(item.SourceType, item.MessageID, payloadSnapshot)
		item.EventTypeName = domain.RawEventType(item.EventType).RawEventTypeName()
		item.ParseStatusName = domain.ParseStatus(item.ParseStatus).ParseStatusName()
		item.ReceivedAtMS = receivedAt.Format("2006-01-02 15:04:05")
		item.PayloadLen = hexPayloadByteLen(payloadHex)
		item.PayloadHex = payloadHex
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func normalizePayloadHexKeyword(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range keyword {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'a' && r <= 'f':
			b.WriteRune(r - 'a' + 'A')
		case r >= 'A' && r <= 'F':
			b.WriteRune(r)
		case r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '-' || r == ':' || r == '_':
			continue
		default:
			return keyword
		}
	}
	if b.Len() == 0 {
		return keyword
	}
	return b.String()
}

// GetPayloadEventDetail 返回单条 payload 事件的完整详情。
func (r *Repository) GetPayloadEventDetail(ctx context.Context, rawEventID uint64) (*domain.PayloadEventDetail, error) {
	var d domain.PayloadEventDetail
	var gwID, nodeID, pointID sql.NullInt64
	var payloadHex sql.NullString
	var payloadSnapshot sql.NullString
	var receivedAt, createdAt time.Time

	err := r.db.QueryRowContext(ctx, `
SELECT raw_event_id, gateway_id, node_id, point_id,
  COALESCE(gateway_name,''), source_type, event_type, parse_status,
  COALESCE(parse_error,''), COALESCE(message_id,''), event_count, parsed_count,
  COALESCE(remote_ip,''), COALESCE(remote_port,0), payload_hex, payload_snapshot,
  received_at_ms, created_at
FROM raw_device_events WHERE raw_event_id = ?`, rawEventID,
	).Scan(&d.RawEventID, &gwID, &nodeID, &pointID,
		&d.GatewayName, &d.SourceType, &d.EventType, &d.ParseStatus,
		&d.ParseError, &d.MessageID, &d.EventCount, &d.ParsedCount,
		&d.RemoteIP, &d.RemotePort, &payloadHex, &payloadSnapshot,
		&receivedAt, &createdAt)
	if err != nil {
		return nil, err
	}

	if gwID.Valid {
		id := uint64(gwID.Int64)
		d.GatewayID = &id
	}
	if nodeID.Valid {
		id := uint64(nodeID.Int64)
		d.NodeID = &id
	}
	if pointID.Valid {
		id := uint64(pointID.Int64)
		d.PointID = &id
	}
	if payloadSnapshot.Valid {
		d.PayloadSnapshot = payloadSnapshot.String
	}
	if payloadHex.Valid {
		d.PayloadHex = payloadHex.String
		d.PayloadByteLen = hexPayloadByteLen(payloadHex.String)
	}
	d.EventTypeName = domain.RawEventType(d.EventType).RawEventTypeName()
	d.SourceTypeName = domain.SourceType(d.SourceType).SourceTypeName()
	d.IsMock = isMockPayloadEvent(d.SourceType, d.MessageID, d.PayloadSnapshot)
	d.ParseStatusName = domain.ParseStatus(d.ParseStatus).ParseStatusName()
	d.ReceivedAtMS = receivedAt.Format("2006-01-02 15:04:05")
	d.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	return &d, nil
}

func hexPayloadByteLen(payloadHex string) int {
	payloadHex = strings.TrimSpace(payloadHex)
	if payloadHex == "" {
		return 0
	}
	decoded, err := hex.DecodeString(payloadHex)
	if err != nil {
		return 0
	}
	return len(decoded)
}

func isMockPayloadEvent(sourceType uint8, messageID string, payloadSnapshot string) bool {
	if domain.SourceType(sourceType) == domain.SourceTypeMock {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(messageID))
	if strings.HasPrefix(msg, "mock-") {
		return true
	}
	payload := strings.ToLower(payloadSnapshot)
	return strings.Contains(payload, `"source_type": "mock_script"`) ||
		strings.Contains(payload, `"source_type":"mock_script"`)
}

// ─── 云边设备 ───

// FindGatewayByExternalID 按 gateway_external_id 查找云边设备，不存在返回 nil。
func (r *Repository) FindGatewayByExternalID(ctx context.Context, externalID string) (*domain.Gateway, error) {
	var g domain.Gateway
	err := r.db.QueryRowContext(ctx, `
SELECT
  gateway_id, gateway_external_id, gateway_name,
  access_mode, ip, port, gateway_sn, online_status, source_type,
  last_heartbeat_at_ms, created_at, updated_at
FROM cloud_gateways
WHERE gateway_external_id = ?
LIMIT 1`, externalID).Scan(
		&g.GatewayID, &g.GatewayExternalID, &g.GatewayName,
		&g.AccessMode, &g.IP, &g.Port, &g.GatewaySN, &g.OnlineStatus, &g.SourceType,
		&g.LastHeartbeatAtMS, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// UpsertGatewayByExternalID 按 gateway_external_id 创建或更新云边设备。
// 如果设备已存在则更新在线状态和心跳时间，否则创建新记录。
// 返回 gateway_id。
func (r *Repository) UpsertGatewayByExternalID(ctx context.Context, gw *domain.Gateway) (uint64, error) {
	// 先查找是否已存在。
	existing, err := r.FindGatewayByExternalID(ctx, gw.GatewayExternalID)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	if existing != nil {
		// 已存在：更新在线状态和心跳时间。
		_, err := r.db.ExecContext(ctx,
			`UPDATE cloud_gateways
			 SET online_status = ?, ip = ?, port = ?,
			     last_heartbeat_at_ms = ?, source_type = ?,
			     updated_at = ?
			 WHERE gateway_id = ?`,
			1, gw.IP, gw.Port, now, uint8(gw.SourceType), now, existing.GatewayID,
		)
		return existing.GatewayID, err
	}

	// 不存在：创建新记录。
	gwID := idgen.NextID()
	_, err = r.db.ExecContext(ctx, `
INSERT INTO cloud_gateways
  (gateway_id, gateway_external_id, gateway_name,
   access_mode, ip, port, gateway_sn, online_status, source_type,
   last_heartbeat_at_ms, first_seen_at_ms, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		gwID, gw.GatewayExternalID, gw.GatewayName,
		string(domain.GatewayAccessModeShared), gw.IP, gw.Port, "", 1, uint8(gw.SourceType),
		now, now, now, now,
	)
	if err != nil {
		return 0, err
	}
	return gwID, nil
}

// ─── 采集执行器节点 ───

// UpsertCollectorNode 按 gateway_id + frequency + device_external_id 创建或更新采集执行器节点。
// 如果节点已存在则更新在线状态和上报时间，否则创建新记录。
// 返回 node_id。
func (r *Repository) UpsertCollectorNode(ctx context.Context, node *domain.CollectorNode) (uint64, error) {
	// 按唯一身份查找。
	var existingID uint64
	err := r.db.QueryRowContext(ctx,
		`SELECT node_id FROM collector_nodes
		 WHERE gateway_id = ? AND frequency = ? AND device_external_id = ?
		 LIMIT 1`,
		node.GatewayID, node.Frequency, node.DeviceExternalID,
	).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	now := time.Now()
	if err == nil {
		// 已存在：更新在线状态和上报时间。
		_, err := r.db.ExecContext(ctx,
			`UPDATE collector_nodes
			 SET online_status = ?, last_report_at_ms = ?, updated_at = ?
			 WHERE node_id = ?`,
			1, now, now, existingID,
		)
		return existingID, err
	}

	// 不存在：创建新记录。
	nodeID := idgen.NextID()
	_, err = r.db.ExecContext(ctx, `
INSERT INTO collector_nodes
  (node_id, gateway_id, frequency, device_external_id,
   node_name, shed_no, base_name, area_name,
   node_status, online_status, last_report_at_ms,
   created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nodeID, node.GatewayID, node.Frequency, node.DeviceExternalID,
		"", "", "", "",
		1, 1, now, now, now,
	)
	if err != nil {
		return 0, err
	}
	return nodeID, nil
}

// ListCollectors 返回所有采集执行器节点列表。
func (r *Repository) ListCollectors(ctx context.Context) ([]domain.CollectorNode, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  node_id, gateway_id, frequency, device_external_id,
  node_name, shed_no, base_name, area_name,
  collector_no, node_status, online_status,
  last_report_at_ms, last_raw_event_id,
  remark, extra, created_at, updated_at
FROM collector_nodes
ORDER BY updated_at DESC
LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]domain.CollectorNode, 0)
	for rows.Next() {
		var n domain.CollectorNode
		if err := rows.Scan(
			&n.NodeID, &n.GatewayID, &n.Frequency, &n.DeviceExternalID,
			&n.NodeName, &n.ShedNo, &n.BaseName, &n.AreaName,
			scanNullInt(&n.CollectorNo), &n.NodeStatus, &n.OnlineStatus,
			scanNullTime(&n.LastReportAtMS), scanNullUint64(&n.LastRawEventID),
			scanNullString(&n.Remark), scanNullString(&n.Extra),
			&n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// ─── 功能点 ───

// UpsertDevicePoint 按 node_id + function_type + channel_no 创建或更新功能点。
// 如果功能点已存在则更新，否则创建新记录。
// 返回 point_id。
func (r *Repository) UpsertDevicePoint(ctx context.Context, point *domain.DevicePoint) (uint64, error) {
	// 按 node_id + function_type + channel_no 查找。
	var existingID uint64
	err := r.db.QueryRowContext(ctx,
		`SELECT point_id FROM device_points
		 WHERE node_id = ? AND function_type = ? AND channel_no = ?
		 LIMIT 1`,
		point.NodeID, uint8(point.FunctionType), point.ChannelNo,
	).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	now := time.Now()
	if err == nil {
		// 已存在：更新实时值和单位。
		_, err := r.db.ExecContext(ctx,
			`UPDATE device_points
			 SET real_value = ?, real_temp = ?, value_status = ?,
			     unit = ?, last_report_at_ms = ?, updated_at = ?
			 WHERE point_id = ?`,
			nullableFloat64(point.RealValue), point.RealTemp, uint8(point.ValueStatus),
			point.Unit, now, now, existingID,
		)
		return existingID, err
	}

	// 不存在：创建新记录。
	pointID := idgen.NextID()
	_, err = r.db.ExecContext(ctx, `
INSERT INTO device_points
  (point_id, node_id, gateway_id, frequency, device_external_id,
   function_type, channel_no, point_name, unit,
   best_min, best_max, real_value, real_temp,
   value_status, run_mode, controllable_status,
   rel1_status, rel2_status, operation_state,
   poll_interval, point_status, last_report_at_ms,
   created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pointID, point.NodeID, point.GatewayID, point.Frequency, point.DeviceExternalID,
		uint8(point.FunctionType), point.ChannelNo, "", point.Unit,
		nullableFloat64(point.BestMin), nullableFloat64(point.BestMax),
		nullableFloat64(point.RealValue), point.RealTemp,
		uint8(point.ValueStatus), uint8(RunModeAuto), 0,
		0, 0, uint8(OpStateUnknown),
		300, 1, now,
		now, now,
	)
	if err != nil {
		return 0, err
	}
	return pointID, nil
}

// SeedGatewayDevicePoints creates or updates one collector node and its test
// points under a gateway. It is an admin 联调 helper used before the hardware
// work-data parser can create the same structure from real packets.
func (r *Repository) SeedGatewayDevicePoints(ctx context.Context, gatewayID uint64, input domain.GatewayDeviceSeedInput) (*domain.GatewayDeviceSeedResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var exists uint64
	if err := tx.QueryRowContext(ctx, `SELECT gateway_id FROM cloud_gateways WHERE gateway_id = ? FOR UPDATE`, gatewayID).Scan(&exists); err != nil {
		return nil, err
	}

	now := time.Now()
	var nodeID uint64
	err = tx.QueryRowContext(ctx, `
SELECT node_id
FROM collector_nodes
WHERE gateway_id = ? AND frequency = ? AND device_external_id = ?
LIMIT 1 FOR UPDATE`, gatewayID, input.Frequency, input.DeviceExternalID).Scan(&nodeID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == sql.ErrNoRows {
		nodeID = idgen.NextID()
		_, err = tx.ExecContext(ctx, `
INSERT INTO collector_nodes
  (node_id, gateway_id, frequency, device_external_id,
   node_name, shed_no, base_name, area_name,
   node_status, online_status, last_report_at_ms,
   remark, extra, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, '', '', 1, 0, NULL, ?, ?, ?, ?)`,
			nodeID, gatewayID, input.Frequency, input.DeviceExternalID,
			input.NodeName, input.ShedNo, nullableString(emptyToNil(input.Remark)), nullableString(strPtr(`{"source":"admin_seed"}`)),
			now, now,
		)
		if err != nil {
			return nil, err
		}
	} else {
		_, err = tx.ExecContext(ctx, `
UPDATE collector_nodes
SET node_name = ?, shed_no = ?, remark = ?, extra = ?, updated_at = ?
WHERE node_id = ?`,
			input.NodeName, input.ShedNo, nullableString(emptyToNil(input.Remark)), nullableString(strPtr(`{"source":"admin_seed"}`)),
			now, nodeID,
		)
		if err != nil {
			return nil, err
		}
	}

	result := &domain.GatewayDeviceSeedResult{
		GatewayID: gatewayID,
		NodeID:    nodeID,
		PointIDs:  make([]string, 0, len(input.Points)),
	}
	for _, p := range input.Points {
		var pointID uint64
		err = tx.QueryRowContext(ctx, `
SELECT point_id
FROM device_points
WHERE node_id = ? AND function_type = ? AND channel_no = ?
LIMIT 1 FOR UPDATE`, nodeID, uint8(p.FunctionType), p.ChannelNo).Scan(&pointID)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		realTemp := 0.0
		if p.FunctionType == domain.FuncTypeTemperature && p.RealValue != nil {
			realTemp = *p.RealValue
		}
		pollCmd := emptyToNil(p.PollCmd)
		remark := emptyToNil(p.Remark)
		extra := strPtr(`{"source":"admin_seed"}`)
		if err == sql.ErrNoRows {
			pointID = idgen.NextID()
			_, err = tx.ExecContext(ctx, `
INSERT INTO device_points
  (point_id, node_id, gateway_id, frequency, device_external_id,
   function_type, channel_no, point_name, unit,
   best_min, best_max, real_value, real_temp,
   value_status, run_mode, controllable_status,
   rel1_status, rel2_status, operation_state,
   poll_interval, poll_cmd, point_status, last_report_at_ms,
   remark, extra, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?, ?, NULL, ?, ?, ?, ?)`,
				pointID, nodeID, gatewayID, input.Frequency, input.DeviceExternalID,
				uint8(p.FunctionType), p.ChannelNo, p.PointName, p.Unit,
				nullableFloat64(p.BestMin), nullableFloat64(p.BestMax), nullableFloat64(p.RealValue), realTemp,
				uint8(p.ValueStatus), uint8(p.RunMode), p.ControllableStatus,
				uint8(domain.OpStateUnknown), p.PollInterval, nullableString(pollCmd), p.PointStatus,
				nullableString(remark), nullableString(extra), now, now,
			)
			if err != nil {
				return nil, err
			}
			result.CreatedPoints++
		} else {
			_, err = tx.ExecContext(ctx, `
UPDATE device_points
SET point_name = ?, unit = ?, best_min = ?, best_max = ?,
    real_value = ?, real_temp = ?, value_status = ?,
    run_mode = ?, controllable_status = ?, operation_state = ?,
    poll_interval = ?, poll_cmd = ?, point_status = ?,
    remark = ?, extra = ?, updated_at = ?
WHERE point_id = ?`,
				p.PointName, p.Unit, nullableFloat64(p.BestMin), nullableFloat64(p.BestMax),
				nullableFloat64(p.RealValue), realTemp, uint8(p.ValueStatus),
				uint8(p.RunMode), p.ControllableStatus, uint8(domain.OpStateUnknown),
				p.PollInterval, nullableString(pollCmd), p.PointStatus,
				nullableString(remark), nullableString(extra), now, pointID,
			)
			if err != nil {
				return nil, err
			}
			result.UpdatedPoints++
		}
		result.PointIDs = append(result.PointIDs, strconv.FormatUint(pointID, 10))
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateDevicePointRealtime 更新功能点实时状态。
func (r *Repository) UpdateDevicePointRealtime(ctx context.Context, pointID uint64, realValue *float64, valueStatus domain.ValueStatus, lastReportAt interface{}) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE device_points
		 SET real_value = ?, real_temp = ?, value_status = ?,
		     last_report_at_ms = ?, updated_at = ?
		 WHERE point_id = ?`,
		nullableFloat64(realValue), float64Value(realValue), uint8(valueStatus),
		now, now, pointID,
	)
	return err
}

// GetMiniDeviceDetail 获取单个功能点详情（C 端实时数据）。
func (r *Repository) GetMiniDeviceDetail(ctx context.Context, pointID uint64) (*domain.MiniDeviceDetail, error) {
	var d domain.MiniDeviceDetail
	var bestMin, bestMax sql.NullFloat64
	var realValue sql.NullFloat64

	err := r.db.QueryRowContext(ctx, `
SELECT
  dp.point_id, dp.node_id, cn.node_name, cg.gateway_name, cg.access_mode, cn.shed_no,
  cn.frequency, cn.device_external_id,
  dp.function_type, dp.point_name, dp.unit,
  dp.best_min, dp.best_max,
  dp.real_value, dp.real_temp,
  dp.value_status, dp.run_mode,
  dp.rel1_status, dp.rel2_status,
  dp.operation_state, dp.poll_interval,
  dp.updated_at
FROM device_points dp
JOIN collector_nodes cn ON dp.node_id = cn.node_id
JOIN cloud_gateways cg ON dp.gateway_id = cg.gateway_id
WHERE dp.point_id = ?
LIMIT 1`, pointID).Scan(
		&d.PointID, &d.NodeID, &d.NodeName, &d.GatewayName, &d.GatewayAccessMode, &d.ShedNo,
		&d.Frequency, &d.DeviceExternalID,
		&d.FunctionType, &d.FunctionName, &d.Unit,
		&bestMin, &bestMax,
		&realValue, &d.RealTemp,
		&d.ValueStatus, &d.RunMode,
		&d.Rel1Status, &d.Rel2Status,
		&d.OperationState, &d.PollInterval,
		&d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if bestMin.Valid {
		d.BestMin = &bestMin.Float64
	}
	if bestMax.Valid {
		d.BestMax = &bestMax.Float64
	}
	if realValue.Valid {
		d.RealValue = &realValue.Float64
	}

	// 补全显示标签。
	if strings.TrimSpace(d.FunctionName) == "" {
		d.FunctionName = domain.FunctionType(d.FunctionType).FunctionTypeName()
	}
	d.ValueStatusLabel = valueStatusLabel(d.ValueStatus)
	d.RunModeLabel = runModeLabel(d.RunMode)
	d.GatewayAccessMode = string(domain.NormalizeGatewayAccessMode(d.GatewayAccessMode))
	d.GatewayAccessModeLabel = domain.GatewayAccessMode(d.GatewayAccessMode).Label()

	return &d, nil
}

// ListMiniDevices 获取用户绑定的功能点列表（C 端设备列表）。
func (r *Repository) ListMiniDevices(ctx context.Context, userID uint64) ([]domain.MiniDevice, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  dp.point_id, dp.node_id, cn.node_name, dp.gateway_id, cg.gateway_name, cg.access_mode,
  cn.shed_no, cn.frequency, cn.device_external_id,
  dp.function_type, dp.point_name,
  dp.real_value, dp.unit,
  dp.value_status, dp.run_mode, dp.updated_at
FROM device_points dp
JOIN cloud_gateways cg ON dp.gateway_id = cg.gateway_id
JOIN collector_nodes cn ON dp.node_id = cn.node_id
WHERE dp.point_status = 1
  AND (
    EXISTS (
      SELECT 1 FROM bind_relations br
      WHERE br.user_id = ? AND br.point_id = dp.point_id AND br.bind_status = 1
    )
    OR EXISTS (
      SELECT 1 FROM bind_relations br
      WHERE br.user_id = ? AND br.node_id = dp.node_id AND br.point_id IS NULL AND br.bind_status = 1
    )
    OR EXISTS (
      SELECT 1 FROM bind_relations br
      WHERE br.user_id = ? AND br.gateway_id = dp.gateway_id
        AND br.node_id IS NULL AND br.point_id IS NULL
        AND br.bind_status = 1 AND cg.access_mode = 'exclusive'
    )
  )
ORDER BY cn.node_id ASC, dp.updated_at DESC`, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := make([]domain.MiniDevice, 0)
	for rows.Next() {
		var d domain.MiniDevice
		var realValue sql.NullFloat64
		var updatedAt time.Time
		if err := rows.Scan(
			&d.PointID, &d.NodeID, &d.NodeName, &d.GatewayID, &d.GatewayName, &d.GatewayAccessMode,
			&d.ShedNo, &d.Frequency, &d.DeviceExternalID,
			&d.FunctionType, &d.FunctionName,
			&realValue, &d.Unit,
			&d.ValueStatus, &d.RunMode, &updatedAt,
		); err != nil {
			return nil, err
		}
		if realValue.Valid {
			d.RealValue = &realValue.Float64
		}
		if strings.TrimSpace(d.FunctionName) == "" {
			d.FunctionName = domain.FunctionType(d.FunctionType).FunctionTypeName()
		}
		d.UpdatedAt = updatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
		d.ValueStatusLabel = valueStatusLabel(d.ValueStatus)
		d.RunModeLabel = runModeLabel(d.RunMode)
		d.GatewayAccessMode = string(domain.NormalizeGatewayAccessMode(d.GatewayAccessMode))
		d.GatewayAccessModeLabel = domain.GatewayAccessMode(d.GatewayAccessMode).Label()
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

const checkBindRelationQuery = `SELECT COUNT(*)
 FROM device_points dp
 WHERE dp.point_id = ?
   AND (
     EXISTS (
       SELECT 1 FROM bind_relations br
       WHERE br.user_id = ? AND br.point_id = dp.point_id AND br.bind_status = 1
     )
     OR EXISTS (
       SELECT 1 FROM bind_relations br
       WHERE br.user_id = ? AND br.node_id = dp.node_id AND br.point_id IS NULL AND br.bind_status = 1
     )
     OR EXISTS (
       SELECT 1 FROM bind_relations br
       JOIN cloud_gateways cg ON cg.gateway_id = dp.gateway_id
       WHERE br.user_id = ? AND br.gateway_id = dp.gateway_id
         AND br.node_id IS NULL AND br.point_id IS NULL
         AND br.bind_status = 1 AND cg.access_mode = 'exclusive'
     )
   )`

// CheckBindRelation 检查用户是否对某个功能点有 active 绑定关系。
func (r *Repository) CheckBindRelation(ctx context.Context, userID uint64, pointID uint64) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, checkBindRelationQuery, pointID, userID, userID, userID).Scan(&count)
	return count > 0, err
}

// GetMiniDeviceHistory 获取功能点历史曲线数据。
func (r *Repository) GetMiniDeviceHistory(ctx context.Context, pointID uint64, rangeType string) ([]domain.MiniHistoryPoint, error) {
	// 根据 range 计算时间范围。
	var since time.Time
	switch rangeType {
	case "1h":
		since = time.Now().Add(-1 * time.Hour)
	case "6h":
		since = time.Now().Add(-6 * time.Hour)
	case "1d", "24h":
		since = time.Now().Add(-24 * time.Hour)
	case "3d":
		since = time.Now().Add(-3 * 24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		since = time.Now().Add(-30 * 24 * time.Hour)
	default:
		since = time.Now().Add(-24 * time.Hour)
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT sampled_at_ms, real_temp
FROM sensor_data
WHERE point_id = ? AND sampled_at_ms >= ?
ORDER BY sampled_at_ms ASC
LIMIT 500`, pointID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]domain.MiniHistoryPoint, 0)
	for rows.Next() {
		var p domain.MiniHistoryPoint
		var t time.Time
		if err := rows.Scan(&t, &p.RealTemp); err != nil {
			return nil, err
		}
		p.Time = t.UTC().Format("2006-01-02T15:04:05.000Z")
		points = append(points, p)
	}
	return points, rows.Err()
}

// InsertSensorData 插入传感器时序数据。
func (r *Repository) InsertSensorData(ctx context.Context, data *domain.SensorData) error {
	if data.SensorID == 0 {
		data.SensorID = idgen.NextID()
	}
	now := time.Now()
	data.CreatedAt = now
	data.UpdatedAt = now
	if data.SampledAtMS.IsZero() {
		data.SampledAtMS = now
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO sensor_data
  (sensor_id, gateway_id, realtime_id, node_id, point_id,
   frequency, device_external_id, sampled_at_ms,
   real_value, real_temp, function_type, unit,
   poll_interval, raw_event_id, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		data.SensorID, data.GatewayID,
		nullableUint64(data.RealtimeID), nullableUint64(data.NodeID), nullableUint64(data.PointID),
		data.Frequency, data.DeviceExternalID, data.SampledAtMS,
		nullableFloat64(data.RealValue), data.RealTemp, uint8(data.FunctionType), data.Unit,
		data.PollInterval, nullableUint64(data.RawEventID),
		data.CreatedAt, data.UpdatedAt,
	)
	return err
}

// ─── device_realtime_maps 兼容投影 ───

// UpsertDeviceRealtimeMap 同步更新 device_realtime_maps 兼容投影表。
// 新代码优先以 device_points 为实时状态主表，此表仅做兼容。
func (r *Repository) UpsertDeviceRealtimeMap(ctx context.Context, point *domain.DevicePoint) error {
	projection := newDeviceRealtimeMapProjection(point)

	// 按 gateway_id + frequency + device_external_id + function_type + channel_no 查找。
	var existingID uint64
	err := r.db.QueryRowContext(ctx,
		`SELECT realtime_id FROM device_realtime_maps
		 WHERE gateway_id = ? AND frequency = ? AND device_external_id = ? AND function_type = ? AND channel_no = ?
		 LIMIT 1`,
		projection.GatewayID, projection.Frequency, projection.DeviceExternalID, projection.FunctionType, projection.ChannelNo,
	).Scan(&existingID)

	now := time.Now()
	if err == nil {
		// 已存在：更新。
		_, err = r.db.ExecContext(ctx,
			`UPDATE device_realtime_maps
			 SET real_value = ?, real_temp = ?, value_status = ?,
			     node_id = ?, point_id = ?,
			     unit = ?, last_report_at_ms = ?, updated_at = ?
			 WHERE realtime_id = ?`,
			nullableFloat64(projection.RealValue), projection.RealTemp, projection.ValueStatus,
			projection.NodeID, projection.PointID,
			projection.Unit, now, now,
			existingID,
		)
		return err
	}

	if err != sql.ErrNoRows {
		return err
	}

	// 不存在：创建。
	// 只填充核心字段，其余用默认值。device_realtime_maps 是兼容投影表。
	rtID := idgen.NextID()
	_, err = r.db.ExecContext(ctx, `
INSERT INTO device_realtime_maps
  (realtime_id, gateway_id, node_id, point_id,
   frequency, device_external_id, function_type, channel_no,
   unit, real_value, real_temp, value_status,
   run_mode, poll_interval, last_report_at_ms,
   created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rtID, projection.GatewayID, projection.NodeID, projection.PointID,
		projection.Frequency, projection.DeviceExternalID, projection.FunctionType, projection.ChannelNo,
		projection.Unit,
		nullableFloat64(projection.RealValue), projection.RealTemp, projection.ValueStatus,
		uint8(RunModeAuto), 300, now,
		now, now,
	)
	return err
}

type deviceRealtimeMapProjection struct {
	GatewayID        uint64
	NodeID           uint64
	PointID          uint64
	Frequency        int
	DeviceExternalID string
	FunctionType     uint8
	ChannelNo        int
	Unit             string
	RealValue        *float64
	RealTemp         float64
	ValueStatus      uint8
}

func newDeviceRealtimeMapProjection(point *domain.DevicePoint) deviceRealtimeMapProjection {
	return deviceRealtimeMapProjection{
		GatewayID:        point.GatewayID,
		NodeID:           point.NodeID,
		PointID:          point.PointID,
		Frequency:        point.Frequency,
		DeviceExternalID: point.DeviceExternalID,
		FunctionType:     uint8(point.FunctionType),
		ChannelNo:        point.ChannelNo,
		Unit:             point.Unit,
		RealValue:        point.RealValue,
		RealTemp:         point.RealTemp,
		ValueStatus:      uint8(point.ValueStatus),
	}
}

// ─── 绑定关系 ───

type bindingRowScanner interface {
	Scan(dest ...any) error
}

func scanBindingRelationRow(row bindingRowScanner) (domain.BindRelation, error) {
	var b domain.BindRelation
	var (
		sUserID, sRealtimeID, sNodeID, sPointID, sCreatedBy sql.NullInt64
		sUnboundAtMS                                        sql.NullTime
		sExtra                                              sql.NullString
		sPointName, sCollectorShedNo, sCollectorNodeName    sql.NullString
		sCollectorNodeID                                    sql.NullInt64
		sChannelNo                                          sql.NullInt64
	)
	if err := row.Scan(
		&b.BindID,
		&sUserID,
		&b.BindUserName,
		&b.GatewayID,
		&b.GatewayName,
		&b.GatewayExternalID,
		&sRealtimeID,
		&sNodeID,
		&sPointID,
		&b.ShedNo, &b.Frequency, &b.DeviceExternalID, &b.FunctionType,
		&b.BindStatus, &b.BoundAtMS,
		&sUnboundAtMS,
		&sCreatedBy,
		&sExtra,
		&b.CreatedAt, &b.UpdatedAt,
		&sPointName, &sChannelNo, &sCollectorShedNo, &sCollectorNodeID, &sCollectorNodeName,
	); err != nil {
		return domain.BindRelation{}, err
	}
	if sUserID.Valid {
		uid := uint64(sUserID.Int64)
		b.UserID = &uid
	}
	if sRealtimeID.Valid {
		rid := uint64(sRealtimeID.Int64)
		b.RealtimeID = &rid
	}
	if sNodeID.Valid {
		nid := uint64(sNodeID.Int64)
		b.NodeID = &nid
	}
	if sPointID.Valid {
		pid := uint64(sPointID.Int64)
		b.PointID = &pid
		b.BindingType = string(domain.BindingTypePoint)
	} else if sNodeID.Valid {
		b.BindingType = string(domain.BindingTypeCollector)
	} else {
		b.BindingType = string(domain.BindingTypeGateway)
	}
	if sCreatedBy.Valid {
		cb := uint64(sCreatedBy.Int64)
		b.CreatedBy = &cb
	}
	if sUnboundAtMS.Valid {
		b.UnboundAtMS = &sUnboundAtMS.Time
	}
	if sExtra.Valid {
		b.Extra = &sExtra.String
	}
	if sNodeID.Valid {
		var collectorNodeID uint64
		if sCollectorNodeID.Valid {
			collectorNodeID = uint64(sCollectorNodeID.Int64)
		} else {
			collectorNodeID = uint64(sNodeID.Int64)
		}
		collector := &domain.BindingCollectorSummary{
			NodeID:           collectorNodeID,
			NodeName:         sCollectorNodeName.String,
			PointName:        sPointName.String,
			FunctionType:     b.FunctionType,
			FunctionName:     b.FunctionType.FunctionTypeName(),
			ChannelNo:        int(sChannelNo.Int64),
			ShedNo:           b.ShedNo,
			Frequency:        b.Frequency,
			DeviceExternalID: b.DeviceExternalID,
		}
		if sPointID.Valid {
			pointID := uint64(sPointID.Int64)
			collector.PointID = &pointID
		}
		if collector.ShedNo == "" && sCollectorShedNo.Valid {
			collector.ShedNo = sCollectorShedNo.String
		}
		b.Collector = collector
	}
	return b, nil
}

const bindingSelectColumns = `
SELECT
  br.bind_id, br.user_id, br.bind_user_name,
  br.gateway_id, cg.gateway_name, cg.gateway_external_id,
  br.realtime_id, br.node_id, br.point_id,
  br.shed_no, br.frequency, br.device_external_id, br.function_type,
  br.bind_status, br.bound_at_ms, br.unbound_at_ms, br.created_by,
  br.extra, br.created_at, br.updated_at,
  dp.point_name, dp.channel_no, cn.shed_no, cn.node_id, cn.node_name
FROM bind_relations br
JOIN cloud_gateways cg ON cg.gateway_id = br.gateway_id
LEFT JOIN device_points dp ON dp.point_id = br.point_id
LEFT JOIN collector_nodes cn ON cn.node_id = COALESCE(br.node_id, dp.node_id)`

// ListBindings 返回所有 active 绑定关系列表。
func (r *Repository) ListBindings(ctx context.Context) ([]domain.BindRelation, error) {
	rows, err := r.db.QueryContext(ctx, bindingSelectColumns+`
WHERE br.bind_status = 1
ORDER BY br.updated_at DESC
LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bindings := make([]domain.BindRelation, 0)
	for rows.Next() {
		b, err := scanBindingRelationRow(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, b)
	}
	return bindings, rows.Err()
}

// FindActiveBindingByUserAndPoint 检查是否已存在用户对某个功能点的 active 绑定。
// 返回已存在的绑定记录，不存在则返回 nil。
func (r *Repository) FindActiveBindingByUserAndPoint(ctx context.Context, userID uint64, pointID uint64) (*domain.BindRelation, error) {
	row := r.db.QueryRowContext(ctx, bindingSelectColumns+`
WHERE br.user_id = ? AND br.point_id = ? AND br.bind_status = 1
LIMIT 1`, userID, pointID)
	b, err := scanBindingRelationRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindActiveBindingByPoint 按 point_id 查询 active 绑定（不限制 user_id）。
// 用于冲突检测：检查 point 是否已被任意用户绑定。
// 返回已存在的绑定记录，不存在则返回 nil。
func (r *Repository) FindActiveBindingByPoint(ctx context.Context, pointID uint64) (*domain.BindRelation, error) {
	row := r.db.QueryRowContext(ctx, bindingSelectColumns+`
WHERE br.point_id = ? AND br.bind_status = 1
LIMIT 1`, pointID)
	b, err := scanBindingRelationRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindDevicePointByID 按 point_id 查询单个功能点。
func (r *Repository) FindDevicePointByID(ctx context.Context, pointID uint64) (*domain.DevicePoint, error) {
	var p domain.DevicePoint
	var bestMin, bestMax sql.NullFloat64
	var realValue sql.NullFloat64

	err := r.db.QueryRowContext(ctx, `
SELECT
  point_id, node_id, gateway_id, frequency, device_external_id,
  function_type, channel_no, point_name, unit,
  best_min, best_max, real_value, real_temp,
  value_status, run_mode, controllable_status,
  rel1_status, rel2_status, operation_state,
  poll_interval, point_status, last_report_at_ms,
  last_raw_event_id, remark, extra, created_at, updated_at
FROM device_points
WHERE point_id = ?
LIMIT 1`, pointID).Scan(
		&p.PointID, &p.NodeID, &p.GatewayID, &p.Frequency, &p.DeviceExternalID,
		&p.FunctionType, &p.ChannelNo, &p.PointName, &p.Unit,
		&bestMin, &bestMax, &realValue, &p.RealTemp,
		&p.ValueStatus, &p.RunMode, &p.ControllableStatus,
		&p.Rel1Status, &p.Rel2Status, &p.OperationState,
		&p.PollInterval, &p.PointStatus,
		scanNullTime(&p.LastReportAtMS),
		scanNullUint64(&p.LastRawEventID),
		scanNullString(&p.Remark), scanNullString(&p.Extra),
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if bestMin.Valid {
		p.BestMin = &bestMin.Float64
	}
	if bestMax.Valid {
		p.BestMax = &bestMax.Float64
	}
	if realValue.Valid {
		p.RealValue = &realValue.Float64
	}
	return &p, nil
}

// GetDeviceTree 返回设备树：gateway → collector → point 三层结构。
func (r *Repository) GetDeviceTree(ctx context.Context) ([]domain.DeviceTreeNode, error) {
	// 1. 查询所有 active gateways。
	gwRows, err := r.db.QueryContext(ctx, `
SELECT gateway_id, gateway_name, access_mode, online_status
FROM cloud_gateways
ORDER BY gateway_id ASC`)
	if err != nil {
		return nil, err
	}
	defer gwRows.Close()

	type gwRow struct {
		ID         uint64
		Name       string
		AccessMode string
		Status     uint8
	}
	var gateways []gwRow
	for gwRows.Next() {
		var g gwRow
		if err := gwRows.Scan(&g.ID, &g.Name, &g.AccessMode, &g.Status); err != nil {
			return nil, err
		}
		g.AccessMode = string(domain.NormalizeGatewayAccessMode(g.AccessMode))
		gateways = append(gateways, g)
	}
	if err := gwRows.Err(); err != nil {
		return nil, err
	}
	if len(gateways) == 0 {
		return []domain.DeviceTreeNode{}, nil
	}

	// 2. 查询所有 active collector_nodes。
	colRows, err := r.db.QueryContext(ctx, `
SELECT node_id, gateway_id, node_name, shed_no, frequency, device_external_id
FROM collector_nodes
WHERE node_status = 1
ORDER BY gateway_id ASC, node_id ASC`)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	type colRow struct {
		NodeID    uint64
		GatewayID uint64
		NodeName  string
		ShedNo    string
		Frequency int
		DevExtID  string
	}
	var collectors []colRow
	for colRows.Next() {
		var c colRow
		if err := colRows.Scan(&c.NodeID, &c.GatewayID, &c.NodeName, &c.ShedNo, &c.Frequency, &c.DevExtID); err != nil {
			return nil, err
		}
		collectors = append(collectors, c)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	// 3. 查询所有 active device_points。
	ptRows, err := r.db.QueryContext(ctx, `
SELECT
  dp.point_id, dp.node_id, dp.function_type, dp.point_name, dp.channel_no,
  dp.unit, dp.best_min, dp.best_max, dp.run_mode, dp.controllable_status,
  dp.poll_interval, dp.poll_cmd, dp.point_status, dp.remark, COALESCE(cn.shed_no, '')
FROM device_points dp
LEFT JOIN collector_nodes cn ON cn.node_id = dp.node_id
WHERE dp.point_status = 1
ORDER BY dp.node_id ASC, dp.channel_no ASC`)
	if err != nil {
		return nil, err
	}
	defer ptRows.Close()

	type ptRow struct {
		PointID            uint64
		NodeID             uint64
		FuncType           uint8
		FuncName           string
		ChNo               int
		Unit               string
		BestMin            *float64
		BestMax            *float64
		RunMode            uint8
		ControllableStatus uint8
		PollInterval       int
		PollCmd            string
		PointStatus        uint8
		Remark             string
		ShedNo             string
	}
	var points []ptRow
	for ptRows.Next() {
		var p ptRow
		var bestMin, bestMax sql.NullFloat64
		var pollCmd, remark sql.NullString
		if err := ptRows.Scan(
			&p.PointID, &p.NodeID, &p.FuncType, &p.FuncName, &p.ChNo,
			&p.Unit, &bestMin, &bestMax, &p.RunMode, &p.ControllableStatus,
			&p.PollInterval, &pollCmd, &p.PointStatus, &remark, &p.ShedNo,
		); err != nil {
			return nil, err
		}
		if bestMin.Valid {
			v := bestMin.Float64
			p.BestMin = &v
		}
		if bestMax.Valid {
			v := bestMax.Float64
			p.BestMax = &v
		}
		if pollCmd.Valid {
			p.PollCmd = pollCmd.String
		}
		if remark.Valid {
			p.Remark = remark.String
		}
		points = append(points, p)
	}
	if err := ptRows.Err(); err != nil {
		return nil, err
	}

	// 4. 查询所有 active 绑定关系，建立 point/node/exclusive gateway → user_id 映射。
	bindRows, err := r.db.QueryContext(ctx, `
SELECT br.gateway_id, br.node_id, br.point_id, br.user_id, cg.access_mode
FROM bind_relations br
JOIN cloud_gateways cg ON cg.gateway_id = br.gateway_id
WHERE br.bind_status = 1`)
	if err != nil {
		return nil, err
	}
	defer bindRows.Close()

	pointBoundMap := make(map[uint64]uint64)     // point_id → user_id
	collectorBoundMap := make(map[uint64]uint64) // node_id → user_id
	gatewayBoundMap := make(map[uint64]uint64)   // exclusive gateway_id → user_id
	for bindRows.Next() {
		var gatewayID, uid uint64
		var nodeID sql.NullInt64
		var pointID sql.NullInt64
		var accessMode string
		if err := bindRows.Scan(&gatewayID, &nodeID, &pointID, &uid, &accessMode); err != nil {
			return nil, err
		}
		if pointID.Valid {
			pointBoundMap[uint64(pointID.Int64)] = uid
			continue
		}
		if nodeID.Valid {
			collectorBoundMap[uint64(nodeID.Int64)] = uid
			continue
		}
		if domain.NormalizeGatewayAccessMode(accessMode) == domain.GatewayAccessModeExclusive {
			gatewayBoundMap[gatewayID] = uid
		}
	}
	if err := bindRows.Err(); err != nil {
		return nil, err
	}

	// 5. 组装树结构。
	// 建立索引：gateway_id → collectors, node_id → points。
	colByGW := make(map[uint64][]colRow)
	for _, c := range collectors {
		colByGW[c.GatewayID] = append(colByGW[c.GatewayID], c)
	}
	ptByNode := make(map[uint64][]ptRow)
	for _, p := range points {
		ptByNode[p.NodeID] = append(ptByNode[p.NodeID], p)
	}

	funcNameLabel := func(ft uint8) string {
		switch ft {
		case 1:
			return "温度"
		case 2:
			return "墒情"
		case 3:
			return "限位"
		case 4:
			return "继电器"
		case 5:
			return "水泵"
		case 6:
			return "风机"
		default:
			return "未知"
		}
	}

	tree := make([]domain.DeviceTreeNode, 0, len(gateways))
	for _, g := range gateways {
		node := domain.DeviceTreeNode{
			GatewayID:       g.ID,
			GatewayName:     g.Name,
			AccessMode:      g.AccessMode,
			AccessModeLabel: domain.GatewayAccessMode(g.AccessMode).Label(),
			Status:          g.Status,
			Collectors:      make([]domain.DeviceTreeCollector, 0),
		}
		for _, c := range colByGW[g.ID] {
			collectorBoundUserID, collectorIsBound := resolveCollectorBoundUser(collectorBoundMap, gatewayBoundMap, g.ID, c.NodeID)
			var collectorBoundUserIDPtr *uint64
			if collectorIsBound {
				collectorBoundUserIDPtr = &collectorBoundUserID
			}
			col := domain.DeviceTreeCollector{
				NodeID:           c.NodeID,
				NodeName:         c.NodeName,
				ShedNo:           c.ShedNo,
				DeviceExternalID: c.DevExtID,
				Frequency:        c.Frequency,
				IsBound:          collectorIsBound,
				BoundUserID:      collectorBoundUserIDPtr,
				Points:           make([]domain.DeviceTreePoint, 0),
			}
			for _, p := range ptByNode[c.NodeID] {
				boundUserID, isBound := resolveDeviceTreeBoundUser(pointBoundMap, collectorBoundMap, gatewayBoundMap, g.ID, p.NodeID, p.PointID)
				var boundUserIDPtr *uint64
				if isBound {
					boundUserIDPtr = &boundUserID
				}
				col.Points = append(col.Points, domain.DeviceTreePoint{
					PointID:            p.PointID,
					FunctionType:       p.FuncType,
					FunctionName:       funcNameLabel(p.FuncType),
					ChannelNo:          p.ChNo,
					PointName:          p.FuncName,
					ShedNo:             p.ShedNo,
					Unit:               p.Unit,
					BestMin:            p.BestMin,
					BestMax:            p.BestMax,
					RunMode:            p.RunMode,
					ControllableStatus: p.ControllableStatus,
					PollInterval:       p.PollInterval,
					PollCmd:            p.PollCmd,
					PointStatus:        p.PointStatus,
					Remark:             p.Remark,
					IsBound:            isBound,
					BoundUserID:        boundUserIDPtr,
				})
			}
			node.Collectors = append(node.Collectors, col)
		}
		tree = append(tree, node)
	}

	return tree, nil
}

func resolveDeviceTreeBoundUser(pointBoundMap map[uint64]uint64, collectorBoundMap map[uint64]uint64, gatewayBoundMap map[uint64]uint64, gatewayID uint64, nodeID uint64, pointID uint64) (uint64, bool) {
	if userID, ok := pointBoundMap[pointID]; ok {
		return userID, true
	}
	if userID, ok := collectorBoundMap[nodeID]; ok {
		return userID, true
	}
	if userID, ok := gatewayBoundMap[gatewayID]; ok {
		return userID, true
	}
	return 0, false
}

func resolveCollectorBoundUser(collectorBoundMap map[uint64]uint64, gatewayBoundMap map[uint64]uint64, gatewayID uint64, nodeID uint64) (uint64, bool) {
	if userID, ok := collectorBoundMap[nodeID]; ok {
		return userID, true
	}
	if userID, ok := gatewayBoundMap[gatewayID]; ok {
		return userID, true
	}
	return 0, false
}

// ListBindingsByUserID 返回指定用户的所有绑定关系（含 active 和已解绑）。
func (r *Repository) ListBindingsByUserID(ctx context.Context, userID uint64) ([]domain.BindRelation, error) {
	rows, err := r.db.QueryContext(ctx, bindingSelectColumns+`
WHERE br.user_id = ? AND br.bind_status = 1
ORDER BY br.updated_at DESC
LIMIT 500`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bindings := make([]domain.BindRelation, 0)
	for rows.Next() {
		b, err := scanBindingRelationRow(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, b)
	}
	return bindings, rows.Err()
}

// FindBindingByID 按 bind_id 查询绑定快照，包含已解绑记录。
func (r *Repository) FindBindingByID(ctx context.Context, bindID uint64) (*domain.BindRelation, error) {
	row := r.db.QueryRowContext(ctx, bindingSelectColumns+`
WHERE br.bind_id = ?
LIMIT 1`, bindID)
	b, err := scanBindingRelationRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindGatewayByID 按 gateway_id 查询 gateway。
func (r *Repository) FindGatewayByID(ctx context.Context, gatewayID uint64) (*domain.Gateway, error) {
	var g domain.Gateway
	err := r.db.QueryRowContext(ctx, `
SELECT gateway_id, gateway_external_id, gateway_name, access_mode, ip, port, gateway_sn, online_status, source_type, last_heartbeat_at_ms, created_at, updated_at
FROM cloud_gateways
WHERE gateway_id = ?
LIMIT 1`, gatewayID).Scan(
		&g.GatewayID, &g.GatewayExternalID, &g.GatewayName, &g.AccessMode, &g.IP, &g.Port, &g.GatewaySN,
		&g.OnlineStatus, &g.SourceType, &g.LastHeartbeatAtMS, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	g.AccessMode = string(domain.NormalizeGatewayAccessMode(g.AccessMode))
	return &g, nil
}

// FindCollectorNodeByID 按 node_id 查询 collector node。
func (r *Repository) FindCollectorNodeByID(ctx context.Context, nodeID uint64) (*domain.CollectorNode, error) {
	var c domain.CollectorNode
	var (
		lastReportAtMS sql.NullTime
		lastRawEventID sql.NullInt64
		remark         sql.NullString
		extra          sql.NullString
	)
	err := r.db.QueryRowContext(ctx, `
SELECT node_id, gateway_id, frequency, device_external_id, node_name, shed_no, base_name, area_name,
       collector_no, node_status, online_status, last_report_at_ms, last_raw_event_id, remark, extra,
       created_at, updated_at
FROM collector_nodes
WHERE node_id = ?
LIMIT 1`, nodeID).Scan(
		&c.NodeID, &c.GatewayID, &c.Frequency, &c.DeviceExternalID, &c.NodeName, &c.ShedNo, &c.BaseName, &c.AreaName,
		scanNullInt(&c.CollectorNo), &c.NodeStatus, &c.OnlineStatus, &lastReportAtMS, &lastRawEventID, &remark, &extra,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastReportAtMS.Valid {
		c.LastReportAtMS = &lastReportAtMS.Time
	}
	if lastRawEventID.Valid {
		v := uint64(lastRawEventID.Int64)
		c.LastRawEventID = &v
	}
	if remark.Valid {
		c.Remark = &remark.String
	}
	if extra.Valid {
		c.Extra = &extra.String
	}
	return &c, nil
}

// FindActiveBindingByUserAndGateway 检查是否已存在用户对某个 gateway 的 active 绑定。
func (r *Repository) FindActiveBindingByUserAndGateway(ctx context.Context, userID uint64, gatewayID uint64) (*domain.BindRelation, error) {
	row := r.db.QueryRowContext(ctx, bindingSelectColumns+`
WHERE br.user_id = ? AND br.gateway_id = ? AND br.bind_status = 1 AND br.point_id IS NULL
  AND br.node_id IS NULL
LIMIT 1`, userID, gatewayID)
	b, err := scanBindingRelationRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindActiveBindingByGateway 按 gateway_id 查询 active gateway 绑定。
func (r *Repository) FindActiveBindingByGateway(ctx context.Context, gatewayID uint64) (*domain.BindRelation, error) {
	row := r.db.QueryRowContext(ctx, bindingSelectColumns+`
WHERE br.gateway_id = ? AND br.bind_status = 1 AND br.point_id IS NULL
  AND br.node_id IS NULL
LIMIT 1`, gatewayID)
	b, err := scanBindingRelationRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindActiveBindingByUserAndNode 检查是否已存在用户对某个采集执行器的 active 绑定。
func (r *Repository) FindActiveBindingByUserAndNode(ctx context.Context, userID uint64, nodeID uint64) (*domain.BindRelation, error) {
	row := r.db.QueryRowContext(ctx, bindingSelectColumns+`
WHERE br.user_id = ? AND br.node_id = ? AND br.point_id IS NULL AND br.bind_status = 1
LIMIT 1`, userID, nodeID)
	b, err := scanBindingRelationRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindActiveBindingByNode 按 node_id 查询 active 采集执行器绑定。
func (r *Repository) FindActiveBindingByNode(ctx context.Context, nodeID uint64) (*domain.BindRelation, error) {
	row := r.db.QueryRowContext(ctx, bindingSelectColumns+`
WHERE br.node_id = ? AND br.point_id IS NULL AND br.bind_status = 1
LIMIT 1`, nodeID)
	b, err := scanBindingRelationRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ListGatewayBindCandidates 返回可绑定 gateway 候选列表。
func (r *Repository) ListGatewayBindCandidates(ctx context.Context, keyword string) ([]domain.GatewayBindCandidate, error) {
	query := `
SELECT g.gateway_id, g.gateway_name, g.gateway_external_id, g.access_mode, g.online_status, g.last_heartbeat_at_ms
FROM cloud_gateways g`
	var args []any
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query += `
WHERE g.gateway_name LIKE ? OR g.gateway_external_id LIKE ?`
		args = append(args, like, like)
	}
	query += `
ORDER BY g.updated_at DESC
LIMIT 100`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.GatewayBindCandidate, 0)
	for rows.Next() {
		var item domain.GatewayBindCandidate
		var lastHeartbeat time.Time
		if err := rows.Scan(&item.GatewayID, &item.GatewayName, &item.GatewayExternalID, &item.AccessMode, &item.OnlineStatus, &lastHeartbeat); err != nil {
			return nil, err
		}
		item.AccessMode = string(domain.NormalizeGatewayAccessMode(item.AccessMode))
		item.AccessModeLabel = domain.GatewayAccessMode(item.AccessMode).Label()
		item.LastHeartbeatAtMS = lastHeartbeat.UTC().Format("2006-01-02T15:04:05.000Z")
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateBinding 创建新的绑定关系。
func (r *Repository) CreateBinding(ctx context.Context, b *domain.BindRelation) error {
	if b.BindID == 0 {
		b.BindID = idgen.NextID()
	}
	if b.BindingType == "" {
		if b.PointID != nil {
			b.BindingType = string(domain.BindingTypePoint)
		} else if b.NodeID != nil {
			b.BindingType = string(domain.BindingTypeCollector)
		} else {
			b.BindingType = string(domain.BindingTypeGateway)
		}
	}
	now := time.Now()
	b.BoundAtMS = now
	b.BindStatus = domain.BindStatusActive
	b.CreatedAt = now
	b.UpdatedAt = now

	uid := nullableUint64(b.UserID)
	rid := nullableUint64(b.RealtimeID)
	nid := nullableUint64(b.NodeID)
	pid := nullableUint64(b.PointID)
	cbid := nullableUint64(b.CreatedBy)

	if b.PointID == nil && b.NodeID == nil {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		var gatewayAccessMode string
		if err := tx.QueryRowContext(ctx, `SELECT access_mode FROM cloud_gateways WHERE gateway_id = ? FOR UPDATE`, b.GatewayID).Scan(&gatewayAccessMode); err != nil {
			return err
		}
		if domain.NormalizeGatewayAccessMode(gatewayAccessMode) == domain.GatewayAccessModeExclusive {
			var existingBindID uint64
			var existingUserID sql.NullInt64
			err = tx.QueryRowContext(ctx, `
SELECT bind_id, user_id
FROM bind_relations
WHERE gateway_id = ? AND bind_status = 1 AND node_id IS NULL AND point_id IS NULL
LIMIT 1 FOR UPDATE`, b.GatewayID).Scan(&existingBindID, &existingUserID)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if err == nil {
				return domain.ErrBindingConflict
			}
		}

		_, err = tx.ExecContext(ctx, `
INSERT INTO bind_relations
  (bind_id, user_id, bind_user_name,
   gateway_id, realtime_id, node_id, point_id,
   shed_no, frequency, device_external_id, function_type,
   bind_status, bound_at_ms, created_by, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			b.BindID, uid, b.BindUserName,
			b.GatewayID, rid, nid, pid,
			b.ShedNo, b.Frequency, b.DeviceExternalID, uint8(b.FunctionType),
			uint8(b.BindStatus), b.BoundAtMS, cbid,
			b.CreatedAt, b.UpdatedAt,
		)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO bind_relations
  (bind_id, user_id, bind_user_name,
   gateway_id, realtime_id, node_id, point_id,
   shed_no, frequency, device_external_id, function_type,
   bind_status, bound_at_ms, created_by, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.BindID, uid, b.BindUserName,
		b.GatewayID, rid, nid, pid,
		b.ShedNo, b.Frequency, b.DeviceExternalID, uint8(b.FunctionType),
		uint8(b.BindStatus), b.BoundAtMS, cbid,
		b.CreatedAt, b.UpdatedAt,
	)
	if err != nil {
		log.Printf("[CreateBinding] INSERT failed: bind_id=%d user_id=%v point_id=%v gateway_id=%d err=%v",
			b.BindID, uid, pid, b.GatewayID, err)
	}
	return err
}

// UnbindRelation 解绑（软删除，更新 bind_status=2）。
func (r *Repository) UnbindRelation(ctx context.Context, bindID uint64) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE bind_relations
		 SET bind_status = ?, unbound_at_ms = ?, updated_at = ?
		 WHERE bind_id = ?`,
		uint8(domain.BindStatusUnbound), now, now, bindID,
	)
	return err
}

// CreateOperationRecord 写入后台操作审计记录。
func (r *Repository) CreateOperationRecord(ctx context.Context, record *domain.OperationRecord) error {
	if record.OpID == 0 {
		record.OpID = idgen.NextID()
	}
	if record.RunMode == 0 {
		record.RunMode = RunModeAuto
	}
	now := time.Now()
	if record.OperatedAtMS.IsZero() {
		record.OperatedAtMS = now
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	opContent := sql.NullString{String: record.OpContent, Valid: record.OpContent != ""}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO op_records
  (op_id, gateway_id, realtime_id, node_id, point_id,
   user_id, bind_user_name, operator_role,
   frequency, device_external_id, run_mode,
   rel1_status, rel2_status, op_type, op_content,
   command_id, operated_at_ms, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.OpID,
		record.GatewayID,
		nullableUint64(record.RealtimeID),
		nullableUint64(record.NodeID),
		nullableUint64(record.PointID),
		nullableUint64(record.UserID),
		record.BindUserName,
		record.OperatorRole,
		record.Frequency,
		record.DeviceExternalID,
		record.RunMode,
		record.Rel1Status,
		record.Rel2Status,
		uint8(record.OpType),
		opContent,
		nullableUint64(record.CommandID),
		record.OperatedAtMS,
		record.CreatedAt,
		record.CreatedAt,
	)
	return err
}

// ─── 控制日志 ───

const (
	defaultAdminLogLimit = 100

	controlLogSelectClause = `
SELECT
  cc.command_id, cc.gateway_id, COALESCE(cg.gateway_name, ''),
  COALESCE(cc.node_id, 0), COALESCE(cc.point_id, 0),
  COALESCE(u.display_name, u.username, '') AS operator_name,
  COALESCE(cc.requested_by, 0), cc.command_source,
  cc.frequency, cc.device_external_id,
  COALESCE(dp.function_type, 0) AS function_type,
  COALESCE(dp.point_name, '') AS point_name,
  cc.command_type, cc.run_mode, cc.command_status,
  cc.command_hex,
  COALESCE(cc.gateway_response, '') AS gateway_response,
  COALESCE(cc.error_message, '') AS error_message,
  COALESCE(cc.gateway_response, cc.error_message, '') AS result,
  cc.requested_at_ms, cc.sent_at_ms, cc.finished_at_ms`

	controlLogFromClause = `
FROM control_commands cc
LEFT JOIN cloud_gateways cg ON cc.gateway_id = cg.gateway_id
LEFT JOIN users u ON cc.requested_by = u.user_id
LEFT JOIN device_points dp ON cc.point_id = dp.point_id`

	listControlLogsQuery = controlLogSelectClause + controlLogFromClause + `
ORDER BY cc.requested_at_ms DESC
LIMIT ?`
)

const (
	operationLogSelectClause = `
SELECT
  op.op_id, op.gateway_id, COALESCE(cg.gateway_name, ''),
  op.realtime_id, op.node_id, op.point_id,
  op.user_id, COALESCE(u.display_name, u.username, '') AS operator_name,
  op.bind_user_name, op.operator_role,
  op.frequency, op.device_external_id,
  op.run_mode, op.rel1_status, op.rel2_status,
  op.op_type, COALESCE(op.op_content, ''),
  op.command_id, op.operated_at_ms, op.created_at`

	operationLogFromClause = `
FROM op_records op
LEFT JOIN cloud_gateways cg ON op.gateway_id = cg.gateway_id
LEFT JOIN users u ON op.user_id = u.user_id`

	listOperationLogsQuery = operationLogSelectClause + operationLogFromClause
)

func buildOperationLogFilterQuery(params domain.OperationLogQuery) (string, []interface{}) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)

	if params.Start != nil {
		conditions = append(conditions, "op.operated_at_ms >= ?")
		args = append(args, *params.Start)
	}
	if params.End != nil {
		conditions = append(conditions, "op.operated_at_ms <= ?")
		args = append(args, *params.End)
	}
	if params.OpType != nil {
		conditions = append(conditions, "op.op_type = ?")
		args = append(args, *params.OpType)
	}
	if params.Keyword != nil && *params.Keyword != "" {
		conditions = append(conditions, "(COALESCE(u.display_name, u.username, '') LIKE ? OR op.bind_user_name LIKE ? OR COALESCE(cg.gateway_name, '') LIKE ? OR COALESCE(op.op_content, '') LIKE ?)")
		kw := "%" + strings.TrimSpace(*params.Keyword) + "%"
		args = append(args, kw, kw, kw, kw)
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func buildControlLogFilterQuery(params domain.ControlLogQuery) (string, []interface{}) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)

	if params.Start != nil {
		conditions = append(conditions, "cc.requested_at_ms >= ?")
		args = append(args, *params.Start)
	}
	if params.End != nil {
		conditions = append(conditions, "cc.requested_at_ms <= ?")
		args = append(args, *params.End)
	}
	if params.CmdStatus != nil {
		conditions = append(conditions, "cc.command_status = ?")
		args = append(args, *params.CmdStatus)
	}
	if params.Source != nil {
		conditions = append(conditions, "cc.command_source = ?")
		args = append(args, *params.Source)
	}
	if params.Keyword != nil && *params.Keyword != "" {
		kw := "%" + strings.TrimSpace(*params.Keyword) + "%"
		conditions = append(conditions, "(CAST(cc.command_id AS CHAR) LIKE ? OR CAST(cc.gateway_id AS CHAR) LIKE ? OR CAST(cc.point_id AS CHAR) LIKE ? OR cc.command_hex LIKE ? OR COALESCE(dp.point_name, '') LIKE ?)")
		args = append(args, kw, kw, kw, kw, kw)
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// ListControlLogs 返回控制日志列表。
func (r *Repository) ListControlLogs(ctx context.Context, params domain.ControlLogQuery) ([]domain.ControlLogSummary, int, error) {
	whereClause, args := buildControlLogFilterQuery(params)
	listQuery := controlLogSelectClause + controlLogFromClause + whereClause + `
ORDER BY cc.requested_at_ms DESC`
	listArgs := append([]interface{}{}, args...)
	if params.Paginated {
		totalQuery := "SELECT COUNT(*) " + controlLogFromClause + whereClause
		var total int
		err := r.db.QueryRowContext(ctx, totalQuery, args...).Scan(&total)
		if err != nil {
			return nil, 0, err
		}
		listQuery += " LIMIT ? OFFSET ?"
		offset := (params.Page - 1) * params.Size
		listArgs = append(listArgs, params.Size, offset)
		rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()

		logs := make([]domain.ControlLogSummary, 0)
		for rows.Next() {
			var l domain.ControlLogSummary
			var commandType uint8
			var commandSource uint8
			var functionType uint8
			var pointName sql.NullString
			var result sql.NullString
			var sentAt sql.NullTime
			var finishedAt sql.NullTime
			if err := rows.Scan(
				&l.CommandID, &l.GatewayID, &l.GatewayName,
				&l.NodeID, &l.PointID,
				&l.BindUserName, &l.RequestedBy, &commandSource,
				&l.Frequency, &l.DevExternalID,
				&functionType, &pointName,
				&commandType, &l.RunMode, &l.CmdStatus,
				&l.CommandHex, &l.GatewayResponse, &l.ErrorMessage,
				&result, &l.OpTime, &sentAt, &finishedAt,
			); err != nil {
				return nil, 0, err
			}
			l.OpID = l.CommandID
			l.CommandSource = commandSource
			l.SourceLabel = domain.CommandSource(commandSource).Label()
			l.FunctionType = functionType
			if pointName.Valid && pointName.String != "" {
				l.FunctionName = pointName.String
			} else {
				l.FunctionName = domain.FunctionType(functionType).FunctionTypeName()
			}
			l.OpType = commandTypeLabel(commandType)
			l.CmdStatusLabel = domain.CommandStatus(l.CmdStatus).Label()
			if result.Valid {
				l.Result = result.String
			}
			if sentAt.Valid {
				l.SentAt = &sentAt.Time
			}
			if finishedAt.Valid {
				l.FinishedAt = &finishedAt.Time
			}
			logs = append(logs, l)
		}
		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
		return logs, total, nil
	}

	// Compatibility mode: return latest records with default limit.
	listQuery += " LIMIT ?"
	listArgs = append(listArgs, defaultAdminLogLimit)
	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := make([]domain.ControlLogSummary, 0)
	for rows.Next() {
		var l domain.ControlLogSummary
		var commandType uint8
		var commandSource uint8
		var functionType uint8
		var pointName sql.NullString
		var result sql.NullString
		var sentAt sql.NullTime
		var finishedAt sql.NullTime
		if err := rows.Scan(
			&l.CommandID, &l.GatewayID, &l.GatewayName,
			&l.NodeID, &l.PointID,
			&l.BindUserName, &l.RequestedBy, &commandSource,
			&l.Frequency, &l.DevExternalID,
			&functionType, &pointName,
			&commandType, &l.RunMode, &l.CmdStatus,
			&l.CommandHex, &l.GatewayResponse, &l.ErrorMessage,
			&result, &l.OpTime, &sentAt, &finishedAt,
		); err != nil {
			return nil, 0, err
		}
		l.OpID = l.CommandID
		l.CommandSource = commandSource
		l.SourceLabel = domain.CommandSource(commandSource).Label()
		l.FunctionType = functionType
		if pointName.Valid && pointName.String != "" {
			l.FunctionName = pointName.String
		} else {
			l.FunctionName = domain.FunctionType(functionType).FunctionTypeName()
		}
		l.OpType = commandTypeLabel(commandType)
		l.CmdStatusLabel = domain.CommandStatus(l.CmdStatus).Label()
		if result.Valid {
			l.Result = result.String
		}
		if sentAt.Valid {
			l.SentAt = &sentAt.Time
		}
		if finishedAt.Valid {
			l.FinishedAt = &finishedAt.Time
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, 0, nil
}

// ListOperationLogs 返回后台操作审计日志列表。
func (r *Repository) ListOperationLogs(ctx context.Context, params domain.OperationLogQuery) ([]domain.OperationRecord, int, error) {
	whereClause, args := buildOperationLogFilterQuery(params)
	listQuery := listOperationLogsQuery + whereClause + `
ORDER BY op.operated_at_ms DESC`
	listArgs := append([]interface{}{}, args...)
	if params.Paginated {
		totalQuery := "SELECT COUNT(*) " + operationLogFromClause + whereClause
		var total int
		err := r.db.QueryRowContext(ctx, totalQuery, args...).Scan(&total)
		if err != nil {
			return nil, 0, err
		}
		listQuery += " LIMIT ? OFFSET ?"
		offset := (params.Page - 1) * params.Size
		listArgs = append(listArgs, params.Size, offset)
		rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()

		logs := make([]domain.OperationRecord, 0)
		for rows.Next() {
			var l domain.OperationRecord
			var realtimeID sql.NullInt64
			var nodeID sql.NullInt64
			var pointID sql.NullInt64
			var userID sql.NullInt64
			var commandID sql.NullInt64
			var opType uint8
			if err := rows.Scan(
				&l.OpID, &l.GatewayID, &l.GatewayName,
				&realtimeID, &nodeID, &pointID,
				&userID, &l.OperatorName,
				&l.BindUserName, &l.OperatorRole,
				&l.Frequency, &l.DeviceExternalID,
				&l.RunMode, &l.Rel1Status, &l.Rel2Status,
				&opType, &l.OpContent,
				&commandID, &l.OperatedAtMS, &l.CreatedAt,
			); err != nil {
				return nil, 0, err
			}
			l.RealtimeID = uint64PtrFromNullInt64(realtimeID)
			l.NodeID = uint64PtrFromNullInt64(nodeID)
			l.PointID = uint64PtrFromNullInt64(pointID)
			l.UserID = uint64PtrFromNullInt64(userID)
			l.CommandID = uint64PtrFromNullInt64(commandID)
			l.OpType = domain.OpType(opType)
			l.OpTypeLabel = l.OpType.Label()
			logs = append(logs, l)
		}
		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
		return logs, total, nil
	}

	// Compatibility mode: return latest records with default limit.
	listQuery += " LIMIT ?"
	listArgs = append(listArgs, defaultAdminLogLimit)
	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := make([]domain.OperationRecord, 0)
	for rows.Next() {
		var l domain.OperationRecord
		var realtimeID sql.NullInt64
		var nodeID sql.NullInt64
		var pointID sql.NullInt64
		var userID sql.NullInt64
		var commandID sql.NullInt64
		var opType uint8
		if err := rows.Scan(
			&l.OpID, &l.GatewayID, &l.GatewayName,
			&realtimeID, &nodeID, &pointID,
			&userID, &l.OperatorName,
			&l.BindUserName, &l.OperatorRole,
			&l.Frequency, &l.DeviceExternalID,
			&l.RunMode, &l.Rel1Status, &l.Rel2Status,
			&opType, &l.OpContent,
			&commandID, &l.OperatedAtMS, &l.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		l.RealtimeID = uint64PtrFromNullInt64(realtimeID)
		l.NodeID = uint64PtrFromNullInt64(nodeID)
		l.PointID = uint64PtrFromNullInt64(pointID)
		l.UserID = uint64PtrFromNullInt64(userID)
		l.CommandID = uint64PtrFromNullInt64(commandID)
		l.OpType = domain.OpType(opType)
		l.OpTypeLabel = l.OpType.Label()
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, 0, nil
}

func commandTypeLabel(t uint8) string {
	switch domain.CommandType(t) {
	case domain.CommandTypeRelayOpen:
		return "开启"
	case domain.CommandTypeRelayClose:
		return "关闭"
	case domain.CommandTypeRelayStop:
		return "停止"
	case domain.CommandTypePoll:
		return "轮询"
	case domain.CommandTypeReadStatus:
		return "读取状态"
	case domain.CommandTypeCustomHex:
		return "自定义 Hex"
	default:
		return "未知"
	}
}

// ─── 用户列表 ───

// ListFarmers 返回农户列表（user_type=2, status<>3），支持 keyword 模糊搜索。
// 返回用户列表和对应的 binding_count。
func (r *Repository) ListFarmers(ctx context.Context, keyword string) ([]domain.User, []int, error) {
	var rows *sql.Rows
	var err error

	if keyword != "" {
		like := "%" + keyword + "%"
		rows, err = r.db.QueryContext(ctx, `
SELECT
  u.user_id, u.user_type, u.role, u.username, u.mobile,
  u.address, u.remark,
  u.wechat_openid, u.wechat_unionid, u.password_hash,
  u.display_name, u.avatar_url, u.status,
  u.last_login_at_ms, u.created_at, u.updated_at,
  COALESCE(bc.cnt, 0) AS binding_count
FROM users u
LEFT JOIN (
  SELECT user_id, COUNT(*) AS cnt
  FROM bind_relations
  WHERE bind_status = 1
  GROUP BY user_id
) bc ON u.user_id = bc.user_id
WHERE u.user_type = 2 AND u.status <> 3
	  AND (CAST(u.user_id AS CHAR) LIKE ? OR u.display_name LIKE ? OR u.mobile LIKE ? OR u.wechat_openid LIKE ?)
	ORDER BY u.created_at DESC
	LIMIT 200`, like, like, like, like)
	} else {
		rows, err = r.db.QueryContext(ctx, `
SELECT
  u.user_id, u.user_type, u.role, u.username, u.mobile,
  u.address, u.remark,
  u.wechat_openid, u.wechat_unionid, u.password_hash,
  u.display_name, u.avatar_url, u.status,
  u.last_login_at_ms, u.created_at, u.updated_at,
  COALESCE(bc.cnt, 0) AS binding_count
FROM users u
LEFT JOIN (
  SELECT user_id, COUNT(*) AS cnt
  FROM bind_relations
  WHERE bind_status = 1
  GROUP BY user_id
) bc ON u.user_id = bc.user_id
WHERE u.user_type = 2 AND u.status <> 3
ORDER BY u.created_at DESC
LIMIT 200`)
	}
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	bindingCounts := make([]int, 0)
	for rows.Next() {
		var u domain.User
		var bc int
		var (
			username      sql.NullString
			mobile        sql.NullString
			address       sql.NullString
			remark        sql.NullString
			wechatOpenID  sql.NullString
			wechatUnionID sql.NullString
			passwordHash  sql.NullString
			displayName   sql.NullString
			avatarURL     sql.NullString
			lastLoginAtMS sql.NullTime
		)
		if err := rows.Scan(
			&u.UserID, &u.UserType, &u.Role,
			&username, &mobile,
			&address, &remark,
			&wechatOpenID, &wechatUnionID, &passwordHash,
			&displayName, &avatarURL, &u.Status,
			&lastLoginAtMS, &u.CreatedAt, &u.UpdatedAt,
			&bc,
		); err != nil {
			return nil, nil, err
		}
		if username.Valid {
			u.Username = &username.String
		}
		if mobile.Valid {
			u.Mobile = &mobile.String
		}
		if address.Valid {
			u.Address = &address.String
		}
		if remark.Valid {
			u.Remark = &remark.String
		}
		if wechatOpenID.Valid {
			u.WechatOpenID = &wechatOpenID.String
		}
		if wechatUnionID.Valid {
			u.WechatUnionID = &wechatUnionID.String
		}
		if passwordHash.Valid {
			u.PasswordHash = &passwordHash.String
		}
		if displayName.Valid {
			u.DisplayName = &displayName.String
		}
		if avatarURL.Valid {
			u.AvatarURL = &avatarURL.String
		}
		if lastLoginAtMS.Valid {
			u.LastLoginAtMS = &lastLoginAtMS.Time
		}
		users = append(users, u)
		bindingCounts = append(bindingCounts, bc)
	}
	return users, bindingCounts, rows.Err()
}

// ListUsers 返回所有用户列表。
func (r *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  user_id, user_type, role, username, mobile,
  address, remark,
  wechat_openid, wechat_unionid, password_hash,
  display_name, avatar_url, status,
  last_login_at_ms, created_at, updated_at
FROM users
ORDER BY created_at DESC
LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var u domain.User
		var (
			username      sql.NullString
			mobile        sql.NullString
			address       sql.NullString
			remark        sql.NullString
			wechatOpenID  sql.NullString
			wechatUnionID sql.NullString
			passwordHash  sql.NullString
			displayName   sql.NullString
			avatarURL     sql.NullString
			lastLoginAtMS sql.NullTime
		)
		if err := rows.Scan(
			&u.UserID, &u.UserType, &u.Role,
			&username, &mobile,
			&address, &remark,
			&wechatOpenID, &wechatUnionID, &passwordHash,
			&displayName, &avatarURL, &u.Status,
			&lastLoginAtMS, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if username.Valid {
			u.Username = &username.String
		}
		if mobile.Valid {
			u.Mobile = &mobile.String
		}
		if address.Valid {
			u.Address = &address.String
		}
		if remark.Valid {
			u.Remark = &remark.String
		}
		if wechatOpenID.Valid {
			u.WechatOpenID = &wechatOpenID.String
		}
		if wechatUnionID.Valid {
			u.WechatUnionID = &wechatUnionID.String
		}
		if passwordHash.Valid {
			u.PasswordHash = &passwordHash.String
		}
		if displayName.Valid {
			u.DisplayName = &displayName.String
		}
		if avatarURL.Valid {
			u.AvatarURL = &avatarURL.String
		}
		if lastLoginAtMS.Valid {
			u.LastLoginAtMS = &lastLoginAtMS.Time
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ─── 辅助函数 ───

// SearchDevicePoints 按关键字搜索功能点，支持按绑定状态过滤。
func (r *Repository) SearchDevicePoints(ctx context.Context, keyword string, bindStatus string) ([]domain.DevicePointSearchResult, error) {
	var args []interface{}
	whereClause := ""

	// keyword 过滤放在应用层（handler），因为 function_name 是通过 function_type 映射的，SQL 无法匹配

	switch bindStatus {
	case "unbound":
		whereClause += ` AND br.bind_id IS NULL`
	case "bound":
		whereClause += ` AND br.bind_id IS NOT NULL`
	}

	query := `SELECT
  dp.point_id, dp.gateway_id, cg.gateway_name,
  dp.node_id, cn.frequency, cn.device_external_id,
  dp.function_type, dp.point_name, dp.unit,
  dp.channel_no, cn.shed_no,
  CASE WHEN br.bind_id IS NOT NULL THEN 1 ELSE 0 END AS is_bound,
  br.user_id AS bind_user_id,
  br.bind_user_name
FROM device_points dp
JOIN cloud_gateways cg ON dp.gateway_id = cg.gateway_id
JOIN collector_nodes cn ON dp.node_id = cn.node_id
LEFT JOIN bind_relations br ON br.bind_status = 1 AND (
  br.point_id = dp.point_id
  OR (br.point_id IS NULL AND br.node_id = dp.node_id)
  OR (br.point_id IS NULL AND br.node_id IS NULL AND br.gateway_id = dp.gateway_id AND cg.access_mode = 'exclusive')
)
WHERE dp.point_status = 1` + whereClause + `
ORDER BY dp.gateway_id, cn.frequency, dp.channel_no
LIMIT 50`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]domain.DevicePointSearchResult, 0)
	for rows.Next() {
		var s domain.DevicePointSearchResult
		var isBound int
		var bindUserID sql.NullInt64
		var bindUserName sql.NullString

		if err := rows.Scan(
			&s.PointID,
			&s.GatewayID,
			&s.GatewayName,
			&s.NodeID,
			&s.Frequency,
			&s.DeviceExternalID,
			&s.FunctionType,
			&s.PointName,
			&s.Unit,
			&s.ChannelNo,
			&s.ShedNo,
			&isBound,
			&bindUserID,
			&bindUserName,
		); err != nil {
			return nil, err
		}

		s.IsBound = isBound == 1
		if bindUserID.Valid {
			uid := uint64(bindUserID.Int64)
			s.BindUserID = &uid
		}
		if bindUserName.Valid {
			s.BindUserName = bindUserName.String
		}
		s.FunctionName = s.FunctionType.FunctionTypeName()

		results = append(results, s)
	}
	return results, rows.Err()
}

// ─── 辅助函数 ───

// 常量别名，避免写魔数。
const (
	RunModeAuto    = 1
	OpStateUnknown = 0
)

func nullableUint64(v *uint64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func uint64PtrFromNullInt64(v sql.NullInt64) *uint64 {
	if !v.Valid {
		return nil
	}
	out := uint64(v.Int64)
	return &out
}

func nullableInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func nullableFloat64(v *float64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func strPtr(s string) *string {
	return &s
}

func emptyToNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// scanNullUint64 返回一个可被 Scan 写入的值，Scan 后自动回填到 dest。
// 使用自定义 scanner 类型确保 sql.NullInt64 的结果正确回填。
func scanNullUint64(dest **uint64) interface{} {
	return &nullUint64Scanner{dest: dest}
}

type nullUint64Scanner struct {
	dest **uint64
}

func (s *nullUint64Scanner) Scan(value interface{}) error {
	var n sql.NullInt64
	if err := n.Scan(value); err != nil {
		return err
	}
	if n.Valid {
		val := uint64(n.Int64)
		*s.dest = &val
	} else {
		*s.dest = nil
	}
	return nil
}

// scanNullInt 返回一个可被 Scan 写入的值，Scan 后自动回填到 dest。
func scanNullInt(dest **int) interface{} {
	return &nullIntScanner{dest: dest}
}

type nullIntScanner struct {
	dest **int
}

func (s *nullIntScanner) Scan(value interface{}) error {
	var n sql.NullInt64
	if err := n.Scan(value); err != nil {
		return err
	}
	if n.Valid {
		val := int(n.Int64)
		*s.dest = &val
	} else {
		*s.dest = nil
	}
	return nil
}

// scanNullString 返回一个可被 Scan 写入的值，Scan 后自动回填到 dest。
func scanNullString(dest **string) interface{} {
	return &nullStringScanner{dest: dest}
}

type nullStringScanner struct {
	dest **string
}

func (s *nullStringScanner) Scan(value interface{}) error {
	var n sql.NullString
	if err := n.Scan(value); err != nil {
		return err
	}
	if n.Valid {
		*s.dest = &n.String
	} else {
		*s.dest = nil
	}
	return nil
}

// scanNullTime 返回一个可被 Scan 写入的值，Scan 后自动回填到 dest。
func scanNullTime(dest **time.Time) interface{} {
	return &nullTimeScanner{dest: dest}
}

type nullTimeScanner struct {
	dest **time.Time
}

func (s *nullTimeScanner) Scan(value interface{}) error {
	var n sql.NullTime
	if err := n.Scan(value); err != nil {
		return err
	}
	if n.Valid {
		*s.dest = &n.Time
	} else {
		*s.dest = nil
	}
	return nil
}

func float64Value(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func valueStatusLabel(s uint8) string {
	switch s {
	case 0:
		return "unknown"
	case 1:
		return "正常"
	case 2:
		return "偏低"
	case 3:
		return "偏高"
	case 4:
		return "开启"
	case 5:
		return "关闭"
	case 6:
		return "离线"
	default:
		return "未知"
	}
}

func runModeLabel(m uint8) string {
	if m == 1 {
		return "自动"
	}
	return "手动"
}

func opTypeLabel(t uint8) string {
	switch t {
	case 1:
		return "绑定"
	case 2:
		return "解绑"
	case 3:
		return "控制"
	case 4:
		return "配置"
	default:
		return "未知"
	}
}
