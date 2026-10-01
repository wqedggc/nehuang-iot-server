package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/idgen"
)

func (r *Repository) CreateControlCommandForPoint(ctx context.Context, input domain.CreateControlCommandInput) (*domain.ControlCommand, error) {
	if input.CommandType == domain.CommandTypeCustomHex {
		safe, err := domain.ValidateControlHex(input.CommandHex)
		if err != nil {
			return nil, err
		}
		input.CommandHex = safe
	}

	if input.PointID == 0 {
		return nil, fmt.Errorf("point_id is required")
	}
	if input.CommandHex == "" {
		return nil, fmt.Errorf("command_hex is required")
	}

	var cmd domain.ControlCommand
	var realtimeID, nodeID, pointID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
SELECT
  dp.gateway_id, dp.node_id, dp.point_id,
  dp.frequency, dp.device_external_id,
  COALESCE(dp.run_mode, 2),
  drm.realtime_id
FROM device_points dp
LEFT JOIN device_realtime_maps drm ON drm.point_id = dp.point_id
WHERE dp.point_id = ?
LIMIT 1`, input.PointID).Scan(
		&cmd.GatewayID, &nodeID, &pointID,
		&cmd.Frequency, &cmd.DeviceExternalID,
		&cmd.RunMode, &realtimeID,
	)
	if err != nil {
		return nil, err
	}
	if realtimeID.Valid {
		v := uint64(realtimeID.Int64)
		cmd.RealtimeID = &v
	}
	if nodeID.Valid {
		v := uint64(nodeID.Int64)
		cmd.NodeID = &v
	}
	if pointID.Valid {
		v := uint64(pointID.Int64)
		cmd.PointID = &v
	}

	now := time.Now()
	cmd.CommandID = idgen.NextID()
	cmd.CommandSource = input.CommandSource
	cmd.CommandType = input.CommandType
	cmd.CommandHex = input.CommandHex
	cmd.CommandStatus = domain.CommandStatusCreated
	cmd.RequestedBy = input.RequestedBy
	cmd.RequestedAtMS = now
	cmd.CreatedAt = now
	cmd.UpdatedAt = now
	cmd.Rel1TargetStatus = input.Rel1TargetStatus
	cmd.Rel2TargetStatus = input.Rel2TargetStatus
	cmd.Extra = input.Extra

	_, err = r.db.ExecContext(ctx, `
INSERT INTO control_commands
  (command_id, gateway_id, realtime_id, node_id, point_id,
   frequency, device_external_id, command_source, command_type, run_mode,
   rel1_target_status, rel2_target_status, command_hex, command_status,
   requested_by, requested_at_ms, extra, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?,
   ?, ?, ?, ?, ?,
   ?, ?, ?, ?,
   ?, ?, ?, ?, ?)`,
		cmd.CommandID, cmd.GatewayID, nullableUint64(cmd.RealtimeID), nullableUint64(cmd.NodeID), nullableUint64(cmd.PointID),
		cmd.Frequency, cmd.DeviceExternalID, uint8(cmd.CommandSource), uint8(cmd.CommandType), uint8(cmd.RunMode),
		nullableUint8(cmd.Rel1TargetStatus), nullableUint8(cmd.Rel2TargetStatus), cmd.CommandHex, uint8(cmd.CommandStatus),
		nullableUint64(cmd.RequestedBy), cmd.RequestedAtMS, nullableString(cmd.Extra), cmd.CreatedAt, cmd.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

const getControlCommandByIDQuery = `
SELECT
  cc.command_id, cc.gateway_id, cc.realtime_id, cc.node_id, cc.point_id,
  cc.frequency, cc.device_external_id, cc.command_source, cc.command_type,
  cc.run_mode, cc.rel1_target_status, cc.rel2_target_status,
  cc.command_hex, cc.command_status, cc.requested_by, cc.requested_at_ms,
  cc.sent_at_ms, cc.finished_at_ms, cc.gateway_response, cc.error_message, cc.extra,
  cc.created_at, cc.updated_at
FROM control_commands cc
WHERE cc.command_id = ?
LIMIT 1`

// GetControlCommandByID returns the existing control_commands record for status queries.
func (r *Repository) GetControlCommandByID(ctx context.Context, commandID uint64) (*domain.ControlCommand, error) {
	cmd, err := scanControlCommand(r.db.QueryRowContext(ctx, getControlCommandByIDQuery, commandID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

func nullableUint8(v *uint8) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func (r *Repository) ListPendingControlCommands(ctx context.Context, gatewayExternalID string, limit int) ([]domain.ControlCommand, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT
  cc.command_id, cc.gateway_id, cc.realtime_id, cc.node_id, cc.point_id,
  cc.frequency, cc.device_external_id, cc.command_source, cc.command_type,
  cc.run_mode, cc.rel1_target_status, cc.rel2_target_status,
  cc.command_hex, cc.command_status, cc.requested_by, cc.requested_at_ms,
  cc.sent_at_ms, cc.finished_at_ms, cc.gateway_response, cc.error_message, cc.extra,
  cc.created_at, cc.updated_at
FROM control_commands cc
JOIN cloud_gateways cg ON cg.gateway_id = cc.gateway_id
WHERE cg.gateway_external_id = ?
  AND cc.command_status = ?
ORDER BY cc.requested_at_ms ASC
LIMIT ?`, gatewayExternalID, uint8(domain.CommandStatusCreated), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	commands := make([]domain.ControlCommand, 0)
	for rows.Next() {
		cmd, err := scanControlCommand(rows)
		if err != nil {
			return nil, err
		}
		commands = append(commands, cmd)
	}
	return commands, rows.Err()
}

func (r *Repository) MarkControlCommandSent(ctx context.Context, commandID uint64, gatewayResponse *string) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx, `
UPDATE control_commands
SET command_status = ?, sent_at_ms = COALESCE(sent_at_ms, ?), gateway_response = ?, updated_at = ?
WHERE command_id = ? AND (command_status = ? OR (command_status = ? AND extra LIKE ?))`,
		uint8(domain.CommandStatusSent), now, nullableString(gatewayResponse), now,
		commandID, uint8(domain.CommandStatusCreated), uint8(domain.CommandStatusUnknown), `{"source":"admin_raw_debug"%`,
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("command is not pending or not found")
	}
	return nil
}

func (r *Repository) MarkControlCommandsTimeout(ctx context.Context, cutoff time.Time) (int64, error) {
	now := time.Now()
	res, err := r.db.ExecContext(ctx, `
UPDATE control_commands
SET command_status = ?, finished_at_ms = COALESCE(finished_at_ms, ?),
    error_message = COALESCE(error_message, ?), updated_at = ?
WHERE (
    command_status = ? AND requested_at_ms < ?
  ) OR (
    command_status IN (?, ?) AND COALESCE(sent_at_ms, requested_at_ms) < ? AND (extra IS NULL OR extra NOT LIKE '{"source":"admin_raw_debug"%')
  )`,
		uint8(domain.CommandStatusTimeout), now,
		"control command timed out before final hardware result", now,
		uint8(domain.CommandStatusCreated), cutoff,
		uint8(domain.CommandStatusSent), uint8(domain.CommandStatusAck), cutoff,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *Repository) ApplyControlCommandResult(ctx context.Context, input domain.AdapterCommandResultInput) error {
	if input.CommandID == 0 {
		return fmt.Errorf("command_id is required")
	}
	if input.Status != domain.CommandStatusAck &&
		input.Status != domain.CommandStatusSucceeded &&
		input.Status != domain.CommandStatusFailed &&
		input.Status != domain.CommandStatusTimeout {
		return fmt.Errorf("invalid command status")
	}

	now := time.Now()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var cmd domain.ControlCommand
	var realtimeID, nodeID, pointID, requestedBy sql.NullInt64
	err = tx.QueryRowContext(ctx, `
SELECT command_id, gateway_id, realtime_id, node_id, point_id,
       frequency, device_external_id, command_source, command_type,
       run_mode, command_status, requested_by
FROM control_commands WHERE command_id = ? FOR UPDATE`, input.CommandID).Scan(
		&cmd.CommandID, &cmd.GatewayID, &realtimeID, &nodeID, &pointID,
		&cmd.Frequency, &cmd.DeviceExternalID, &cmd.CommandSource, &cmd.CommandType,
		&cmd.RunMode, &cmd.CommandStatus, &requestedBy,
	)
	if err != nil {
		return err
	}
	if realtimeID.Valid {
		v := uint64(realtimeID.Int64)
		cmd.RealtimeID = &v
	}
	if nodeID.Valid {
		v := uint64(nodeID.Int64)
		cmd.NodeID = &v
	}
	if pointID.Valid {
		v := uint64(pointID.Int64)
		cmd.PointID = &v
	}

	if cmd.CommandStatus.CanAdvanceResult(input.Status) {
		var finishedAt any
		if input.Status.IsTerminal() {
			finishedAt = now
		}
		_, err = tx.ExecContext(ctx, `
UPDATE control_commands
SET command_status = ?, finished_at_ms = ?, gateway_response = ?, error_message = ?, updated_at = ?
WHERE command_id = ?`,
			uint8(input.Status), finishedAt, nullableString(input.GatewayResponse), nullableString(input.ErrorMessage), now, input.CommandID,
		)
		if err != nil {
			return err
		}
	}

	resultStatus := uint8(4)
	switch input.Status {
	case domain.CommandStatusAck:
		resultStatus = 1
	case domain.CommandStatusSucceeded:
		resultStatus = 2
	case domain.CommandStatusFailed, domain.CommandStatusTimeout:
		resultStatus = 3
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO control_results
  (result_id, command_id, gateway_id, realtime_id, node_id, point_id,
   frequency, device_external_id, result_status, result_hex, result_snapshot,
   received_at_ms, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idgen.NextID(), input.CommandID, cmd.GatewayID, nullableUint64(cmd.RealtimeID), nullableUint64(cmd.NodeID), nullableUint64(cmd.PointID),
		cmd.Frequency, cmd.DeviceExternalID, resultStatus, nullableString(input.ResultHex), nullableString(input.ResultSnapshot),
		now, now, now,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

type controlCommandScanner interface {
	Scan(dest ...any) error
}

func scanControlCommand(rows controlCommandScanner) (domain.ControlCommand, error) {
	var cmd domain.ControlCommand
	var realtimeID, nodeID, pointID, requestedBy sql.NullInt64
	var rel1, rel2 sql.NullInt64
	var sentAt, finishedAt sql.NullTime
	var gatewayResponse, errorMessage, extra sql.NullString
	err := rows.Scan(
		&cmd.CommandID, &cmd.GatewayID, &realtimeID, &nodeID, &pointID,
		&cmd.Frequency, &cmd.DeviceExternalID, &cmd.CommandSource, &cmd.CommandType,
		&cmd.RunMode, &rel1, &rel2,
		&cmd.CommandHex, &cmd.CommandStatus, &requestedBy, &cmd.RequestedAtMS,
		&sentAt, &finishedAt, &gatewayResponse, &errorMessage, &extra,
		&cmd.CreatedAt, &cmd.UpdatedAt,
	)
	if err != nil {
		return cmd, err
	}
	if realtimeID.Valid {
		v := uint64(realtimeID.Int64)
		cmd.RealtimeID = &v
	}
	if nodeID.Valid {
		v := uint64(nodeID.Int64)
		cmd.NodeID = &v
	}
	if pointID.Valid {
		v := uint64(pointID.Int64)
		cmd.PointID = &v
	}
	if requestedBy.Valid {
		v := uint64(requestedBy.Int64)
		cmd.RequestedBy = &v
	}
	if rel1.Valid {
		v := uint8(rel1.Int64)
		cmd.Rel1TargetStatus = &v
	}
	if rel2.Valid {
		v := uint8(rel2.Int64)
		cmd.Rel2TargetStatus = &v
	}
	if sentAt.Valid {
		cmd.SentAtMS = &sentAt.Time
	}
	if finishedAt.Valid {
		cmd.FinishedAtMS = &finishedAt.Time
	}
	if gatewayResponse.Valid {
		cmd.GatewayResponse = &gatewayResponse.String
	}
	if errorMessage.Valid {
		cmd.ErrorMessage = &errorMessage.String
	}
	if extra.Valid {
		cmd.Extra = &extra.String
	}
	return cmd, nil
}
